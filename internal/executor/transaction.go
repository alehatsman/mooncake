package executor

// transaction.go implements the spec-30 §80–90 transaction-execution
// state machine on top of the planner's TxnParent / TxnRole tagging
// (PR A). The planner expanded a `transaction:` Step into a parent +
// sibling children with TxnRole="body" or TxnRole="rollback"; this
// file owns the bookkeeping that turns those siblings into all-or-
// nothing semantics:
//
//   - Each body child's success captures its (step, result) for later
//     potential Reverse().
//   - A body child's failure marks the transaction failed, walks the
//     completed children in LIFO order calling Reverse() on each, and
//     trips RolledBack so subsequent rollback children fire.
//   - Subsequent body children of a failed transaction skip with reason
//     "transaction rolled back".
//   - Rollback (on_rollback) children skip when the transaction
//     committed, run when it rolled back.
//
// Post-R0.1: the pure-logic state machine (TxnState struct, skip-
// reason determination) lives in internal/control. This file is the
// executor-side glue — it owns the `*Result` snapshot of completed
// body children (which can't live in control without a circular
// import) and the handler-dispatch path for Reverse().

import (
	"errors"
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/control"
	"github.com/alehatsman/mooncake/internal/events"
)

// errHandlerIrreversible signals that a completed body step can't be
// reversed because its handler implements no Reverser (shell / cmd and
// other inherently-irreversible actions). The rollback loop treats this
// as a quiet skip — NOT a rollback failure — and keeps reversing the
// rest of the LIFO walk. Distinct from an error returned by Reverse()
// itself, which still halts the walk and trips a partial rollback.
var errHandlerIrreversible = errors.New("handler does not implement Reverser")

// txn returns (or lazily creates) the TxnState for a given parent ID.
// All transaction bookkeeping flows through this so the map is never
// touched directly outside transaction.go.
func (ec *ExecutionContext) txn(parentID string) *control.TxnState {
	if ec.OpenTxns == nil {
		ec.OpenTxns = make(map[string]*control.TxnState)
	}
	t, ok := ec.OpenTxns[parentID]
	if !ok {
		t = &control.TxnState{}
		ec.OpenTxns[parentID] = t
	}
	return t
}

// txnSkipReason returns a non-empty string when step (a transaction
// child) should skip this run. Thin wrapper over control.TxnSkipReason
// that handles the executor-side OpenTxns lookup.
func (ec *ExecutionContext) txnSkipReason(step config.Step) string {
	if step.TxnParent == "" {
		return ""
	}
	return control.TxnSkipReason(ec.OpenTxns[step.TxnParent], step.TxnRole)
}

// recordTxnBodyCompletion is called after a body child runs to
// completion. It snapshots the step + its result so a later Reverse()
// has the data it needs. Called only for successful runs — failures
// route through handleTxnBodyFailure instead.
//
// The snapshot lives in ec.CompletedByTxn rather than on TxnState
// because *Result can't cross into internal/control.
func (ec *ExecutionContext) recordTxnBodyCompletion(step config.Step, result *Result) {
	if step.TxnParent == "" || step.TxnRole != "body" {
		return
	}
	// Ensure the txn state exists so the parent ID is tracked.
	_ = ec.txn(step.TxnParent)
	if ec.CompletedByTxn == nil {
		ec.CompletedByTxn = make(map[string][]TxnCompletedChild)
	}
	ec.CompletedByTxn[step.TxnParent] = append(
		ec.CompletedByTxn[step.TxnParent],
		TxnCompletedChild{Step: step, Result: result},
	)
}

