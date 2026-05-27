package executor

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestSyncResultEnvelope_NilErr_NoOp pins the contract that the
// helper is a no-op when the handler returned cleanly — the success
// path must not touch any envelope field.
func TestSyncResultEnvelope_NilErr_NoOp(t *testing.T) {
	r := &Result{Changed: true, Operation: OpUpdate}
	syncResultEnvelope(context.Background(), r, nil, nil)
	if r.Failed || r.Cancelled || r.Error != "" || r.Rc != 0 {
		t.Errorf("nil-err: envelope mutated: %+v", r)
	}
}

// TestSyncResultEnvelope_PlainErr_Failed is the proposal-06 case:
// handler returned err, runCtx is healthy, so this is a handler-level
// failure that should land on Failed + Error + Rc.
func TestSyncResultEnvelope_PlainErr_Failed(t *testing.T) {
	r := &Result{}
	syncResultEnvelope(context.Background(), r, errors.New("disk full"), nil)
	if !r.Failed {
		t.Errorf("Failed should be true")
	}
	if r.Cancelled {
		t.Errorf("Cancelled must stay false — runCtx wasn't cancelled")
	}
	if r.Error != "disk full" {
		t.Errorf("Error = %q, want %q", r.Error, "disk full")
	}
	if r.Rc != 1 {
		t.Errorf("Rc = %d, want 1", r.Rc)
	}
}

// TestSyncResultEnvelope_RunCtxCanceled_Sigint covers the proposal-02
// attribution: when the run-wide ctx is cancelled at the moment the
// handler errors, classify as Cancelled rather than Failed and bump
// Stats.Cancelled. SIGINT-equivalent context.Canceled maps to
// CancelledReason=sigint.
func TestSyncResultEnvelope_RunCtxCanceled_Sigint(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	stats := NewExecutionStats()
	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("aborted mid-write"), stats)

	if r.Failed {
		t.Errorf("Failed must stay false — this is a cancel, not a failure")
	}
	if !r.Cancelled {
		t.Errorf("Cancelled should be true when runCtx.Err() != nil")
	}
	if r.CancelledReason != CancelledReasonSigint {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonSigint)
	}
	if r.Error != "aborted mid-write" {
		t.Errorf("Error = %q, want %q", r.Error, "aborted mid-write")
	}
	if *stats.Cancelled != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1", *stats.Cancelled)
	}
}

// TestSyncResultEnvelope_RunCtxDeadline_Timeout pins the second
// CancelledReason classification path: DeadlineExceeded → "timeout".
func TestSyncResultEnvelope_RunCtxDeadline_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()
	// Spin briefly so the deadline fires deterministically.
	for ctx.Err() == nil {
		time.Sleep(1 * time.Millisecond)
	}

	stats := NewExecutionStats()
	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("deadline exceeded"), stats)

	if !r.Cancelled {
		t.Errorf("Cancelled should be true")
	}
	if r.CancelledReason != CancelledReasonTimeout {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonTimeout)
	}
	if *stats.Cancelled != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1", *stats.Cancelled)
	}
}

// TestSyncResultEnvelope_NilStats_NoBump guards the nil-stats caller
// shape (dispatch contexts that don't track counters). The envelope
// must still be classified; only the Stats bump is skipped.
func TestSyncResultEnvelope_NilStats_NoBump(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("boom"), nil)
	if !r.Cancelled {
		t.Errorf("Cancelled should be true even when stats is nil")
	}
}

// TestSyncResultEnvelope_HandlerPreSetFailed_Preserved guards the
// "handler already populated Failed/Error" path — the helper must
// not clobber a handler-set diagnostic. Handlers that DO set Failed
// + Error explicitly (assert, shell with rc!=0) are the canonical
// path; the helper only fills the gap for the spec-69 B0 cluster.
func TestSyncResultEnvelope_HandlerPreSetFailed_Preserved(t *testing.T) {
	r := &Result{Failed: true, Error: "assertion failed: x != y", Rc: 1}
	syncResultEnvelope(context.Background(), r, errors.New("assertion failed: x != y"), nil)
	if r.Error != "assertion failed: x != y" {
		t.Errorf("handler-set Error was clobbered: %q", r.Error)
	}
	if r.Rc != 1 {
		t.Errorf("Rc = %d, want 1 (handler-set)", r.Rc)
	}
}
