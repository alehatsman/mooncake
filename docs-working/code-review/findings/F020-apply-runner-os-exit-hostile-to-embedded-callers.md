---
id: F020
title: apply.Runner.installSignalHandler calls os.Exit — hostile to embedded callers (agentd, MCP, SDK)
severity: risk
package: internal/apply
file: internal/apply/runner.go
lines: 154-155, 336-359
status: open
---

## What

`apply.Runner.Run` (line 154) installs a SIGINT/SIGTERM handler:

```go
stopSig := installSignalHandler()
defer stopSig()
```

And the handler (line 336-359):

```go
func installSignalHandler() (stop func()) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
    done := make(chan struct{})
    go func() {
        select {
        case sig := <-sigCh:
            fmt.Fprintf(os.Stderr, "\n⚠ received %s, aborting apply\n", sig)
            signal.Stop(sigCh)
            code := 130
            if sig == syscall.SIGTERM {
                code = 143
            }
            os.Exit(code)
        case <-done:
        }
    }()
    return func() { /* unregister */ }
}
```

For the CLI (`mooncake apply`), this is the right behavior: Ctrl-C
should terminate the process cleanly with the standard exit code.

For **every other apply.Runner caller**, it's wrong:

- **`agentd` worker** (`internal/agentd/worker.go:166`) calls
  `apply.NewRunner(...).Run(ctx)`. If the daemon receives SIGTERM
  (e.g. systemd stopping the service), apply.Runner's handler
  fires *first* and calls `os.Exit(143)`. The daemon's own
  `Worker.Shutdown()` (worker.go:96-99) — which closes the
  submit channel and waits for the in-flight run — never runs.
  Result.json is not written. Subscribers don't get
  `subscriber.Close()`. Hub doesn't get its Close() (F015).
  The whole graceful-shutdown story dies.
- **MCP** (`internal/mcp/tools.go`, post-runCollector refactor)
  also calls `apply.Runner`. An MCP server receiving SIGTERM
  expects to drain in-flight requests and close cleanly; instead
  any in-flight apply yanks the process out.
- **Any future SDK / library caller** — same hazard.

`apply.Runner` is the kernel boundary. The kernel doesn't get
to decide that signals terminate the process; that's the
embedding shell's job.

## Why it's `risk`

Concrete failure mode:

```sh
# Terminal A
systemctl start mooncake-daemon

# Terminal B
mooncake fleet apply -p plan.yaml  # apply takes 5 minutes

# Terminal A, 30 s into apply
systemctl stop mooncake-daemon
```

Expected: daemon stops accepting new submissions, in-flight
apply completes (or is cancelled cleanly), result.json is
written, then daemon exits.

Actual: SIGTERM hits daemon → apply.Runner's signal goroutine
fires → `os.Exit(143)` → daemon process gone. Controller side:
SSE connection drops mid-run, no terminal status, no
result.json on disk. From the operator's perspective the run is
in an inconsistent state.

## Suggested fix

Move signal handling **out of apply.Runner** and into the CLI
caller. apply.Runner should respect `ctx` for cancellation;
the caller decides what cancels `ctx`.

```go
// Before — runner.go:64:
func (r *Runner) Run(ctx context.Context) (*KernelResult, error) {
    // ... validate, set up publishers ...
    stopSig := installSignalHandler()   // ← delete this
    defer stopSig()
    // ...
}

// After — runner.go drops installSignalHandler entirely.
// Callers wire ctx as appropriate.
```

CLI side (`cmd/mooncake.go`'s apply command):

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()
result, err := apply.NewRunner(cfg).Run(ctx)
if err != nil && ctx.Err() != nil {
    // Cancelled by signal; map to 130/143 exit code here.
    if ctx.Err() == context.Canceled { // signal flavor not preserved by NotifyContext
        os.Exit(130)
    }
}
```

`signal.NotifyContext` is Go 1.16+ stdlib; it's the idiomatic
shape for "make this context get cancelled on SIGINT/SIGTERM."
The exit-code decision becomes the caller's, where it belongs.

agentd side: nothing to do — Worker already calls
`apply.Runner.Run(ctx)` (with `context.Background()` today; F016
proposes wiring a real ctx). After F020+F016 together, the daemon
controls its own shutdown deterministically.

MCP / SDK: same.

## Cancellation semantics still need work

After F020 alone, `ctx.Done()` on apply.Runner is *observed at
the API boundary* (the runner returns) but the executor's hot
loop still doesn't check ctx. Today, `executor.Start` runs to
completion regardless of ctx. So Run() can return (because the
signal handler cancelled the ctx), but executor.Start's
goroutine is still inside `DispatchStepAction`.

The full chain — `apply.Runner.Run(ctx)` → `executor.Start(ctx)`
→ handler `Execute(ctx, step)` → `exec.CommandContext` — needs
to be wired through. That's F016's stage-3 audit work. F020 is
just the API surface fix.

## Adjacent observation

`installSignalHandler` calls `signal.Stop(sigCh)` inside the
handler goroutine *and* in the returned `stop()` func. The race
is small (both call the same idempotent func) but the goroutine
that received the signal can no longer reach the `done` channel
case after `os.Exit` — so the `stop()` func's `close(done)` will
never be observed. That's fine because the process is dying;
just noting it for the rewrite.

## Verification

- After fix: `kill -TERM $(pgrep mooncake-daemon)` mid-run.
  Daemon should run its shutdown sequence, write result.json,
  exit cleanly within a bounded time.
- Existing test in `runner_test.go` does not exercise signals;
  add one that asserts no os.Exit is invoked on Run() with a
  cancelled-by-ctx scenario.

## References

- F015 — depends on this; deferred hub.Close() can't fire if
  apply.Runner kills the process via os.Exit.
- F016 — complementary: worker needs a real ctx to *replace*
  the signal-handler approach.
- `signal.NotifyContext` Go stdlib reference.
