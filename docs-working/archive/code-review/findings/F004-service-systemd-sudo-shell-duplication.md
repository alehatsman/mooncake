---
id: F004
title: service handler has ~6× repeated "if ShouldBecome / sudo -S / else plain" shell-out block
severity: smell
package: internal/actions/service
file: internal/actions/service/handler.go
lines: 457-493, 528-565, 568-588, 591-647, 650-672, 1024-1071
status: done
resolved_by: worktree-fix-f004 (0a8e4eff / 97f111a3)
verified: 2026-05-16 — confirmed fix shape on master @ 97f111a3. becomeAwareCommand + runBecomeAware helpers used at 13 call sites; only 3 ShouldBecome checks remain (was 6 in original); LOC 1607→1202 (back under 1500 soft-cap); tests green
---

## What

Six functions in `handler.go` each hand-roll the same shell-out
pattern:

```go
var cmd *exec.Cmd
if step.ShouldBecome() {
    if !security.IsBecomeSupported() {
        return false, &executor.SetupError{Component: "become", Issue: ...}
    }
    if ec.Svc.SudoPass == "" {
        return false, &executor.SetupError{Component: "sudo", Issue: "no password provided. Use --sudo-pass flag"}
    }
    cmd = exec.Command("sudo", "-S", PROGRAM, ARGS...)
    cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
} else {
    cmd = exec.Command(PROGRAM, ARGS...)
}

output, err := cmd.CombinedOutput()
if err != nil {
    exitCode := 1
    if cmd.ProcessState != nil { exitCode = cmd.ProcessState.ExitCode() }
    return false, &executor.CommandError{ExitCode: exitCode, Cause: fmt.Errorf("PROGRAM ACTION failed: %w (output: %s)", err, string(output))}
}
```

Hit sites:

| Function | Line | Program |
|---|---|---|
| `systemdDaemonReload` | 457 | `systemctl daemon-reload` |
| `manageSystemdServiceState` | 528 | `systemctl <start/stop/restart/reload>` |
| `getSystemdServiceState` | 568 | `systemctl is-active` |
| `manageSystemdServiceEnabled` | 603 | `systemctl <enable/disable>` |
| `isSystemdServiceEnabled` | 650 | `systemctl is-enabled` |
| `executeLaunchctlCommand` | 1024 | `launchctl <command>` |

`executeLaunchctlCommand` (1024) is a partial extraction — it
DRYs the launchd uses but not the systemd ones. Three of the six
systemd shell-outs (`getSystemdServiceState`, `isSystemdServiceEnabled`)
even ignore the command error (`cmd.Output()` → `_`) but still
emit the same 20 lines for sudo detection.

## Why it matters

1. **The handler is 1,607 LOC, over the 1,500 soft cap.** A single
   `becomeAwareCommand(program string, args []string, sudoPass string, doBecome bool)
   (*exec.Cmd, error)` helper would collapse each call site to ~3
   lines. Conservative estimate: **−80 LOC**, bringing the package
   under cap.

2. **It's a real bug surface, not just a smell.** Two of the six
   call sites omit `IsBecomeSupported()` (lines 569 and 651 in
   `getSystemdServiceState` / `isSystemdServiceEnabled`). Both
   still gate on `SudoPass`, but a become-unsupported-but-asked
   case sneaks through to `exec.Command("sudo"...)` instead of
   short-circuiting with a clear error. Likely harmless in
   practice because OS-level absence of `sudo` would just produce
   a `cmd.Output()` error, but the asymmetry is a hand-grenade.

3. **`exec.Command("sudo", "-S", program, args...)` with
   `SudoPass + "\n"` on stdin is a sudo-prompt pattern that's
   re-implemented in at least three packages** (here, in
   `internal/actions/copy/handler.go` via `executeSudoCommand`,
   and probably elsewhere — `grep -rn 'sudo.*SudoPass'` to
   audit). A package-level helper in `internal/security` or
   `internal/sudoexec` would centralize the policy of "how do we
   shell with elevation" so password-handling rules stay in one
   place.

## Suggested fix

Stage 1 (in-package):

```go
// becomeAwareCommand builds a *exec.Cmd for `program args...` and
// front-ends it with `sudo -S` when step.ShouldBecome() is true.
// Returns an error early if become is requested but the host
// doesn't support it or no sudo password is configured.
func becomeAwareCommand(step config.Step, ec *executor.ExecutionContext, program string, args ...string) (*exec.Cmd, error) {
    if !step.ShouldBecome() {
        return exec.Command(program, args...), nil
    }
    if !security.IsBecomeSupported() {
        return nil, &executor.SetupError{Component: "become", Issue: fmt.Sprintf("not supported on %s", runtime.GOOS)}
    }
    if ec.Svc.SudoPass == "" {
        return nil, &executor.SetupError{Component: "sudo", Issue: "no password provided. Use --sudo-pass flag"}
    }
    sudoArgs := append([]string{"-S", program}, args...)
    cmd := exec.Command("sudo", sudoArgs...)
    cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
    return cmd, nil
}

// runBecomeAware is becomeAwareCommand + CombinedOutput + standard
// CommandError wrapping. Returns (stdout+stderr, error).
func runBecomeAware(step config.Step, ec *executor.ExecutionContext, what string, program string, args ...string) ([]byte, error) {
    cmd, err := becomeAwareCommand(step, ec, program, args...)
    if err != nil {
        return nil, err
    }
    out, err := cmd.CombinedOutput()
    if err != nil {
        exit := 1
        if cmd.ProcessState != nil { exit = cmd.ProcessState.ExitCode() }
        return out, &executor.CommandError{ExitCode: exit, Cause: fmt.Errorf("%s failed: %w (output: %s)", what, err, string(out))}
    }
    return out, nil
}
```

Each call site collapses to ~3 lines. `executeLaunchctlCommand`
becomes a thin wrapper that adds idempotency-check matching.
`getSystemdServiceState` / `isSystemdServiceEnabled` use
`becomeAwareCommand` + `cmd.Output()` and ignore the result like
they do today.

Stage 2 (cross-package):

Move `becomeAwareCommand` / `runBecomeAware` to a new
`internal/security` (or `internal/sudoexec`) helper. Callers in
`copy`, `service`, anywhere else doing `exec.Command("sudo", "-S",
…)` migrate. Single source of truth for "this is how we shell
with elevation."

## Verification

- `go test ./internal/actions/service/...`
- `make budget-status` — `service` LOC drops below 1,500.
- `grep -rn 'sudo.*-S' internal/` — fewer hits after Stage 2.

## References

- `internal/actions/copy/handler.go executeSudoCommand` — same
  pattern, different package.
- `internal/security.IsBecomeSupported` — already the gate, just
  needs to be invoked consistently.
