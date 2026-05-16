---
id: F016
title: agentd.Worker.executeRun calls apply.Runner with context.Background() — applies cannot be cancelled, daemon shutdown blocks indefinitely
severity: risk
package: internal/agentd
file: internal/agentd/worker.go
lines: 166-173, 96-99
status: open
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
