---
id: F015
title: agentd.Worker.executeRun cleanup is asymmetric — store.Update / panic paths leak the hub
severity: smell
package: internal/agentd
file: internal/agentd/worker.go
lines: 118-120, 135-160
status: done
fixed: 2026-05-16 — merge `0b6acf22 merge: fix-F015 — unify agentd worker hub cleanup via single defer`. executeRun now reserves the hub at the top and unconditionally closes it via a single defer, removing the asymmetric Update/panic paths that previously leaked.
verified: 2026-05-17 — confirmed fixed on master @ 099ee336. Dedicated regression test `internal/agentd/worker_chdir_test.go:31 TestWorkerChdirFailureClosesHub` passes; this test (per its header comment) exercises "the F015 path" — chdir failure inside executeRun must still close the hub. `defer w.hubMu.Unlock()` at worker.go:95 + the single deferred hub-close handle the store.Update and panic exits.
---

## ✅ Fixed

`hub.Close()` is now hoisted into a unified defer that runs on every
exit path of `executeRun` (store.Update failure, sink creation failure,
chdir failure, normal success, normal apply failure, panic). The
explicit `hub.Close()` on the sink-creation-error path is removed —
the deferred Close subsumes it (Hub.Close is idempotent so the
sink-cascade Close on chdir/apply-runner paths stays harmless).

The defer orders `hub.Close()` BEFORE `delete(w.hubs, runID)` so a
subscriber that called `GetHub()` concurrently with the delete sees a
closed hub and lands on `Subscribe()`'s "already closed" branch (which
closes the subscriber's channel immediately), rather than a
stale-but-open one.

### Severity correction

The original finding called this a bug and claimed the chdir-error
path leaked the hub. After verification that claim is incorrect:
`RunEventSink.Close()` (`jsonl_sink.go:122`) already calls
`s.hub.Close()`, so the chdir-error path's `sink.Close()` cascades
into `hub.Close()`. Same for the normal-path apply.Runner exit, which
calls `sub.Close()` on every `ExtraSubscribers` entry
(`apply/runner.go:195`). The leak the finding described didn't exist.

Two paths *did* leak pre-fix, both of which the defer now covers:

1. **`store.Update` failure at line 118-120** — returns before the
   hub lookup, so neither `sink.Close()` nor an explicit
   `hub.Close()` runs. Subscribers attached between Submit and
   pickup keep open channels until the daemon exits.
2. **Panic during executeRun** — no defer recovers; the panic
   propagates up the goroutine, and the hub stays open.

