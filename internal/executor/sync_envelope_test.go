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
	syncResultEnvelope(context.Background(), r, nil)
	if r.Failed || r.Cancelled || r.Error != "" || r.Rc != 0 {
		t.Errorf("nil-err: envelope mutated: %+v", r)
	}
}

// TestSyncResultEnvelope_PlainErr_Failed is the proposal-06 case:
// handler returned err, runCtx is healthy, so this is a handler-level
// failure that should land on Failed + Error + Rc.
func TestSyncResultEnvelope_PlainErr_Failed(t *testing.T) {
	r := &Result{}
	syncResultEnvelope(context.Background(), r, errors.New("disk full"))
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

// TestSyncResultEnvelope_RunCtxCanceled_Generic covers the proposal-02
// attribution: when the run-wide ctx is cancelled at the moment the
// handler errors, tag the envelope Cancelled rather than Failed. A
// plain context.WithCancel with no cause attached maps to
// CancelledReason=cancelled (the F4 generic bucket — producer didn't
// attribute the cancel, so the envelope refuses to guess).
//
// F058: the run-counter bump moved to handleStepError; this helper now
// only tags the *Result envelope, so there is no Stats.Cancelled
// assertion here. See cancel_classification_test.go for the counter.
func TestSyncResultEnvelope_RunCtxCanceled_Generic(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("aborted mid-write"))

	if r.Failed {
		t.Errorf("Failed must stay false — this is a cancel, not a failure")
	}
	if !r.Cancelled {
		t.Errorf("Cancelled should be true when runCtx.Err() != nil")
	}
	if r.CancelledReason != CancelledReasonCancelled {
		t.Errorf("CancelledReason = %q, want %q (no cause attached → generic)", r.CancelledReason, CancelledReasonCancelled)
	}
	if r.Error != "aborted mid-write" {
		t.Errorf("Error = %q, want %q", r.Error, "aborted mid-write")
	}
}

// TestSyncResultEnvelope_CancelCause_Signal covers F4 attribution:
// WithCancelCause(ctx, ErrCancelSignal) → CancelledReason=sigint.
// The signal handlers in cmd/kernel/apply.go and
// internal/fleet/orchestrator.go are the live producers.
func TestSyncResultEnvelope_CancelCause_Signal(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrCancelSignal)

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("aborted by ^C"))

	if !r.Cancelled {
		t.Errorf("Cancelled should be true")
	}
	if r.CancelledReason != CancelledReasonSigint {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonSigint)
	}
}

// TestSyncResultEnvelope_CancelCause_Fleet pins ErrCancelFleet →
// CancelledReason=fleet_kill. No live producer today; future fleet
// kill wire-handler will attach this sentinel.
func TestSyncResultEnvelope_CancelCause_Fleet(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrCancelFleet)

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("peer killed"))

	if r.CancelledReason != CancelledReasonFleetKill {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonFleetKill)
	}
}

// TestSyncResultEnvelope_CancelCause_MCP pins ErrCancelMCP →
// CancelledReason=mcp_shutdown. No live producer today; future MCP
// shutdown path will attach this sentinel.
func TestSyncResultEnvelope_CancelCause_MCP(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(ErrCancelMCP)

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("mcp shutting down"))

	if r.CancelledReason != CancelledReasonMCPShutdown {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonMCPShutdown)
	}
}

// TestSyncResultEnvelope_CancelCause_UnknownError pins the precedence
// rule: a cancel cause that doesn't match any registered sentinel
// (e.g. a caller's own typed error) falls back to the generic
// "cancelled" bucket — the envelope refuses to invent attribution.
func TestSyncResultEnvelope_CancelCause_UnknownError(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errors.New("custom-domain cancel"))

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("torn down"))

	if r.CancelledReason != CancelledReasonCancelled {
		t.Errorf("CancelledReason = %q, want %q (unknown cause → generic)", r.CancelledReason, CancelledReasonCancelled)
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

	r := &Result{}
	syncResultEnvelope(ctx, r, errors.New("deadline exceeded"))

	if !r.Cancelled {
		t.Errorf("Cancelled should be true")
	}
	if r.CancelledReason != CancelledReasonTimeout {
		t.Errorf("CancelledReason = %q, want %q", r.CancelledReason, CancelledReasonTimeout)
	}
}

// TestSyncResultEnvelope_HandlerPreSetFailed_Preserved guards the
// "handler already populated Failed/Error" path — the helper must
// not clobber a handler-set diagnostic. Handlers that DO set Failed
// + Error explicitly (assert, shell with rc!=0) are the canonical
// path; the helper only fills the gap for the spec-69 B0 cluster.
func TestSyncResultEnvelope_HandlerPreSetFailed_Preserved(t *testing.T) {
	r := &Result{Failed: true, Error: "assertion failed: x != y", Rc: 1}
	syncResultEnvelope(context.Background(), r, errors.New("assertion failed: x != y"))
	if r.Error != "assertion failed: x != y" {
		t.Errorf("handler-set Error was clobbered: %q", r.Error)
	}
	if r.Rc != 1 {
		t.Errorf("Rc = %d, want 1 (handler-set)", r.Rc)
	}
}
