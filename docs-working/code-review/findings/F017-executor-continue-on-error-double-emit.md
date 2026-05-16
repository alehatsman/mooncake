---
id: F017
title: executor: continue_on_error emits BOTH step.failed and step.completed for the same step
severity: bug
package: internal/executor
file: internal/executor/executor.go
lines: 477-523, 561-600, 736-749
status: open
---

## What

When a step fails and the step has `continue_on_error: true`,
the executor calls `handleStepError` followed by
`postExecuteSuccess` — emitting two terminal events for the
same step:

Flow in `ExecuteStep` (lines 736-749):

```go
stepErr := DispatchStepAction(step, ec)
// ...
if stepErr != nil {
    if err := handleStepError(...); err != nil {
        return err   // re-raised: stops the run / triggers parent error
    }
    // else fall through — continue_on_error swallowed the error
}
postExecuteSuccess(step, ec, stepID, stepName, depth, stepDuration)
```

`handleStepError` (line 477-523):

- Line 500: `ec.EmitEvent(events.EventStepFailed, failedData)` ← event #1
- Line 513-519: `if step.ContinueOnError { ...; return nil }` — swallows error

Then back in `ExecuteStep` we fall through to:

`postExecuteSuccess` (line 561-600):

- Line 576: `ec.EmitEvent(events.EventStepCompleted, ...)` ← event #2

A consumer of the run's event stream sees `step.failed` followed
by `step.completed` for the *same* `StepID`. Both reach
`events.jsonl`, both reach SSE subscribers, both feed the runlog.

## Why it's a bug

1. **Event consumers don't expect a step to have two terminal
   events.** SSE clients that draw a step as "running" until they
   see a terminal event will flip-state from failed→completed,
   which presents the step as "succeeded after all" — exactly the
   wrong UX. The fleet-side run summary (`internal/runlog`) may
   double-count.
2. **`Stats.Failed` AND `Stats.Executed` both increment** for the
   same step (line 479, line 562-564). Mooncake's printed summary
   "X executed, Y failed" therefore reports `executed > total
   steps` when any continue_on_error step fails. Subtle
   off-by-N.
3. **`step.completed`'s `Changed` field reads from
   `ec.CurrentResult`** which `handleStepError` set to a
   synthetic failed `Result` via `captureResult`. So the
   "completed" event reports the failure state — confusing
   payload.

## Why it's `bug` and not `smell`

Reproducible. Single failing test case:

```go
func TestContinueOnErrorEmitsSingleTerminalEvent(t *testing.T) {
    var events []string
    sink := func(ev events.Event) {
        events = append(events, ev.Type)
    }
    // Build a step with: action that returns error, ContinueOnError=true
    // Run via ExecuteStep
    // Assert exactly one of {EventStepFailed, EventStepCompleted} in events.
}
```

Today that test would fail; both events fire.

## Suggested fix

Two clean ways:

**Option A — return early from ExecuteStep when handleStepError
returned nil (swallowed):**

```go
if stepErr != nil {
    if err := handleStepError(...); err != nil {
        return err
    }
    return nil   // ← was missing; continue_on_error path stops here
}
postExecuteSuccess(...)
```

Smallest diff. `handleStepError` is now the sole terminal-event
emitter for the failed path. `postExecuteSuccess` stays
the sole terminal-event emitter for the success path. Clean
separation.

**Option B — `handleStepError` returns a sentinel so callers can
decide:**

Heavier signature change. Not worth it for a one-line fix.

**Option A is the right move.** Stats accounting in
`handleStepError` should already increment Failed; the missing
`Stats.Executed++` for the failed path is a separate question
(today: continue_on_error increments Executed in
`postExecuteSuccess`; the right semantic is probably
"every dispatched step counts as Executed, including failures"
— add the increment in `handleStepError` itself if so).

## Adjacent observation

`ExecuteStep`'s `//nolint:gocyclo` (line 604) appears to be stale.
`make budget-status` (today) lists only `explain.DisplayFacts`
as over the gocyclo cap; ExecuteStep is no longer flagged after
the arch-wins extractions (`postExecuteSuccess` and
`dispatchPlanMode`). The nolint can probably go. Quick check:

```sh
gocyclo -over 0 internal/executor/executor.go | sort -rn | head -5
```

If `ExecuteStep` shows up < 35, delete the directive.

## Verification

- Write the test above; confirm one terminal event.
- `go test ./internal/executor/...` after the fix.
- Manual: `mooncake apply` with a `continue_on_error: true` step
  that fails; check `events.jsonl` — should contain exactly one
  terminal event per step (whichever is appropriate).

## References

- Stats counters live on `ec.Svc.Stats` — see
  `internal/executor/types.go` for the struct.
- `handleStepError` line 513-519 is where the swallow happens.
- The Spec-23 try/catch path also flows through here; verify
  the fix doesn't break try-block error capture (catch should
  fire on failure regardless of continue_on_error).