Both are rare in practice (store.Update needs a disk/permissions
failure; panics shouldn't happen) but the unified-defer pattern
removes the asymmetry without adding cost.

### Regression test

`internal/agentd/worker_chdir_test.go::TestWorkerChdirFailureClosesHub`
covers the chdir-error path's hub-close contract. Passes both with
and without the unified-defer refactor (today's `sink.Close →
hub.Close` cascade keeps it green), but it's a contract guard against
a future change that unwires the cascade — at that point this fix's
defer becomes the only thing keeping subscribers from leaking on the
chdir path.

---

## What

`executeRun` has two early-return paths between hub registration
and apply invocation. They handle cleanup asymmetrically.

```go
// Path 1 — NewRunEventSink failed (line 141-147):
sink, err := NewRunEventSink(...)
if err != nil {
    w.log.Error(...)
    w.markFailed(run, err.Error(), time.Now().UTC())
    hub.Close()      // ← closes subscribers
    return
}

// Path 2 — os.Chdir failed (line 149-160):
prevDir, _ := os.Getwd()
if run.BaseDir != "" {
    if err := os.Chdir(run.BaseDir); err != nil {
        w.log.Error(...)
        sink.Close()  // ← closes sink, but does NOT close hub
        w.markFailed(run, "chdir base_dir: "+err.Error(), time.Now().UTC())
        return
    }
    defer func() { _ = os.Chdir(prevDir) }()
}
```

Path 1 calls `hub.Close()`. Path 2 doesn't. Both paths reach the
deferred `delete(w.hubs, runID)` (line 135-139), which removes
the hub from the worker's index — so **new** subscribers can't
find it — but pre-existing subscribers keep their channel
references.

## Why it's a bug, not a smell

`Hub.Close()` (`sse_hub.go:100`) is what signals subscribers that
the stream is done:

```go
func (h *Hub) Close() {
    h.mu.Lock()
    defer h.mu.Unlock()
    if h.closed {
        return
    }
    h.closed = true
    for _, sub := range h.subscribers {
        close(sub.ch)
    }
    h.subscribers = nil
}
```

If a subscriber's channel never closes, the SSE handler that
subscribed (via `Subscribe()`) blocks forever waiting for events
that will never arrive — `executeRun` is gone, the worker has
moved on, but the subscriber's goroutine is stuck.

Race window:

1. Controller calls Submit() → hub registered.
2. Controller subscribes to SSE
   `/v1/runs/{id}/events` → `GetHub(runID)` returns the hub →
   subscriber gets a channel.
3. Worker dequeues, tries `os.Chdir(run.BaseDir)`, fails.
4. `sink.Close()` runs but `hub.Close()` does not.
5. `executeRun` returns. Deferred delete removes the hub from the
   map but does not close it.
6. Subscriber's goroutine reads from a channel that will never
   receive another message and never close. Goroutine leak.

The controller side eventually times out or the user Ctrl-Cs.
But on the daemon side, the goroutine sticks around until the
daemon process exits.

Frequency: rare in practice (chdir to a synced plan dir usually
works) but the failure mode is real — a stale `base_dir` that
got deleted between submit and pickup will trigger it.

## Suggested fix

Move `hub.Close()` into a defer that runs on every executeRun
exit path. This puts it on a single line of cleanup logic and
removes the asymmetry:

```go
func (w *Worker) executeRun(runID string) {
    // ... stats, Update(Running) ...

    w.hubMu.Lock()
    hub, ok := w.hubs[runID]
    if !ok {
        hub = NewHub()
        w.hubs[runID] = hub
    }
    w.hubMu.Unlock()

    // Hub lifetime is tied to executeRun: every exit path (success,
    // failure, panic) must close the hub so subscribers' channels
    // signal end-of-stream. Close before removing from the map so a
    // subscriber that called GetHub() concurrently with the delete
    // observes a closed hub rather than a stale-but-open one.
    defer func() {
        hub.Close()
        w.hubMu.Lock()
        delete(w.hubs, runID)
        w.hubMu.Unlock()
    }()

    // ... rest of executeRun, with explicit hub.Close() calls removed ...
}
```

Note the ordering: `hub.Close()` runs *before* `delete()`. A
subscriber that called `Subscribe()` between the Close() and the
delete() lands on the "already closed" branch (line 54-58 in
`sse_hub.go`) which closes the subscriber's channel immediately
— that's the right behavior.

Also delete the redundant `hub.Close()` at line 145 and the
ad-hoc nature of the cleanup goes away.

## Verification

- Add a test that exercises the chdir-error path and asserts the
  hub closes:

```go
func TestWorkerChdirFailureClosesHub(t *testing.T) {
    // submit a run with BaseDir = "/nonexistent/path"
    // subscribe via GetHub before the worker picks up
    // worker.Submit + worker.Run() in a goroutine
    // verify the subscriber's channel closes within a deadline
}
```

- `go test -race ./internal/agentd/...` after the fix.
- Manually: `mooncake fleet apply` with a forged plan whose
  base_dir got deleted between submit and pickup; the SSE
  connection should disconnect cleanly instead of hanging.

## Adjacent observation

`executeRun`'s normal-path completion (lines 178-193) also lacks
an explicit `hub.Close()`. The deferred `delete(w.hubs, runID)`
removes the hub from the map, but the hub isn't closed. Same
underlying issue: subscribers attached during apply.Runner.Run()
keep their channels open after the worker returns. This is
mitigated in practice by the SSE handler's own context wiring —
when the run reaches terminal state the handler closes its
connection — but the unified defer above is cleaner.

## References

- `internal/agentd/sse_hub.go:98-111` — `Hub.Close()` semantics.
- Spec-49 comment in `worker.go:50-55` explains the eager-hub
  pattern; this finding extends the lifecycle invariant.
