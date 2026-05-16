---
id: F014
title: fleet.Apply uses context.WithoutCancel for terminal-state fetches without a timeout — hangs ignore Ctrl-C
severity: risk
package: internal/fleet
file: internal/fleet/apply.go
lines: 195, 218
status: fixed
---

## ✅ Fixed

Both post-stream HTTP fetches now wrap `context.WithoutCancel(ctx)`
in a 10 s `context.WithTimeout`. The intent stays the same — a
user's Ctrl-C during the SSE stream must not cancel the recovery
probe — but the recovery is now bounded: a hung / unreachable /
stop-paused daemon (e.g. `kill -STOP $pid`) lets Apply return
cleanly with a "context deadline exceeded" error after 10 s
instead of blocking forever.

Cancel is called explicitly after each GetRun / GetRunResult to
release the context promptly; equivalent to `defer cancel()` but
without the per-iteration defer cost on the loop body's exit path.

### Adjacent observation — not addressed

The finding's note about the `sink` channel buffer (size 64,
back-pressure on slow consumers) is correct but unrelated to the
Ctrl-C hang. Tracked separately if it becomes a real symptom.

### No new regression test

The finding's verification recipe (`kill -STOP` a real daemon and
observe Apply returns within 10 s) is solid but requires
infrastructure the unit-test suite doesn't have today (an in-process
fake transport with a settable "GetRun hangs" knob). The existing
real-TCP integration test (`TestApply_RoundTrip`) and the package
race-test suite all still pass. A follow-up could add a
HangingPeer transport stub; not bundled here.

---

## What

Two post-stream HTTP fetches use `context.WithoutCancel(ctx)`:

```go
// line 195 — recover terminal status if the SSE stream closed
// before run.completed fired
if rec, rerr := opts.Peer.GetRun(context.WithoutCancel(ctx), runID); rerr == nil {
    // ...
}

// line 218 — fetch typed KernelResult after terminal status
kr, kerr := opts.Peer.GetRunResult(context.WithoutCancel(ctx), runID)
```

The intent (documented in nearby comments) is sound: even if the
user Ctrl-C'd, surface the final run record / kernel result so the
output isn't an unexplained silence.

The problem: `context.WithoutCancel(ctx)` produces a context that
**never** times out (it inherits values but drops the deadline /
cancellation). If the daemon is hung / unreachable / slow,
`GetRun` and `GetRunResult` block forever. From the user's
perspective, **Ctrl-C does nothing** — `fleet.Apply` never
returns.

## Why it's a real risk

The whole point of `WithoutCancel` here is "don't let the user's
cancellation also cancel our recovery fetch." But that policy is
too strong: the recovery fetch should have its own bounded
deadline so it returns either with the answer or with a clean
"timed out fetching tail."

In practice: a stuck daemon (deadlocked, OOM-killed mid-response,
unresponsive HTTP server) leaves `fleet apply` stuck across all
peers. The multiplexer can't render anything because Apply is
still in its post-stream code path. The user has to SIGKILL.

This matches the F012 cross-cutting pattern: **`internal/fleet/transport`
is one of the packages whose HTTP client should bound by default,
and even where it doesn't, callers should impose deadlines.**

## Suggested fix

Wrap the WithoutCancel context with a bounded deadline:

```go
// Detach from the user's ctx so a Ctrl-C doesn't kill the
// recovery probe, but cap recovery time so a stuck daemon
// doesn't hang Apply indefinitely.
recoveryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
defer cancel()

if rec, rerr := opts.Peer.GetRun(recoveryCtx, runID); rerr == nil {
    // ...
}
```

Same shape for line 218.

Timeout choice: 10 s is generous for an HTTP request that's
expected to take milliseconds. Long enough that a healthy-but-slow
daemon completes; short enough that a stuck daemon doesn't dominate
the user's wait time. If 10 s is wrong, surface it as an
`ApplyOptions.RecoveryTimeout`.

## Adjacent observation (not blocking)

`opts.Peer.Stream(ctx, runID, sink)` (line 168) does the right
thing — it respects the parent ctx. But `sink` is a channel of
size 64 (`make(chan transport.Event, 64)`). If the consumer side
of `Apply` falls behind (slow Writer, slow event subscriber), the
Stream goroutine **blocks** trying to send to `sink`. That doesn't
deadlock today because the `for {}` loop on line 170 is the only
sink-reader and it drives forward unless ctx is done, but it's
worth a load-bearing comment near line 166 explaining the buffer
choice ("64 events absorbs a brief stall; under sustained
back-pressure the Stream goroutine pauses, which is fine because
the daemon's SSE writer will also pause").

## Verification

- Run `mooncake fleet apply` against a daemon that you have just
  `kill -STOP`-ed (suspends without dropping the socket). Today
  the apply hangs forever; after the fix it returns with a
  bounded error.
- `go test ./internal/fleet/...` — no test exercises this path
  today (the suite uses an in-process fake transport that
  responds immediately). Adding one is a good follow-up but not
  blocking.

## References

- F012 — cross-cutting HTTP-no-timeout audit. `internal/fleet/transport`
  was noted as out-of-scope there pending its own review; F014 is
  the at-call-site fix while the transport package itself is being
  audited.
