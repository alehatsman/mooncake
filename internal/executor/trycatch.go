package executor

// trycatch.go implements the spec-23 §2 try / catch / finally
// execution state on top of the planner's TryParent / TryRole tagging.
// The planner expanded a Try compound Step into a parent + sibling
// children with TryRole = "try" | "catch" | "finally"; this file owns
// the bookkeeping that makes those siblings behave correctly:
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

import "github.com/alehatsman/mooncake/internal/config"

// TryState is the per-try-block state the executor accumulates as it
// walks try / catch / finally children of one compound parent.
type TryState struct {
	// Failed is true once any try child of this block has errored.
	// Gates subsequent try children (skip) and catch children (run).
	Failed bool

	// FailedStepID is the plan-step ID of the first try child that
	// errored. Surfaced to catch via outputs so a notify step can
	// reference which step failed.
	FailedStepID string

	// FailedError is the error message from the first try failure.
	// Surfaced to catch via outputs.
	FailedError string

	// CatchFailed is true if any catch child errored after the try
	// failure. Per spec-23 §2: "If a catch step itself errors, the
	// compound Step propagates the later error" — so when this is
	// true the executor reports the catch error rather than the
	// original try error.
	CatchFailed bool

	// ContinueOnError mirrors the compound Step's continue_on_error
	// flag. When the try-block resolves with failure (try child
	// errored, catch ran but didn't recover, or catch itself
	// errored), the executor checks this to decide whether to
	// propagate the error or swallow it and keep the outer run going.
	// Issue #23: without this, `continue_on_error: true` on a `try:`
	// compound was silently discarded — operators reasonably expect
	// the universal Step.ContinueOnError field to compose with
	// compound shapes too, not just leaf actions.
	ContinueOnError bool
}

// tryStateFor returns (or lazily creates) the TryState for a given
// parent ID. All try-block bookkeeping flows through this so the map
// is never touched directly outside this file.
func (ec *ExecutionContext) tryStateFor(parentID string) *TryState {
	if ec.OpenTries == nil {
		ec.OpenTries = make(map[string]*TryState)
	}
	t, ok := ec.OpenTries[parentID]
	if !ok {
		t = &TryState{}
		ec.OpenTries[parentID] = t
	}
	return t
}

// trySkipReason returns a non-empty string when step (a try-block
// child) should skip this run. The reason is suitable for
// emitStepSkipped's user-facing message.
//
// Try children skip after a sibling try child of the same TryParent
// has already failed. Catch children skip when the try-block ran to
// completion without error. Finally children never skip on this
// basis — they always run.
func (ec *ExecutionContext) trySkipReason(step config.Step) string {
	if step.TryParent == "" {
		return ""
	}
	t := ec.OpenTries[step.TryParent]
	switch step.TryRole {
	case "try":
		// First try child of this block — TryState may still be nil
		// (no prior child recorded a failure). No skip on this basis.
		if t != nil && t.Failed {
			return "try-block already failed"
		}
	case "catch":
		// Catch runs ONLY when try recorded a failure. A nil TryState
		// means no try child ever errored (either try ran clean or
		// every try child skipped) — either way, catch must not run.
		if t == nil || !t.Failed {
			return "try-block succeeded (catch skipped)"
		}
	}
	// "finally" never skips on this basis — always run.
	return ""
}

// recordTryBodyFailure is called when a try child errors. It marks
// the try-block failed and stashes the first failure's step ID and
// error so catch can inspect them.
//
// Idempotent on the failure capture: if Failed is already set (e.g.
// the executor re-enters this path), the original failure information
// is preserved.
func (ec *ExecutionContext) recordTryBodyFailure(step config.Step, err error) {
	if step.TryParent == "" || step.TryRole != "try" {
		return
	}
	t := ec.tryStateFor(step.TryParent)
	if t.Failed {
		return
	}
	t.Failed = true
	t.FailedStepID = step.ID
	if err != nil {
		t.FailedError = err.Error()
	}
}

// recordTryCatchFailure is called when a catch child errors. The
// compound Step then propagates the catch error in preference to the
// original try error, per spec-23 §2.
func (ec *ExecutionContext) recordTryCatchFailure(step config.Step) {
	if step.TryParent == "" || step.TryRole != "catch" {
		return
	}
	t := ec.tryStateFor(step.TryParent)
	t.CatchFailed = true
}
