---
id: F016
title: agentd.Worker.executeRun calls apply.Runner with context.Background() — applies cannot be cancelled, daemon shutdown blocks indefinitely
severity: risk
package: internal/agentd
file: internal/agentd/worker.go
lines: 166-173, 96-99
status: done
resolved_by: worktree-fix-f016 (7c79da7)
shipped: stage-1(a) — ctx threaded worker→apply.Runner→executor.Start; step loop checks ctx between steps; Shutdown cancels runCtx. Stage-3 handler-level audit (shell exec.CommandContext, wait.* honors ctx, etc.) remains a follow-up. Per-run Deadline (stage 2) deferred.
verified: 2026-05-16 on master @ 7c79da7 (pre-fix verification preserved below)
---

## Pre-fix verification (2026-05-16, master @ 01f5cac)

The agentd-worker case was unchanged from the original finding
at the moment the fix landed:

- `internal/agentd/worker.go:185` called
  `apply.NewRunner(...).Run(context.Background())`.
- `internal/agentd/worker.go:96-99` `Shutdown()` was
  `close(w.submit); <-w.done` — no cancellation propagation.
- `internal/agentd/server.go:259-260` acknowledged the problem
  in a comment: *"Drain in-flight runs. v1 has no
  cancellation, so this may block until the current plan
  finishes."* The 30s `shutdownCtx` it built at line 260 was
  passed to the HTTP servers' `Shutdown` but NOT to
  `worker.Shutdown()`, so a hung run defeated the daemon-level
  timeout entirely.

### The ctx-propagation chain had FOUR breakages, not one (pre-fix)

Post-F020 the CLI plumbed ctx into `apply.Runner.Run(ctx)`. But
the Go ctx was dropped at four further layers:

```
✓ cmd: runWithSignalCtx (signal → cancel ctx)
✓ cmd: applyCommand → apply.NewRunner(cfg).Run(ctx)
✗ agentd: worker.executeRun → apply.NewRunner(cfg).Run(context.Background())   ← F016 stage-1
✗ apply: Runner.Run(ctx) → executor.Start(startConfig, log, publisher)         ← executor.Start had no ctx parameter
✗ executor: runner.Run(ec, &step)                                              ← actions.Context interface carried no Go context
✗ shell: setupCommandContext → cmdCtx := context.Background()                  ← handler discarded any parent and started fresh
```

So F016 stage 1 (worker → Runner) was only the *first* step.
Stage 1(a) per the resolved_by line covers worker→Runner →
executor.Start signature change, plus between-step ctx checks
inside executor. Handler-level audit (stage 3) and per-run
Deadline (stage 2) are tracked as follow-ups.

### Pre-fix failure mode

With F020 fixed but F016 still open, the agentd shutdown path
was:

1. SIGTERM → `cmd/agentd.go:124` cancels outer ctx
2. `Server.Serve(ctx)` unblocks → `Server.shutdown()`
3. HTTP servers `Shutdown(shutdownCtx)` — 30s budget, drains cleanly
4. `worker.Shutdown()` — blocks on `<-w.done`
5. In-flight `apply.Runner.Run(context.Background())` keeps running
6. If the run took > 30 s of post-SIGTERM time (or hung
   forever on a stuck wait.http), daemon stayed alive
   indefinitely. Operator escalated to SIGKILL — `events.jsonl`
   not flushed, `result.json` not written, hub not closed, F015
   defers all skipped (same end-state as pre-F020).

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
