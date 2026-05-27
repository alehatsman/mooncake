---
id: F054
title: spec-30 rollback-event surface never implemented — transaction rollback is invisible to runlog / history / fleet telemetry
severity: smell
package: internal/executor + internal/runlog + internal/events
file: internal/executor/transaction.go (handleTxnBodyFailure, runReverse) + internal/events/event.go (missing event types) + internal/runlog/runlog.go (StepEntry shape)
lines: transaction.go handleTxnBodyFailure 90-130; runlog.go StepEntry 60-69
status: done
discovered: 2026-05-27 — F053 cold-read of internal/executor/. transaction.go:179 calls dispatchRunner directly for the inverse step, bypassing step.started / step.completed events. Spec-30 §"Key files" line 158 explicitly designed for a six-event surface (transaction_begin / commit / rollback_begin / step_reversed / rollback_complete / rollback_failed) wired into internal/runlog/runlog.go, but the implementation shipped only the executor-side semantics — none of the events were ever added to internal/events/event.go.
fixed: 2026-05-27 — implemented the **hybrid** option: four rollback events (RollbackBegin / StepReversed / RollbackComplete / RollbackFailed) emitted at natural boundaries inside `handleTxnBodyFailure`, plus a per-step `Reverted` flag that flows from `RunCapture.markStepReverted` through `executor.StepRecord.Reverted` into `runlog.StepEntry.Reverted`. `mooncake history` / `explain` / fleet telemetry consumers now see rollback structurally — no more "single ↺ log line is the only signal" gap. TransactionBegin / TransactionCommit deferred: those need compound-parent step.started semantics that today's executor doesn't expose (transaction parents aren't dispatched; only their body / rollback children are). Followup tracked as note inside this finding.
post-fix verified: 2026-05-27 — three new tests: `TestTransaction_F054_RollbackEvents` (end-to-end via real planner + executor: a 3-step transaction failing at step 3 emits the four events in order Begin → StepReversed × 2 → Complete; data carries CompletedSteps=2, FailedStepID/Name, per-reversed Action="file.write", ReversedSteps=2); `TestRunCapture_MarkStepReverted` (capture record flips Reverted=true for the matching step ID, leaves siblings untouched); `TestRunCapture_MarkStepReverted_UnknownID` (defensive: unknown ID is no-op, nil capture receiver doesn't panic). Existing transaction tests (HappyPath / RollbackOnFailure / RollbackRecapMath / OnRollbackFiresOnFailureOnly / RemainingBodyStepsSkipOnFailure / PlanModeIsSafe) all still green. Full `mooncake task ci` green.
related: F053 (cold-read pass that surfaced this), spec-30 phase 5 (Reverse semantics; shipped), spec-30 phase 7 (run output formatting; partial — UX log line ↺ landed, machine-readable events did not), spec-68 §"queries that should work" line 123 ("show me every reverse step ever executed against this resource" — now answerable via runlog.StepEntry.Reverted).
---

## What

Three layers of the rollback-visibility design landed during spec-30 / spec-22 but the event-emit + runlog-tagging layer never did. The functional rollback was correct — transactions did unwind on failure, the inverse step did run — but the operator's only evidence was the one-line `↺ Reverse: <name>` log statement in `transaction.go:106`.

Three downstream consumers were broken by the gap:

1. **`mooncake history`** — `runlog.Entry.Steps[i].Result` was `"changed"` for both committed and rolled-back body steps. Operators reading the log couldn't tell which rows survived and which were undone.
2. **`apply.KernelResult` (the typed run result)** — `RunCapture.Steps()` returned the original body steps with no rollback annotation. Programmatic consumers (fleet aggregator, MCP `mooncake explain`, agent telemetry) couldn't distinguish.
3. **Fleet telemetry / log scrapers** — no JSON event to subscribe to. The "↺ Reverse:" log line was the only signal, and it's a free-form Infof call easily lost in noise.

Spec-30 §"Key files" line 158 had explicitly committed to six event types in `internal/runlog/runlog.go`:

> `transaction_begin`, `transaction_commit`, `transaction_rollback_begin`, `transaction_step_reversed`, `transaction_rollback_complete`, `transaction_rollback_failed`.

`grep -rn "TransactionBegin\|transaction_begin\|transaction_step_reversed" internal/` before this fix returned **zero hits**. Spec-68 §"queries that should work" (lines 123-124) named "show me every reverse step ever executed against this resource" as a should-answer query — unanswerable today because the underlying data wasn't recorded.

## Fix

**Layer 1 — Four new event types** (`internal/events/event.go`):

```go
const (
    EventTransactionRollbackBegin    EventType = "transaction.rollback_begin"
    EventTransactionStepReversed     EventType = "transaction.step_reversed"
    EventTransactionRollbackComplete EventType = "transaction.rollback_complete"
    EventTransactionRollbackFailed   EventType = "transaction.rollback_failed"
)
```

Each has a dedicated `*Data` struct carrying the txn parent ID, the failed / reversed step's identity, and the cumulative `ReversedSteps` count.

**Layer 2 — Emit at boundaries** (`internal/executor/transaction.go`):

- `EventTransactionRollbackBegin` at top of `handleTxnBodyFailure` (data: failed step ID/name, error message, `CompletedSteps` count).
- `EventTransactionStepReversed` after each successful `runReverse` (data: original step ID/name/action, per-reverse duration).
- `EventTransactionRollbackComplete` on clean rollback (no `firstErr`).
- `EventTransactionRollbackFailed` when any Reverse erred (data: failed inverse step ID/name + error, `ReversedSteps` count of inverses that ran before the failure).

The two terminal events are mutually exclusive — consumers can subscribe to one without disambiguating from the begin event.

**Layer 3 — Per-step Reverted flag** (`internal/executor/capture.go` + `internal/runlog/runlog.go` + `internal/apply/runlog_write.go`):

- `executor.StepRecord` gains `Reverted bool`.
- `RunCapture.markStepReverted(stepID)` flips the flag on the latest matching record (nil-safe; unknown ID is a no-op).
- `transaction.go` calls `ec.Svc.Capture.markStepReverted(entry.Step.ID)` after emitting `StepReversed`.
- `runlog.StepEntry.Reverted bool` (new field, `omitempty`).
- `apply.buildStepEntries` propagates `sr.Reverted` → `entry.Reverted` in the runlog projection.

The original step's record stays in `RunCapture.steps[]` — the action did run; rollback is decoration on top. `mooncake history`'s typed StepEntry now shows both facts: `Result: "changed", Reverted: true` ("this step ran, then was rolled back"). They're orthogonal to `Reversible` (handler-declared at registration time) vs `Reverted` (actually undone on this run).

## What's deferred

**`TransactionBegin` / `TransactionCommit` events** are NOT implemented. Spec-30 promised six events; this PR shipped four.

The two missing events need a per-compound-parent `step.started` / `step.completed` event surface that the executor doesn't expose today. The planner expands a `transaction:` block into a parent + body/rollback children; only the children get dispatched. The compound parent (which would be the natural emission point for begin / commit) has no `dispatchRunner` call to wrap. Synthesizing those events would require either:

- A new compound-parent lifecycle hook in the dispatch loop (cleaner; non-trivial), or
- Lazy emission at the first body child's start + last body child's success (heuristic; fragile to plan reshape).

Both are out of scope for F054. Tracked as a followup in `docs-working/code-review/TODO.md` under "Cross-cutting themes" — the deeper question is whether compound parents (transaction, try) should have their own lifecycle events independent of their children. Until that's answered, the four rollback events are the high-leverage subset (they're what consumers actually need to answer "what was undone?").

## Why "smell" not "risk"

The pre-fix state was a **visibility gap**, not a correctness bug. Rollback worked correctly — the inverse Steps dispatched, `Stats.Reverted` counted them, the `↺` log line surfaced to the operator. What was missing was machine-readable downstream. No data was wrong; data was just absent. Severity ladder:

- **bug** if `mooncake history` showed `Result: "changed"` for rolled-back steps (it didn't surface them at all in the typed StepEntry — Reverted field didn't exist).
- **smell** because the absence was internally consistent and didn't mislead — the field just wasn't there. `mooncake history` readers couldn't query rollback, but they didn't get a *wrong* answer either.

The fix promotes this from "internally consistent gap" to "complete spec-30 contract" — closes the absence rather than fixing a misdirection.
