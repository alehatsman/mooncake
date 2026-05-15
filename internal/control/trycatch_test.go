package control

import (
	"errors"
	"testing"
)

// TestTrySkipReason_NilState_CatchSkips — when no try child has
// failed yet (nil state), catch must skip with the spec-23 §2 message.
// Try and finally do not skip on this basis.
func TestTrySkipReason_NilState_CatchSkips(t *testing.T) {
	if got := TrySkipReason(nil, "catch"); got != "try-block succeeded (catch skipped)" {
		t.Errorf("nil-state catch: got %q, want catch-skipped message", got)
	}
	if got := TrySkipReason(nil, "try"); got != "" {
		t.Errorf("nil-state try should not skip; got %q", got)
	}
	if got := TrySkipReason(nil, "finally"); got != "" {
		t.Errorf("nil-state finally should not skip; got %q", got)
	}
}

// TestTrySkipReason_TrySkipsAfterFailure — once a sibling try has
// failed, subsequent try children skip.
func TestTrySkipReason_TrySkipsAfterFailure(t *testing.T) {
	state := &TryState{Failed: true}
	if got := TrySkipReason(state, "try"); got != "try-block already failed" {
		t.Errorf("got %q, want try-already-failed message", got)
	}
}

// TestTrySkipReason_CatchRunsAfterFailure — catch runs only when a
// try child failed.
func TestTrySkipReason_CatchRunsAfterFailure(t *testing.T) {
	failed := &TryState{Failed: true}
	if got := TrySkipReason(failed, "catch"); got != "" {
		t.Errorf("catch after failure should run (no skip); got %q", got)
	}
	cleanState := &TryState{Failed: false}
	if got := TrySkipReason(cleanState, "catch"); got != "try-block succeeded (catch skipped)" {
		t.Errorf("catch with no failure: got %q, want catch-skipped message", got)
	}
}

// TestTrySkipReason_FinallyNeverSkips — finally is unconditional.
func TestTrySkipReason_FinallyNeverSkips(t *testing.T) {
	for _, state := range []*TryState{
		nil,
		{},
		{Failed: true},
		{Failed: true, CatchFailed: true},
	} {
		if got := TrySkipReason(state, "finally"); got != "" {
			t.Errorf("finally should never skip; state=%+v got %q", state, got)
		}
	}
}

// TestRecordTryBodyFailure_CapturesFirst — idempotent on the failure
// capture: first failure wins; subsequent calls don't overwrite.
func TestRecordTryBodyFailure_CapturesFirst(t *testing.T) {
	state := &TryState{}
	first := errors.New("first failure")
	second := errors.New("second failure (should not overwrite)")

	RecordTryBodyFailure(state, "step-1", first)
	if !state.Failed {
		t.Fatal("expected Failed=true after first call")
	}
	if state.FailedStepID != "step-1" {
		t.Errorf("FailedStepID = %q, want step-1", state.FailedStepID)
	}
	if state.FailedError != first.Error() {
		t.Errorf("FailedError = %q, want %q", state.FailedError, first.Error())
	}

	RecordTryBodyFailure(state, "step-2", second)
	if state.FailedStepID != "step-1" {
		t.Errorf("idempotent: FailedStepID should remain step-1; got %q", state.FailedStepID)
	}
	if state.FailedError != first.Error() {
		t.Errorf("idempotent: FailedError should remain first; got %q", state.FailedError)
	}
}

// TestRecordTryBodyFailure_NilStateSafe — defensive: calling with
// nil state must not panic.
func TestRecordTryBodyFailure_NilStateSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("nil state should not panic; got %v", r)
		}
	}()
	RecordTryBodyFailure(nil, "step-1", errors.New("e"))
}

// TestRecordTryCatchFailure_FlipsCatchFailed — catch error during a
// failed-try resolution flips the CatchFailed signal.
func TestRecordTryCatchFailure_FlipsCatchFailed(t *testing.T) {
	state := &TryState{Failed: true}
	RecordTryCatchFailure(state)
	if !state.CatchFailed {
		t.Error("expected CatchFailed=true")
	}
}
