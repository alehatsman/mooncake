---
id: F016
title: agentd.Worker.executeRun calls apply.Runner with context.Background() — applies cannot be cancelled, daemon shutdown blocks indefinitely
severity: risk
package: internal/agentd
file: internal/agentd/worker.go
lines: 166-173, 96-99
status: open
verified: 2026-05-16 on master @ 01f5cac (post-F020 merge)
---

## Verification (2026-05-16, master @ 01f5cac)

The agentd-worker case is unchanged:

- `internal/agentd/worker.go:185` still calls
  `apply.NewRunner(...).Run(context.Background())`.
- `internal/agentd/worker.go:96-99` `Shutdown()` is still
  `close(w.submit); <-w.done` — no cancellation propagation.
- `internal/agentd/server.go:259-260` even acknowledges the
  problem in a comment: *"Drain in-flight runs. v1 has no
  cancellation, so this may block until the current plan
  finishes."* The 30s `shutdownCtx` it builds at line 260 is
  passed to the HTTP servers' `Shutdown` but NOT to
  `worker.Shutdown()`, so a hung run defeats the daemon-level
  timeout entirely.

### The ctx-propagation chain has FOUR breakages, not one

Post-F020 the CLI plumbs ctx into `apply.Runner.Run(ctx)`. But
the Go ctx is dropped at four further layers before it could
ever cancel a running shell child:

```
✓ cmd: runWithSignalCtx (signal → cancel ctx)
✓ cmd: applyCommand → apply.NewRunner(cfg).Run(ctx)
✗ agentd: worker.executeRun → apply.NewRunner(cfg).Run(context.Background())   ← F016
✗ apply: Runner.Run(ctx) → executor.Start(startConfig, log, publisher)         ← executor.Start has no ctx parameter
✗ executor: runner.Run(ec, &step)                                              ← actions.Context interface (interfaces.go:44-243) carries no Go context
✗ shell: setupCommandContext → cmdCtx := context.Background()                  ← handler discards any parent and starts fresh (handler.go:310)
```

`grep -n 'GoContext\|context.Context' internal/actions/interfaces.go`
returns nothing — the `actions.Context` interface (44 methods)
intentionally hides Go's `context.Context` from handlers. Every
handler that needs cancellation has to invent its own (shell
does via `setupCommandContext` from `context.Background()`).

So F016 stage 1 (worker → Runner) is the right *first* step but
not sufficient. Stage 2 (executor.Start signature) and a stage 3
(actions.Context exposes a Go ctx) are required before a
SIGTERM-mid-run actually stops a hung `sleep 30`.

### Concrete failure today

With F020 fixed, the agentd shutdown path now reads:

1. SIGTERM → `cmd/agentd.go:124` cancels outer ctx
2. `Server.Serve(ctx)` unblocks → `Server.shutdown()`
3. HTTP servers `Shutdown(shutdownCtx)` — 30s budget, drains cleanly
4. `worker.Shutdown()` — blocks on `<-w.done`
5. In-flight `apply.Runner.Run(context.Background())` keeps running
6. If the run takes > 30 s of post-SIGTERM time (or hangs
   forever on a stuck wait.http), daemon process stays alive
   indefinitely. Operator escalates to SIGKILL — `events.jsonl`
   not flushed, `result.json` not written, hub not closed, F015
   defers all skipped (same end-state as pre-F020).

Severity `risk` still fits: same observable as F020 pre-fix
(daemon hangs on hung apply), just via a different mechanism.
F020 fixed the os.Exit shortcut; F016 fixes the missing
cancellation plumbing.

### Not a follow-up finding — fold the chain observation into F016

The four breakage points above are all "the same bug": Go ctx
isn't threaded through the apply → executor → handler stack.
Splitting into multiple findings would over-fragment. The
existing F016 fix sketch (stage 1: worker → Runner) is the
right starting point; the fix PR should either land all four
together or be honest in its commit message about which layers
still drop ctx.

---

## What

```go
kr, execErr := apply.NewRunner(&apply.Config{
    ConfigPath:       run.PlanPath,
    VarsFiles:        run.VarsFiles,
    Tags:             run.Tags,
    Names:            run.Names,
    OutputFormat:     "quiet",
    ExtraSubscribers: []events.Subscriber{sink},
}).Run(context.Background())
```

`context.Background()` means:

1. The apply has **no deadline**. A stuck step
   (`wait.http` with no timeout, a hung subprocess, a deadlocked
   container) blocks the worker indefinitely. The worker is
   single-goroutine — every other queued run waits behind it.
