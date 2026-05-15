package control

// TryState is the kernel's per-try-block state machine. Owned by the
// executor's OpenTries map; populated as try / catch / finally
// children run; consulted by TrySkipReason to gate siblings.
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
	// failure. Per spec-23 §2 the compound Step propagates the catch
	// error in preference to the original try error when this is true.
	CatchFailed bool

	// ContinueOnError mirrors the compound Step's continue_on_error
	// flag. When the try-block resolves with failure (try child
	// errored, catch didn't recover, or catch itself errored), the
	// executor checks this to decide whether to propagate the error
	// or swallow it. Issue #23.
	ContinueOnError bool
}

// TrySkipReason returns a non-empty string when a try-block child
// should skip this run. The reason is suitable for the user-facing
// step-skipped message.
//
// state may be nil — for the first child of a try-block, no TryState
// has been allocated. In that case, only "catch" skips (catch
// requires a recorded try failure to run); "try" and "finally" do not.
//
// role is the child's TryRole ("try", "catch", or "finally"). Try
// children skip after a sibling try has failed. Catch children skip
// when no try failure was recorded. Finally children never skip on
// this basis — they always run.
func TrySkipReason(state *TryState, role string) string {
	if state == nil {
		if role == "catch" {
			// No state means no try-child ever failed (either try ran
			// clean or every try child skipped) — either way, catch
			// must not run.
			return "try-block succeeded (catch skipped)"
		}
		return ""
	}
	switch role {
	case "try":
		if state.Failed {
			return "try-block already failed"
		}
	case "catch":
		if !state.Failed {
			return "try-block succeeded (catch skipped)"
		}
	}
	// "finally" never skips on this basis — always run.
	return ""
}

// RecordTryBodyFailure marks a try-block as failed and captures the
// first failure's step ID and error so catch can inspect them.
//
// Idempotent on the failure capture: if Failed is already set, the
// original information is preserved (subsequent try children won't
// reach this path anyway — they skip — but defensive).
func RecordTryBodyFailure(state *TryState, stepID string, err error) {
	if state == nil || state.Failed {
		return
	}
	state.Failed = true
	state.FailedStepID = stepID
	if err != nil {
		state.FailedError = err.Error()
	}
}

// RecordTryCatchFailure marks a catch child as having errored. The
// compound Step then propagates the catch error in preference to the
// original try error, per spec-23 §2.
func RecordTryCatchFailure(state *TryState) {
	if state == nil {
		return
	}
	state.CatchFailed = true
}
