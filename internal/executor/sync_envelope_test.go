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

// TestSyncResultEnvelope_RunCtxCanceled_Generic covers the proposal-02
// attribution: when the run-wide ctx is cancelled at the moment the
// handler errors, classify as Cancelled rather than Failed and bump
// Stats.Cancelled. A plain context.WithCancel with no cause attached
// maps to CancelledReason=cancelled (the F4 generic bucket — producer
// didn't attribute the cancel, so the envelope refuses to guess).
func TestSyncResultEnvelope_RunCtxCanceled_Generic(t *testing.T) {
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
	if r.CancelledReason != CancelledReasonCancelled {
		t.Errorf("CancelledReason = %q, want %q (no cause attached → generic)", r.CancelledReason, CancelledReasonCancelled)
	}
	if r.Error != "aborted mid-write" {
		t.Errorf("Error = %q, want %q", r.Error, "aborted mid-write")
	}
	if *stats.Cancelled != 1 {
		t.Errorf("Stats.Cancelled = %d, want 1", *stats.Cancelled)
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
	syncResultEnvelope(ctx, r, errors.New("aborted by ^C"), nil)

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
	syncResultEnvelope(ctx, r, errors.New("peer killed"), nil)

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
	syncResultEnvelope(ctx, r, errors.New("mcp shutting down"), nil)

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
	syncResultEnvelope(ctx, r, errors.New("torn down"), nil)

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
