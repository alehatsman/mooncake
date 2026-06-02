package agent

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

// approveResult runs cc.Approver against planBytes and returns its result,
// failing the test if it doesn't answer within a short deadline (a hang here
// means a routing bug, not a slow machine).
func approveResult(t *testing.T, cc *ControlChannel, ctx context.Context, plan string) (ConfirmResult, error) {
	t.Helper()
	type out struct {
		res ConfirmResult
		err error
	}
	done := make(chan out, 1)
	go func() {
		res, err := cc.Approver(nil)(ctx, []byte(plan))
		done <- out{res, err}
	}()
	select {
	case o := <-done:
		return o.res, o.err
	case <-time.After(2 * time.Second):
		t.Fatal("Approver did not return within 2s")
		return ConfirmResult{}, nil
	}
}

func TestControlChannel_Approve(t *testing.T) {
	cc := NewControlChannel(strings.NewReader(`{"type":"approve"}`+"\n"), func() {})
	res, err := approveResult(t, cc, context.Background(), "[]")
	if err != nil {
		t.Fatalf("Approver err: %v", err)
	}
	if res.Outcome != OutcomeApply {
		t.Errorf("Outcome = %v, want OutcomeApply", res.Outcome)
	}
}

func TestControlChannel_Reject(t *testing.T) {
	cc := NewControlChannel(strings.NewReader(`{"type":"reject"}`+"\n"), func() {})
	res, err := approveResult(t, cc, context.Background(), "[]")
	if err != nil {
		t.Fatalf("Approver err: %v", err)
	}
	if res.Outcome != OutcomeReject {
		t.Errorf("Outcome = %v, want OutcomeReject", res.Outcome)
	}
}

// TestControlChannel_Edit covers an edit message carrying a replacement plan:
// the approver returns OutcomeApply with the replacement bytes so RunLoop
// executes the driver's plan instead of the proposed one.
func TestControlChannel_Edit(t *testing.T) {
	edited := `[{"name":"x","cmd":{"argv":["true"]}}]`
	cc := NewControlChannel(strings.NewReader(`{"type":"edit","plan":`+edited+`}`+"\n"), func() {})
	res, err := approveResult(t, cc, context.Background(), "[]")
	if err != nil {
		t.Fatalf("Approver err: %v", err)
	}
	if res.Outcome != OutcomeApply {
		t.Errorf("Outcome = %v, want OutcomeApply", res.Outcome)
	}
	if string(res.PlanBytes) != edited {
		t.Errorf("PlanBytes = %q, want %q", res.PlanBytes, edited)
	}
}

// TestControlChannel_Stop covers a stop message invoking the run's cancel.
func TestControlChannel_Stop(t *testing.T) {
	canceled := make(chan struct{})
	NewControlChannel(strings.NewReader(`{"type":"stop"}`+"\n"), func() { close(canceled) })
	select {
	case <-canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("stop message did not invoke cancel within 2s")
	}
}

// TestControlChannel_CancelWhileParked covers a stop landing while the gate
// is parked waiting: the approver unblocks via ctx and returns OutcomeAbort
// with the ctx error, which RunLoop maps to StopCanceled.
func TestControlChannel_CancelWhileParked(t *testing.T) {
	// A reader that never yields a message and never EOFs, so the approver
	// can only be unblocked by ctx cancellation.
	pr, pw := io.Pipe()
	defer pw.Close()
	cc := NewControlChannel(pr, func() {})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	res, err := approveResult(t, cc, ctx, "[]")
	if err == nil {
		t.Error("Approver err = nil, want ctx error on cancel")
	}
	if res.Outcome != OutcomeAbort {
		t.Errorf("Outcome = %v, want OutcomeAbort", res.Outcome)
	}
}

// TestControlChannel_AnnounceCarriesPlan covers the announce callback being
// invoked with the plan bytes (the cmd layer uses it to emit
// plan.awaiting_approval).
func TestControlChannel_AnnounceCarriesPlan(t *testing.T) {
	cc := NewControlChannel(strings.NewReader(`{"type":"approve"}`+"\n"), func() {})
	var got string
	_, err := cc.Approver(func(pb []byte) { got = string(pb) })(context.Background(), []byte("the-plan"))
	if err != nil {
		t.Fatalf("Approver err: %v", err)
	}
	if got != "the-plan" {
		t.Errorf("announce got %q, want %q", got, "the-plan")
	}
}
