package executor

// trycatch.go implements the spec-23 §2 try / catch / finally
// execution state on top of the planner's TryParent / TryRole tagging.
// The planner expanded a Try compound Step into a parent + sibling
// children with TryRole = "try" | "catch" | "finally"; this file owns
// the executor-side glue that makes those siblings behave correctly:
//
//   - A try child's failure marks the try-block failed and records
//     which child failed + with what error (catch can inspect later).
//   - Subsequent try children of the same TryParent skip with reason
//     "try-block already failed".
//   - Catch children skip when the try-block completed without error.
//   - Finally children always run (never gated by TryState).
//
// The compound Step's overall outcome is failure if try failed —
// catch is for notification / cleanup, NOT for swallowing the error.
// This mirrors spec-30 transactions: on_rollback fires after a failed
// transaction but the original failure still propagates. The "exit
// code non-zero" acceptance criterion in spec-23 confirms this shape.
//
// Post-R0.1: the pure-logic state machine (TryState struct, skip-
// reason determination, failure recording) lives in internal/control.
// This file is the executor-side glue — thin wrapper methods that
// own the OpenTries map lookup and call into control.

import (
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/control"
)

// tryStateFor returns (or lazily creates) the TryState for a given
// parent ID. All try-block bookkeeping flows through this so the map
// is never touched directly outside this file.
func (ec *ExecutionContext) tryStateFor(parentID string) *control.TryState {
	if ec.OpenTries == nil {
		ec.OpenTries = make(map[string]*control.TryState)
	}
	t, ok := ec.OpenTries[parentID]
	if !ok {
		t = &control.TryState{}
		ec.OpenTries[parentID] = t
	}
	return t
}

// trySkipReason returns a non-empty string when step (a try-block
// child) should skip this run. Thin wrapper over
// control.TrySkipReason that handles the executor-side OpenTries
// lookup.
func (ec *ExecutionContext) trySkipReason(step config.Step) string {
	if step.TryParent == "" {
		return ""
	}
	return control.TrySkipReason(ec.OpenTries[step.TryParent], step.TryRole)
}

// recordTryBodyFailure is called when a try child errors. Marks the
// try-block failed (lazily creating its TryState) and stashes the
// first failure's step ID and error so catch can inspect them.
func (ec *ExecutionContext) recordTryBodyFailure(step config.Step, err error) {
	if step.TryParent == "" || step.TryRole != "try" {
		return
	}
	control.RecordTryBodyFailure(ec.tryStateFor(step.TryParent), step.ID, err)
}

// recordTryCatchFailure is called when a catch child errors. The
// compound Step then propagates the catch error in preference to the
// original try error, per spec-23 §2.
func (ec *ExecutionContext) recordTryCatchFailure(step config.Step) {
	if step.TryParent == "" || step.TryRole != "catch" {
		return
	}
	control.RecordTryCatchFailure(ec.tryStateFor(step.TryParent))
}