2. The apply **cannot be cancelled**. The daemon's `Shutdown()`
   (`worker.go:96-99`) closes the submit channel and waits for
   the in-flight run to finish, but provides no path to interrupt
   that run. If the in-flight run is hung, daemon shutdown hangs
   too. SIGTERM has to escalate to SIGKILL.

The Submit comment (line 94-95) explicitly documents this:

> Per the v1 plan there is no cancellation: in-flight runs run to
> completion.

That's a design choice, but the consequence — daemon shutdown
hanging on a hung apply — isn't surfaced anywhere user-visible.

## Why it's `risk` and not just a design note

The fleet controller has `fleet.Apply` (F014) that imposes its
own deadlines on the SSE stream, but **the daemon doesn't honor
those deadlines on the run itself**. So:

- Controller sets a 10-minute timeout on apply.
- Daemon receives the run, starts apply.Runner with
  `context.Background()`.
- A step hangs.
- Controller's 10-minute timer expires; it closes the SSE
  connection.
- Daemon's apply.Runner keeps running. The hub still gets events,
  but no subscriber.
- Worker queue blocked.

A subsequent operator who tries to `mooncake fleet apply ...
mycontroller` sees their submit succeed (queueDepth++) but no
events ever fire because the worker is stuck on the previous run.

## Suggested fix

**Stage 1 — wire context.Context through the worker so
Shutdown can cancel:**

```go
type Worker struct {
    // ...existing...
    runCtx    context.Context
    runCancel context.CancelFunc
}

func NewWorker(...) *Worker {
    ctx, cancel := context.WithCancel(context.Background())
    return &Worker{
        // ...
        runCtx:    ctx,
        runCancel: cancel,
    }
}

func (w *Worker) Shutdown() {
    close(w.submit)
    w.runCancel()  // signal in-flight applies to wind down
    <-w.done
}

// in executeRun:
kr, execErr := apply.NewRunner(...).Run(w.runCtx)
```

This makes Shutdown a graceful-stop signal: in-flight applies
get a cancelled context and can abandon (handlers that respect
ctx will return; handlers that don't will be observed and called
out in a follow-up).

**Stage 2 — optional, per-run deadline:**

```go
ctx := w.runCtx
if run.Deadline != nil {
    var cancel context.CancelFunc
    ctx, cancel = context.WithDeadline(ctx, *run.Deadline)
    defer cancel()
}
kr, execErr := apply.NewRunner(...).Run(ctx)
```

`Run.Deadline` is a new field on the persisted run record
(populated from a controller-side `--deadline` flag, or absent).
Lets the controller bound runaway applies.

**Stage 3 — audit handlers for ctx-respect:**

`shell`, `wait.*`, `tool.fetch` (F007), `assert` (HTTP probe),
`observe.http`, `download`, `pkg`, `git.clone` are all candidates.
A handler that ignores ctx makes Stage 1 only partially
effective. This is a per-handler audit, related to F012
(http-no-context) but broader.

## Trade-off

The current "no cancellation" design has a real benefit:
applies are atomic from the daemon's perspective; you never get
a partially-cancelled half-finished apply with no Reverse run.
That's load-bearing for the kernel-guarantee story.

Stage 1's cancellation **interrupts apply.Runner mid-execution**
without invoking Reverse on completed steps. That's a behavior
change. Two ways to make it tolerable:

- (a) Cancellation signals the runner but the runner finishes
  the current step then exits cleanly with a "cancelled" status.
  No mid-step interruption. Compatible with existing handlers.
- (b) Cancellation signals each handler, and the runner runs
  Reverse on completed steps in LIFO order. Matches the
  transaction-rollback semantic.

(a) is the smaller change. (b) is the long-run right answer for
transactional applies.

## Verification

- `go test ./internal/agentd/...`
- Manual: `mooncake-daemon` running a plan with `shell: sleep 600`,
  then send SIGTERM to the daemon. Today: daemon hangs ~600s.
  After (a): daemon waits for the sleep to finish but new
  submissions are rejected. After (b): daemon cancels mid-sleep
  and runs any Reverse paths.

## References

- F014 — `fleet.Apply` post-stream WithoutCancel: complementary
  layer; the controller side already has the cancellation
  primitives in place.
- F012 — HTTP-no-context. Several handlers also discard ctx,
  which would defeat Stage 1.
