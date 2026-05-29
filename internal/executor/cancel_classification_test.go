package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestHandleStepError_CancelledStepNotDoubleCounted pins the F058 fix.
//
// When the run-wide ctx is cancelled while a handler is in flight, the
// step must land on Stats.Cancelled only — never on both Cancelled and
// Failed. The double-count broke the proposal-02 exit-code contract:
// mapCancelExit gates exit 130 on `Cancelled > 0 && Failed == 0`, so a
// step counted in both buckets made timeout/fleet/MCP cancels exit 1
// (indistinguishable from a real failure).
//
// F058 choice-2: the single run-counter classification site is
// handleStepError, keyed off ec.Svc.Ctx.Err(). syncResultEnvelope still
// tags the *Result envelope Cancelled (for handlers that return a
// *Result) but no longer touches any counter — so after dispatchRunner
// the counters are untouched and the bump appears only after
// handleStepError runs.
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
	// F058: syncResultEnvelope no longer bumps the counter — the bump is
	// handleStepError's job now.
	if got := readStat(ec.Svc.Stats.Cancelled); got != 0 {
		t.Fatalf("Stats.Cancelled = %d after dispatchRunner, want 0 (bump moved to handleStepError)", got)
	}

	// Fix under test: a cancelled step counts as Cancelled, not Failed.
	_ = handleStepError(step, ec, dispatchErr, "step-1", "cancel-me", 0, 0)

	if got := readStat(ec.Svc.Stats.Failed); got != 0 {
		t.Errorf("Stats.Failed = %d, want 0 (cancelled step must not count as failed)", got)
	}
	if got := readStat(ec.Svc.Stats.Cancelled); got != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1 (handleStepError is the single bump site)", got)
	}
}

// TestHandleStepError_NilResultHandler_CancelClassified is the
// F058 choice-2 regression: handlers that return (nil, err) — pkg
// install of a missing package, most spec-69 RawRunners on error —
// never reach syncResultEnvelope, so dispatchRunner leaves
// ec.CurrentResult as a fresh NewResult() (Cancelled=false). The
// pre-choice-2 fix keyed off ec.CurrentResult.Cancelled, so these
// handlers stayed on the Failed path and exited 1 on a mid-flight
// cancel. Keying off ec.Svc.Ctx.Err() classifies them as Cancelled
// regardless of return shape.
func TestHandleStepError_NilResultHandler_CancelClassified(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runner := &fakeRawRunner{
		result: nil, // typed-nil *Result → syncResultEnvelope is skipped
		err:    errors.New("missing package during teardown"),
	}
	step := config.Step{}

	ec := dispatchTestContext()
	ec.Svc.Ctx = ctx
	ec.Svc.Stats = NewExecutionStats()

	dispatchErr := dispatchRunner(step, ec, runner)
	if dispatchErr == nil {
		t.Fatal("dispatchRunner returned nil; expected the handler err to propagate")
	}
	// Premise: the (nil, err) shape leaves CurrentResult untagged — this
	// is exactly the gap choice-1 (keyed off the Result) could not close.
	if ec.CurrentResult == nil {
		t.Fatal("ec.CurrentResult is nil; dispatchRunner should install a fresh NewResult()")
	}
	if ec.CurrentResult.Cancelled {
		t.Fatal("Result tagged Cancelled by dispatchRunner for a (nil, err) handler; test premise broken")
	}

	_ = handleStepError(step, ec, dispatchErr, "step-1", "missing-pkg", 0, 0)

	if got := readStat(ec.Svc.Stats.Cancelled); got != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1 ((nil, err) handler must classify as cancelled on mid-flight cancel)", got)
	}
	if got := readStat(ec.Svc.Stats.Failed); got != 0 {
		t.Errorf("Stats.Failed = %d, want 0 (cancelled step must not count as failed)", got)
	}
	// The envelope is back-filled so runlog/streaming see a consistent
	// view even though the handler returned no Result.
	if !ec.CurrentResult.Cancelled {
		t.Errorf("ec.CurrentResult.Cancelled = false, want true (handleStepError back-fills the envelope)")
	}
	if ec.CurrentResult.CancelledReason != CancelledReasonCancelled {
		t.Errorf("CancelledReason = %q, want %q", ec.CurrentResult.CancelledReason, CancelledReasonCancelled)
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
