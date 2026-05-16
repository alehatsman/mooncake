---
id: F015
title: agentd.Worker.executeRun chdir-error path leaks the hub — SSE subscribers hang forever
severity: bug
package: internal/agentd
file: internal/agentd/worker.go
lines: 149-160
status: open
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
