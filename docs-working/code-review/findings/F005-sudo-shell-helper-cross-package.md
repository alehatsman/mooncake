---
id: F005
title: Cross-package: 6 distinct implementations of "shell out with sudo -S"
severity: smell
package: cross-cutting
files:
  - internal/actions/service/handler.go (×12 call sites + 1 partial helper)
  - internal/actions/package/handler.go (Handler.runCmd)
  - internal/actions/download/handler.go:484
  - internal/actions/template/handler.go:346
  - internal/effects/default.go (defaultPerformer.runSudo)
  - internal/executor/sudo_integration_test.go (test only)
status: open
---

## What

`grep -rn 'sudo.*-S' internal/ --include='*.go'` returns 17 hits
across 6 packages. Each implements the same "if become, prepend
`sudo -S` and pipe sudo password on stdin" pattern, but with
inconsistent contracts:

| Package | Helper / call shape | Validates `IsBecomeSupported`? | Validates SudoPass empty? |
|---|---|---|---|
| `effects/default.go`     | `defaultPerformer.runSudo(command)` | ✅ yes | ❌ no (would pass empty string + `\n`) |
| `actions/package`        | `Handler.runCmd(ec, become, args)` | ❌ no | ❌ no |
| `actions/service` (6×)   | inline                              | mixed (4/6) | ✅ yes |
| `actions/download`       | inline (`sudo -S sh -c command`)    | ❌ no | ❌ no |
| `actions/template`       | inline (`sudo -S sh -c command`)    | ❌ no | ❌ no |

That table is the bug surface. The "validates" inconsistency
means a become-requested-but-unsupported run produces:

- A clean `SetupError` in `effects` and 4 of 6 `service` sites.
- An OS-level "command not found" via `exec.Command("sudo", ...)`
  in `package`, `download`, `template`, and 2 of 6 `service` sites.

Worse, the empty-SudoPass case in `package`, `download`, `template`,
`effects/default.go` produces `cmd.Stdin = bytes.NewBufferString("\n")`,
which makes sudo prompt-and-hang (when it has a TTY) or fail with
"no password" (when it doesn't), instead of returning a
diagnosable error to the caller.

## Why this is worth a finding (not just a code-style nit)

The handlers run **as the moat** of the mooncake kernel (per
`docs-working/vision/kernel.md`). Sudo-elevation is one of the
load-bearing pieces of that moat. Six independent implementations
of a sensitive shell-out (sudo password on stdin, exit-code
propagation, stderr capture) is exactly the situation the kernel
boundary is meant to prevent.

This is also the kind of latent issue that doesn't fire until a
specific user hits it: someone with `become: true` on a host
without sudo installed, or someone running mooncake from an
unattended job with no `--sudo-pass` set. Each handler will
react differently. That's a confusing-bug-report machine.

## Suggested fix

Stage 1 — define the canonical helper in `internal/security`
(it already has `IsBecomeSupported`):

```go
// internal/security/become.go (new file)

package security

// BecomeRunner is the project-wide policy for "run a command with
// optional sudo -S elevation." All handlers that shell out under
// `become: true` must use it; direct exec.Command("sudo", "-S", …)
// in handler code is a layering violation.
type BecomeRunner struct {
    SudoPass string
}

// Command builds an *exec.Cmd for `program args...`, prepending
// `sudo -S` and wiring SudoPass to stdin when become is requested.
//
// Returns &SetupError when:
//   - become is true and IsBecomeSupported() is false
//   - become is true and SudoPass is empty
//
// Caller invokes cmd.CombinedOutput() / cmd.Run() as usual.
func (r BecomeRunner) Command(become bool, program string, args ...string) (*exec.Cmd, error) {
    if !become {
        return exec.Command(program, args...), nil //nolint:gosec // provisioning tool runs user-defined programs
    }
    if !IsBecomeSupported() {
        return nil, fmt.Errorf("become not supported on %s", runtime.GOOS)
    }
    if r.SudoPass == "" {
        return nil, errors.New("become requested but no sudo password configured (use --sudo-pass)")
    }
    sudoArgs := append([]string{"-S", program}, args...)
    cmd := exec.Command("sudo", sudoArgs...) //nolint:gosec
    cmd.Stdin = bytes.NewBufferString(r.SudoPass + "\n")
    return cmd, nil
}
```

(The exact error type can match the existing `executor.SetupError`
once the import direction is reviewed — see "Open questions" below.)

Stage 2 — migrate the 17 call sites. Pattern:

```go
// Before
var cmd *exec.Cmd
if become {
    cmd = exec.Command("sudo", append([]string{"-S"}, args...)...)
    cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
} else {
    cmd = exec.Command(args[0], args[1:]...)
}
return cmd.CombinedOutput()

// After
runner := security.BecomeRunner{SudoPass: ec.Svc.SudoPass}
cmd, err := runner.Command(become, args[0], args[1:]...)
if err != nil { return nil, err }
return cmd.CombinedOutput()
```

Stage 3 (cleanup):

- Delete `defaultPerformer.runSudo` (`internal/effects/default.go:556`)
- Delete `Handler.runCmd` (`internal/actions/package/handler.go:382`)
- Delete the inline copies in `download`, `template`, `service` (×6)

## Open questions

- **Import direction.** `internal/security` is leaf-ish in the
  dependency graph. `internal/executor.SetupError` is the
  current error type these handlers return; using it requires
  `security` to import `executor` (probably a cycle). Two
  options:
  1. New error type in `internal/security`. Handlers wrap into
     `executor.SetupError` at the call site. Tidy but verbose.
  2. Move `SetupError` to a shared package (e.g. `internal/errors`
     which already exists). Better long-run, costs more LOC churn
     in this PR.
- **`actions/download` and `actions/template`** pass a shell
  string (`sudo -S sh -c command`), not a program+args. Decide
  whether the helper supports both shapes or whether those
  callers move to `exec.Command("/bin/sh", "-c", command)` via
  the helper.

## Expected payoff

- **Deletes ~150 LOC** across the 6 packages.
- Makes "what happens when become is requested but unsupported"
  exactly one code path.
- Makes adding a future elevation backend (`run0`, `doas`, polkit)
  a one-file change instead of six.

## Verification

- `go test ./...` — same pass set.
- `grep -rn 'sudo.*-S' internal/ --include='*.go'` — only hits in
  `internal/security/` and tests.
- Manual: run a mooncake config with `become: true` on a host
  without sudo; should get the same clean `SetupError` regardless
  of which handler fires.

## References

- F004 is the in-package version of this (service-only). F005
  scope is cross-package.
- `docs-working/vision/kernel.md` — kernel boundary rationale.