// handleTxnBodyFailure is called when a body child errors out. It
// marks the transaction failed and walks the completed body children
// in reverse, calling Reverse() on each and dispatching the returned
// inverse step. Returns the rollback's own outcome — non-nil error
// means at least one Reverse failed (partial rollback).
//
// The original failure-causing step is NOT reversed (its handler
// failed before producing meaningful state). Only previously-
// completed children get reversed.
func (ec *ExecutionContext) handleTxnBodyFailure(failedStep config.Step) error {
	if failedStep.TxnParent == "" || failedStep.TxnRole != "body" {
		return nil
	}
	t := ec.txn(failedStep.TxnParent)
	t.Failed = true

	completed := ec.CompletedByTxn[failedStep.TxnParent]

	// F054 / spec-30: emit the rollback_begin event so machine-
	// readable consumers (runlog, fleet telemetry, mooncake history)
	// see the boundary. Pre-F054 only the "↺ Reverse:" log line
	// surfaced this; explain/JSON consumers had no signal.
	failedErrMsg := ""
	if ec.CurrentResult != nil && ec.CurrentResult.Failed {
		// CurrentResult is the failing body child's result; its
		// stderr/stdout tail are already on the step.failed event,
		// but the high-level "what triggered rollback" message
		// belongs on rollback_begin so consumers don't have to
		// stitch two events together.
		failedErrMsg = ec.CurrentResult.Reason
	}
	ec.EmitEvent(events.EventTransactionRollbackBegin, events.TransactionRollbackBeginData{
		TxnParentID:    failedStep.TxnParent,
		FailedStepID:   failedStep.ID,
		FailedStepName: failedStep.Name,
		ErrorMessage:   failedErrMsg,
		CompletedSteps: len(completed),
	})

	// LIFO reverse walk. Stop at the first Reverse failure but keep
	// RolledBack=true so on_rollback still fires for visibility.
	var firstErr error
	var reversedCount, skippedCount int
	var failedReverseStepID, failedReverseStepName string
	for i := len(completed) - 1; i >= 0; i-- {
		entry := completed[i]
		// MT-45: log the rollback step visibly so the operator can see
		// what's being undone. The README documents `↺ Reverse:` lines.
		ec.Svc.Logger.Infof("↺ Reverse: %s", entry.Step.Name)
		reverseStart := time.Now()
		if err := ec.runReverse(entry.Step, entry.Result); err != nil {
			// #66: an inherently-irreversible handler (no Reverser) is a
			// quiet skip, not a rollback failure — nothing was ever going
			// to be undone, so the system state isn't indeterminate. Emit
			// a structured reverse.skipped event and keep walking the rest
			// of the completed steps.
			if errors.Is(err, errHandlerIrreversible) {
				ec.EmitEvent(events.EventTransactionStepReverseSkipped, events.TransactionStepReverseSkippedData{
					TxnParentID: failedStep.TxnParent,
					StepID:      entry.Step.ID,
					Name:        entry.Step.Name,
					Action:      entry.Step.DetermineActionType(),
					Reason:      "irreversible",
				})
				skippedCount++
				continue
			}
			t.PartialRollback = true
			failedReverseStepID = entry.Step.ID
			failedReverseStepName = entry.Step.Name
			if firstErr == nil {
				firstErr = err
			}
			// Don't continue past a failed Reverse — the system state
			// is now indeterminate and we'd be guessing about further
			// undos. Halt and let on_rollback surface the partial state.
			break
		}
		// F054: per-step reversed event. Identifies the ORIGINAL step
		// (the one whose effect just got undone), not the inverse —
		// that's what `mooncake history` readers care about.
		ec.EmitEvent(events.EventTransactionStepReversed, events.TransactionStepReversedData{
			TxnParentID: failedStep.TxnParent,
			StepID:      entry.Step.ID,
			Name:        entry.Step.Name,
			Action:      entry.Step.DetermineActionType(),
			DurationMs:  time.Since(reverseStart).Milliseconds(),
		})
		// F054: mark the original step's RunCapture record as
		// Reverted so the runlog StepEntry projection lights up
		// the Reverted flag. Lookup uses step.ID; nil-capture
		// callers (legacy ExecutePlan without RunCapture) no-op.
		ec.Svc.Capture.markStepReverted(entry.Step.ID)
		reversedCount++
		// MT-45: a successful Reverse cancels out the original body
		// step's reported change. Subtract from the run-wide Changed
		// counter and bump Reverted so the recap reflects net effect
		// (rolled-back files no longer count as user-visible writes).
		// decStat / incStat are nil-safe (see context.go) so the
		// outer Stats != nil check is the only guard needed.
		if entry.Result != nil && entry.Result.Changed && ec.Svc.Stats != nil {
			decStat(ec.Svc.Stats.Changed)
			// F6: keep OK+Changed==Executed across rollback. The step
			// is still counted in Executed (rollback doesn't unrun
			// it); moving it from the Changed bucket to OK preserves
			// the invariant. Reverted is bumped in parallel as the
			// rollback-specific subcategory — it overlaps OK the way
			// the existing recap line ("ok=N reverted=M") already
			// reads, just no longer via subtraction.
			incStat(ec.Svc.Stats.OK)
			incStat(ec.Svc.Stats.Reverted)
		}
	}
	t.RolledBack = true

	// F054: terminal event — Complete on clean rollback, Failed when
	// any Reverse erred (partial rollback). The two events are
	// mutually exclusive; consumers can subscribe to whichever they
	// care about without needing to disambiguate from the
	// rollback_begin alone.
	if firstErr == nil {
		ec.EmitEvent(events.EventTransactionRollbackComplete, events.TransactionRollbackCompleteData{
			TxnParentID:   failedStep.TxnParent,
			ReversedSteps: reversedCount,
			SkippedSteps:  skippedCount,
		})
	} else {
		ec.EmitEvent(events.EventTransactionRollbackFailed, events.TransactionRollbackFailedData{
			TxnParentID:           failedStep.TxnParent,
			ReversedSteps:         reversedCount,
			FailedReverseStepID:   failedReverseStepID,
			FailedReverseStepName: failedReverseStepName,
			ErrorMessage:          firstErr.Error(),
		})
	}
	return firstErr
}

