# Spec 63: `observe.*` Streaming / Subscription Mode

**Status:** Draft (depends on spec-59; revisit only if a real consumer surfaces)
**Epic:** E9 Modern Action Surface — bucket E9.4 (observability extensions)
**Effort:** M (cross-cutting design, not a per-handler add)
**Value:** Speculative. No current consumer; this spec exists to record
the design boundaries *if* someone later asks for "long-running
observe → fire-on-change" inside a plan step.

**Design principles:** [`action-design-principles.md`](../../action-design-principles.md) + [`non-goals.md`](../../non-goals.md)

---

## Problem

[`spec-59`](./spec-59-typed-observability.md) ships point-in-time
observers — single-shot reads with typed output. The natural follow-on
question is: *what if I want to react when an observation changes,
not just check its current value?*

Two concrete shapes that might motivate this:

1. **Restart-on-port-change.** Watch `observe.port :80` and run a
   step when the listener changes (e.g. a process replaced itself).
2. **Tail-and-react.** Watch `observe.logs --follow` for a pattern,
   trigger a remediation step when matched.

Today, both shapes have working alternatives:

- The polling cousin is `wait.*` (spec-29 ✅), which loops until a
  predicate flips. Combined with `for_each` over a retry list, it
  covers most "wait until ready" needs.
- For continuous behavior, the right primitive is **drift detection
  on agentd** (spec-58), which polls observers on a daemon-side
  cadence and acts via the drift policy (`notify | reapply | revert`).
  This already gives "long-running observe → action" semantics, just
  on a different layer than per-step.

So: **a streaming observation primitive inside a plan step is
probably wrong** — it pulls long-running behavior into the plan
executor, which is structured for sequential mutation. The right
home for "watch state and react" is the drift daemon, not the plan.

This spec exists to make that judgment explicit and record the
boundary, not to ship the feature.

---

## Decision (current)

**Do not build streaming observers into the plan executor.** Use one
of these instead, by use case:

| Use case | Today's primitive |
|---|---|
| Wait until a condition becomes true, then continue | `wait.*` (spec-29) |
| Continuously enforce a desired state | spec-58 drift loop |
| Single-shot read for the current step | spec-59 `observe.*` |
| React only when a specific run's prior step changed something | spec-23 `on_change:` |
| Snapshot state for later comparison | spec-04 snapshot + spec-14 diff |

If a real consumer surfaces that doesn't fit any of those, revisit
this spec with the specific shape in mind.

---

## Open question: predicate-driven assert without polling

The one shape that *might* not fit existing primitives: a one-shot
"observe this thing, but only once it crosses a typed threshold
within window W." Today's pattern is `wait.command` running a custom
script, but a typed version (`wait.observe`?) could close that gap
without becoming a streaming primitive.

**Lean:** defer this until a real example. The shape is small enough
that it's not worth speccing speculatively.

---

## Non-goals (explicit)

Per [`non-goals.md`](../../non-goals.md), this spec restates a few
explicit boundaries grounded in the historical-systems audit:

1. **No reactor / event-DSL layer.** SaltStack's reactor system is
   the canonical failure mode for "react to events from inside the
   tool." Observers emit events to the SSE stream; consumers
   subscribe and run their own programs. Mooncake's job is to
   *emit* honestly, not to define the consumer.
2. **No long-running plan steps.** Plans are sequences of finite
   mutations + observations. A step that runs for the lifetime of
   the daemon breaks every assumption (timeouts, retries, recap
   accounting). If continuous behavior is needed, it lives on the
   daemon (spec-58), not in the plan.
3. **No streaming over MCP.** The MCP tools return typed payloads;
   they don't open subscriptions. If an agent wants "watch and
   react," it polls.

---

## Cross-references

- [`spec-59-typed-observability.md`](./spec-59-typed-observability.md) — the point-in-time primitive this spec deliberately does NOT extend.
- [`spec-29-wait-primitives.md`](../done/spec-29-wait-primitives.md) — the polling-until-condition primitive that already exists.
- [`spec-58-fleet-drift.md`](../personal-fleet/spec-58-fleet-drift.md) — the right home for "continuously enforce state."
- [`spec-23-framework-primitives.md`](../done/spec-23-framework-primitives.md) — `on_change:` is the "react only when a step changed something" primitive.
- [`non-goals.md`](../../non-goals.md) — the discipline this spec defends.
