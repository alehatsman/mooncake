package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestHandleStepError_CancelledStepNotDoubleCounted pins the F058 fix.
//
// When the run-wide ctx is cancelled while a handler is in flight,
// dispatchRunner's syncResultEnvelope tags the Result Cancelled and
// bumps Stats.Cancelled. handleStepError must NOT then also bump
// Stats.Failed for the same step — the double-count broke the
// proposal-02 exit-code contract: mapCancelExit gates exit 130 on
// `Cancelled > 0 && Failed == 0`, so a step counted in both buckets
// made timeout/fleet/MCP cancels exit 1 (indistinguishable from a real
// failure).
//
// This drives the real two-function integration (dispatchRunner →
// handleStepError); the sync_envelope_test.go suite exercises
// syncResultEnvelope in isolation and never reaches the run-counter
// re-bump, so it does not catch the regression.
func TestHandleStepError_CancelledStepNotDoubleCounted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // run-wide ctx already cancelled when the handler errors

	runner := &fakeRawRunner{
		result: NewResult(), // non-nil *Result so syncResultEnvelope runs
		err:    errors.New("handler errored during teardown"),
	}
	step := config.Step{}

	ec := dispatchTestContext()
	ec.Svc.Ctx = ctx
	ec.Svc.Stats = NewExecutionStats()

	dispatchErr := dispatchRunner(step, ec, runner)
	if dispatchErr == nil {
		t.Fatal("dispatchRunner returned nil; expected the handler err to propagate")
	}
	if ec.CurrentResult == nil || !ec.CurrentResult.Cancelled {
		t.Fatal("syncResultEnvelope did not tag the Result Cancelled; test premise broken")
	}
	if got := readStat(ec.Svc.Stats.Cancelled); got != 1 {
		t.Fatalf("Stats.Cancelled = %d after dispatchRunner, want 1", got)
	}

	// Fix under test: a cancelled step must not be re-counted as Failed.
	_ = handleStepError(step, ec, dispatchErr, "step-1", "cancel-me", 0, 0)

	if got := readStat(ec.Svc.Stats.Failed); got != 0 {
		t.Errorf("Stats.Failed = %d, want 0 (cancelled step must not double-count as failed)", got)
	}
	if got := readStat(ec.Svc.Stats.Cancelled); got != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1 (unchanged by handleStepError)", got)
	}
}

// TestHandleStepError_PlainFailureStillCounted is the control: a
// genuine handler failure with a live (non-cancelled) ctx must still
// land on Stats.Failed and leave Stats.Cancelled at zero. Guards
// against the F058 fix over-reaching and swallowing real failures.
func TestHandleStepError_PlainFailureStillCounted(t *testing.T) {
	runner := &fakeRawRunner{
		result: NewResult(),
		err:    errors.New("genuine handler failure"),
	}
	step := config.Step{}

	ec := dispatchTestContext()
	ec.Svc.Ctx = context.Background() // live ctx — not a cancel
	ec.Svc.Stats = NewExecutionStats()

	dispatchErr := dispatchRunner(step, ec, runner)
	if dispatchErr == nil {
		t.Fatal("dispatchRunner returned nil; expected the handler err to propagate")
	}
	if ec.CurrentResult != nil && ec.CurrentResult.Cancelled {
		t.Fatal("Result tagged Cancelled with a live ctx; test premise broken")
	}

	_ = handleStepError(step, ec, dispatchErr, "step-1", "fail-me", 0, 0)

	if got := readStat(ec.Svc.Stats.Failed); got != 1 {
		t.Errorf("Stats.Failed = %d, want 1 (real failure must count)", got)
	}
	if got := readStat(ec.Svc.Stats.Cancelled); got != 0 {
		t.Errorf("Stats.Cancelled = %d, want 0 (no cancel occurred)", got)
	}
}
