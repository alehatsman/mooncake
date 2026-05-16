---
id: F027
title: agentd.sanityCheckBinary runs the staged binary without a timeout — a hanging `--version` blocks the upgrade handler forever
severity: risk
package: internal/agentd
file: internal/agentd/self_upgrade.go
lines: 256-271
status: done
resolved: 2026-05-16 — added `sanityCheckBinaryTimeout = 5 * time.Second` constant; converted `sanityCheckBinary` to `exec.CommandContext` under that deadline with an explicit `errors.Is(ctx.Err(), context.DeadlineExceeded)` branch that returns `"staged binary timed out on --version after 5s"`. Doc-comment also updated — the original copy claimed "with a short timeout" while the implementation had none (doc-drift). Regression test `TestSanityCheckBinary_HangingBinaryTimesOut` covers the contract: a fake binary that `exec sleep 30`s must return a timeout error within ~5 s, not 30 s. Out-of-scope note: a staged binary that *forks* a subprocess holding stdout/stderr still hangs `CombinedOutput()` because SIGKILL only goes to the direct child; a process-group kill would address that but is a separate platform-specific change. The single-process deadlock scenario the finding describes is fully covered.
verified: 2026-05-16 — tests green, fix shape confirmed by code inspection on worktree-tester
---

## What

`sanityCheckBinary` runs `<staged> --version` to verify a newly
staged daemon binary boots. The exec has **no timeout**:

```go
func sanityCheckBinary(path string) error {
    cmd := exec.Command(path, "--version")
    cmd.Stdin = nil
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(out)))
    }
    if !strings.Contains(strings.ToLower(string(out)), "mooncake") {
        return fmt.Errorf("output %q does not look like mooncake --version", strings.TrimSpace(string(out)))
    }
    return nil
}
```

`CombinedOutput()` blocks until the child process exits. If the
staged binary hangs (deadlock during init, blocking syscall, waiting
on stdin even though Stdin=nil — some binaries probe `os.Stdin`
properties anyway), this call blocks forever.

The caller holds `upgradeMu` for the entire duration
(`selfBinaryHandler` line 67-72) so the daemon refuses every
subsequent `/v1/self/binary` and `/v1/self/replace` request with
`409 upgrade_in_progress`. From the controller's perspective:
the daemon is stuck "almost upgrading", neither succeeding nor
failing.

## Why it's `risk` (not `bug`)

- The mooncake binary itself doesn't hang on `--version`, so this
  doesn't fire today.
- But the staged binary is **untrusted in detail** — the controller
  PUT'd it and the daemon verified SHA256 + OS/arch + name-of-output,
  but the daemon has no way to know the binary won't deadlock until
  it runs it.
- An attacker with controller access AND the bearer token can
  craft a binary that *looks* like mooncake (passes SHA256
  because the SHA256 is the attacker's, prints "mooncake" early
  in output then hangs) and brick the daemon's upgrade path
  without ever taking over execution.
- More mundanely: a corrupted upload that survives the SHA256
  check (e.g. attacker-controlled checksum) but has random
  bytes that happen to spawn a hanging process.

Even without an attacker: a build broken in a way that hangs
during init turns a controller-side `fleet upgrade` into a stuck
daemon.

## Suggested fix

```go
import "context"

func sanityCheckBinary(path string) error {
    // 5 s is generous — `mooncake --version` completes in tens of
    // milliseconds on every supported platform. If the staged
    // binary takes longer, something is wrong.
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    cmd := exec.CommandContext(ctx, path, "--version")
    cmd.Stdin = nil
    out, err := cmd.CombinedOutput()
    if errors.Is(ctx.Err(), context.DeadlineExceeded) {
        return fmt.Errorf("staged binary timed out on --version after 5s")
    }
    if err != nil {
        return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(out)))
    }
    if !strings.Contains(strings.ToLower(string(out)), "mooncake") {
        return fmt.Errorf("output %q does not look like mooncake --version", strings.TrimSpace(string(out)))
    }
    return nil
}
```

`exec.CommandContext` sends SIGKILL to the process when ctx
expires (`os.Process.Kill()`). The 5 s budget gives the
daemon's `selfBinaryHandler` a bounded ceiling; combined with
the explicit deadline-exceeded error, the controller gets a
clear "binary_unhealthy" 400 instead of a hung connection.

## Adjacent observation — reExec failure leaves state divergent

`selfReplaceHandler` (line 248-253) runs the actual
exec-replacement in a goroutine after responding 202 Accepted:

```go
go func() {
    time.Sleep(1 * time.Second)
    if err := reExec(currentPath); err != nil {
        s.log.Error("self-upgrade re-exec failed", "err", err, "binary", currentPath)
    }
}()
```

If `reExec` fails, the daemon stays running on the OLD code in
memory but the NEW binary is on disk. The controller polls
`/v1/version` waiting for a PID change that never comes.

Today's behavior: the controller eventually times out and the
operator has to SSH in to restart manually. Recovery is
documented (the `previous-<ts>` snapshot is preserved) but the
"upgrade silently no-ops on reExec failure" is a UX hazard.

Out of scope for F027 — flagged here because it touches the same
code path. Worth a separate finding if the user wants tighter
upgrade telemetry.

## Verification

- Manual: stage a binary that does `time.Sleep(10*time.Second);
  os.Exit(0)` in its `--version` path. Today: handler hangs ~10 s
  then succeeds. After fix: returns `400 binary_unhealthy:
  staged binary timed out on --version after 5s` within 5 s.
- `go test ./internal/agentd/...` — existing self_upgrade_test.go
  uses a real exec; verify the timeout doesn't break the happy path.

## References

- F016 — broader cancellation gap in agentd worker; same family
  of issue but different code path.
- F012 — http-no-context. Self-upgrade is exec-based, not
  HTTP-based, so the categorization is different but the
  underlying principle (bound external waits) is the same.
