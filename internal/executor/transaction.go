package executor

// transaction.go implements the spec-30 §80–90 transaction-execution
// state machine on top of the planner's TxnParent / TxnRole tagging
// (PR A). The planner expanded a `transaction:` Step into a parent +
// sibling children with TxnRole="body" or TxnRole="rollback"; this file
// owns the bookkeeping that turns those siblings into all-or-nothing
// semantics:
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

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// txn returns (or lazily creates) the TxnState for a given parent ID.
// All transaction bookkeeping flows through this so the map is never
// touched directly outside transaction.go.
func (ec *ExecutionContext) txn(parentID string) *TxnState {
	if ec.OpenTxns == nil {
		ec.OpenTxns = make(map[string]*TxnState)
	}
	t, ok := ec.OpenTxns[parentID]
	if !ok {
		t = &TxnState{}
		ec.OpenTxns[parentID] = t
	}
	return t
}

// txnSkipReason returns a non-empty string when step (a transaction
// child) should skip this run. The reason is suitable for
// emitStepSkipped's user-facing message.
//
// Body children skip when their transaction has already failed (later
// children of a rolled-back txn don't run). Rollback children skip
// when the transaction committed successfully (on_rollback only fires
// on rollback).
func (ec *ExecutionContext) txnSkipReason(step config.Step) string {
	if step.TxnParent == "" {
		return ""
	}
	t := ec.OpenTxns[step.TxnParent]
	if t == nil {
		// First child of this transaction; no skip.
		return ""
	}
	switch step.TxnRole {
	case "body":
		if t.Failed {
			return "transaction rolled back"
		}
	case "rollback":
		if !t.RolledBack {
			return "transaction committed (on_rollback skipped)"
		}
	}
	return ""
}

// recordTxnBodyCompletion is called after a body child runs to
// completion. It snapshots the step + its result so a later Reverse()
// has the data it needs. Called only for successful runs — failures
// route through handleTxnBodyFailure instead.
func (ec *ExecutionContext) recordTxnBodyCompletion(step config.Step, result *Result) {
	if step.TxnParent == "" || step.TxnRole != "body" {
		return
	}
	t := ec.txn(step.TxnParent)
	t.Completed = append(t.Completed, TxnCompletedChild{Step: step, Result: result})
}

// handleTxnBodyFailure is called when a body child errors out. It
// marks the transaction failed and walks Completed in reverse,
// calling Reverse() on each child + dispatching the returned inverse
// step. Returns the rollback's own outcome — non-nil error means
// at least one Reverse failed (partial rollback).
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

	// LIFO reverse walk. Stop at the first Reverse failure but keep
	// RolledBack=true so on_rollback still fires for visibility.
	var firstErr error
	for i := len(t.Completed) - 1; i >= 0; i-- {
		entry := t.Completed[i]
		if err := ec.runReverse(entry.Step, entry.Result); err != nil {
			t.PartialRollback = true
			if firstErr == nil {
				firstErr = err
			}
			// Don't continue past a failed Reverse — the system state
			// is now indeterminate and we'd be guessing about further
			// undos. Halt and let on_rollback surface the partial state.
			break
		}
	}
	t.RolledBack = true
	return firstErr
}

// runReverse runs Reverse() for one completed step and dispatches the
// returned inverse Step. Returns an error if the handler reports
// "irreversible" or if the inverse step's apply fails.
func (ec *ExecutionContext) runReverse(step config.Step, result *Result) error {
	actionType := step.ActionType
	if actionType == "" {
		actionType = step.DetermineActionType()
	}
	handler, ok := actions.Get(actionType)
	if !ok {
		return fmt.Errorf("reverse %s: unknown action %q", step.Name, actionType)
	}
	reverser, ok := handler.(actions.Reverser)
	if !ok {
		// Planner's reversibility check should have caught this, but
		// guard defensively — allow_irreversible can let irreversibles
		// into the plan.
		return fmt.Errorf("reverse %s: handler does not implement Reverser", step.Name)
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