// runReverse runs Reverse() for one completed step and dispatches the
// returned inverse Step. Returns an error if the handler reports
// "irreversible" or if the inverse step's apply fails. Stays in
// executor because it needs the actions registry + dispatchRunner.
func (ec *ExecutionContext) runReverse(step config.Step, result *Result) error {
	actionType := step.ActionType
	if actionType == "" {
		actionType = step.DetermineActionType()
	}
	handler, ok := ec.ActionRegistry().Get(actionType)
	if !ok {
		return fmt.Errorf("reverse %s: unknown action %q", step.Name, actionType)
	}
	reverser, ok := handler.(actions.Reverser)
	if !ok {
		// Inherently-irreversible handler (shell / cmd, …). This is not a
		// rollback failure — there was never anything to undo — so signal
		// it distinctly. The caller emits a reverse.skipped event and
		// continues the LIFO walk instead of halting with a prose error
		// (#66). allow_irreversible can legitimately let these into a plan.
		return fmt.Errorf("reverse %s: %w", step.Name, errHandlerIrreversible)
	}

	inverse, err := reverser.Reverse(ec, &step, result)
	if err != nil {
		return fmt.Errorf("reverse %s: %w", step.Name, err)
	}
	if inverse == nil {
		// Handler said "no reverse needed" (e.g. step was a noop).
		return nil
	}

	// Dispatch the inverse step through the normal handler path. The
	// inverse step doesn't itself participate in transaction
	// bookkeeping — it carries no TxnParent / TxnRole — so the regular
	// dispatch path runs it like a one-off.
	inverse.ID = "" // let downstream code mint a fresh ID
	inverse.TxnParent = ""
	inverse.TxnRole = ""

	// Build a runner from the handler — same dispatch shape as a
	// normal step. Refuses to wrap if handler doesn't implement Runner
	// (every Spec-16 action does, but defensive).
	runner, ok := handler.(actions.Runner)
	if !ok {
		return fmt.Errorf("reverse %s: handler is not a Runner", step.Name)
	}
	if dispatchErr := dispatchRunner(*inverse, ec, runner); dispatchErr != nil {
		return fmt.Errorf("reverse %s: applying inverse: %w", step.Name, dispatchErr)
	}
	return nil
}
