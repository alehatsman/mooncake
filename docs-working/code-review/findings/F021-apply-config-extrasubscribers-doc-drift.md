---
id: F021
title: apply.Config.ExtraSubscribers doc says "closed by publisher" but runner explicitly closes them after Flush
severity: doc
package: internal/apply
file: internal/apply/config.go
lines: 69-75
status: open
---

## What

`apply.Config.ExtraSubscribers` doc-comment (config.go:69-75):

```go
// ExtraSubscribers are additional event subscribers injected into the
// publisher before the run starts. Useful for callers (e.g. agentd)
// that need daemon-specific sinks (SSE hub, events.jsonl writer) that
// are outside the kernel's standard subscriber set.
// Subscribers are closed by the publisher when it closes; callers
// must not close them independently.
ExtraSubscribers []events.Subscriber
```

The runner does the opposite (runner.go:189-197):

```go
// ExtraSubscribers may buffer events in their own goroutines
// (e.g. RunEventSink's writeLoop). Flush() guarantees all OnEvent
// calls are complete, so it is safe to Close them here — their
// internal queues drain and flush to backing stores before this
// function returns. publisher.Close() (deferred) closes channels
// but does NOT call subscriber.Close().
for _, sub := range r.cfg.ExtraSubscribers {
    sub.Close()
}
```

The runner-side comment is the truth: `publisher.Close()` does NOT
call `subscriber.Close()`. The runner explicitly walks
ExtraSubscribers and calls Close on each. The Config doc-comment
is wrong about WHO closes them.

## Why it matters (doc, not bug)

The contract a caller reads is in `config.go`. A user who reads
"closed by the publisher when it closes; callers must not close
them independently" might:

- Implement a subscriber that's not safe to double-close
  (e.g., `Close()` panics on second call).
- Or implement one that requires explicit close to flush
  to disk, and assume the publisher does it. (This is the
  agentd `RunEventSink` case — it works today, but only because
  the runner *does* call Close, contradicting the doc.)

Both consumers would be surprised by the runtime behavior. Today
the runner's behavior is the right one (Flush → Close → publisher
closes); the doc just needs to match.

## Adjacent — `RunEventSink.Close()` is idempotent? Verify.

The runner closes each ExtraSubscriber once. If any
subscriber's `Close()` is non-idempotent and the caller (e.g.
agentd's worker test harness, future code) tries to close it
again — panic.

agentd's `RunEventSink` (`internal/agentd/event_sink.go`, not
read here) needs to be idempotent for safety. Audit candidate.

## Suggested fix

Replace lines 73-74:

```go
// ExtraSubscribers are additional event subscribers injected into the
// publisher before the run starts. Useful for callers (e.g. agentd)
// that need daemon-specific sinks (SSE hub, events.jsonl writer) that
// are outside the kernel's standard subscriber set.
//
// Lifecycle: Runner subscribes them eagerly, calls publisher.Flush()
// after the executor returns (drains all OnEvent calls), then calls
// subscriber.Close() on each in declaration order. publisher.Close()
// only closes its internal channels; it does NOT call Close on
// subscribers. Callers must not call Close themselves — the runner
// owns the lifecycle.
ExtraSubscribers []events.Subscriber
```

Now the doc and the code agree.

## Verification

- Re-read both comments side by side after the change.
- `grep -rn 'ExtraSubscribers' internal/` — every reference uses
  the runner-owned-close model.

## References

- F015 (worker hub lifecycle) — touches the same area; fixes
  there should not introduce a second close of ExtraSubscribers.
