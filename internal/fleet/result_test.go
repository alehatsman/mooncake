package fleet

import (
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
)

// TestFleetKernelResult_Reverse_WireGap covers the documented Option B
// behaviour: with no peer carrying a populated apply.KernelResult,
// Reverse returns ErrPerPeerKernelResultNotWired. This is the
// expected state TODAY because the SSE wire schema doesn't carry the
// daemon's full KernelResult tail back to the controller.
func TestFleetKernelResult_Reverse_WireGap(t *testing.T) {
	r := &FleetKernelResult{
		Peers: map[PeerID]*PeerResult{
			"main_pc": {RunID: "r1", Status: "success"},
			"laptop":  {RunID: "r2", Status: "success"},
		},
	}
	plan, err := r.Reverse()
	if err == nil {
		t.Fatalf("expected error for wire-gap state, got plan=%v", plan)
	}
	if !errors.Is(err, ErrPerPeerKernelResultNotWired) {
		t.Errorf("expected ErrPerPeerKernelResultNotWired, got %v", err)
	}
}

// TestFleetKernelResult_Reverse_ComposesPopulatedPeers covers the
// forward-compatible algorithm: when at least one peer DOES carry a
// populated apply.KernelResult (the future wire shape), Reverse
// composes a FleetPlan with one entry per populated peer. We use
// synthetic empty KernelResults here — the inner Reverse() returns
// an empty inverse plan, but the assembly + per-peer key shape is
// what's under test.
func TestFleetKernelResult_Reverse_ComposesPopulatedPeers(t *testing.T) {
	r := &FleetKernelResult{
		Peers: map[PeerID]*PeerResult{
			"main_pc": {
				RunID:        "r1",
				Status:       "success",
				KernelResult: &apply.KernelResult{},
			},
			"laptop": {
				RunID:        "r2",
				Status:       "success",
				KernelResult: &apply.KernelResult{},
			},
			"vps-1": {
				// No KernelResult — should be skipped, not error.
				RunID:  "r3",
				Status: "success",
			},
		},
	}
	got, err := r.Reverse()
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil FleetPlan")
	}
	// Two populated peers → two entries; vps-1 (no KernelResult) skipped.
	if len(got.ByPeer) != 2 {
		t.Errorf("FleetPlan.ByPeer = %d entries, want 2 (skipping vps-1)", len(got.ByPeer))
	}
	for _, want := range []PeerID{"main_pc", "laptop"} {
		if _, ok := got.ByPeer[want]; !ok {
			t.Errorf("FleetPlan.ByPeer missing %q", want)
		}
	}
}

// TestFleetKernelResult_Reverse_NilReceiver guards against nil-pointer
// dereference; frontends might call Reverse on a nil result if an
// upstream error path returned nil.
func TestFleetKernelResult_Reverse_NilReceiver(t *testing.T) {
	var r *FleetKernelResult
	_, err := r.Reverse()
	if err == nil {
		t.Fatal("expected error on nil receiver")
	}
}

// TestExitError covers the *fleet.ExitError shape cmd asserts on
// via errors.As to translate to cli.Exit.
func TestExitError(t *testing.T) {
	e := &ExitError{ExitCode: 2, Message: "fleet apply: unreachable peer(s): main_pc"}
	if got := e.Error(); got != "fleet apply: unreachable peer(s): main_pc" {
		t.Errorf("Error() = %q, want message verbatim", got)
	}
	// Empty message + cause: should surface cause.Error()
	cause := errors.New("upstream boom")
	e2 := &ExitError{ExitCode: 1, Cause: cause}
	if got := e2.Error(); got != "upstream boom" {
		t.Errorf("Error() with empty message and cause = %q, want %q", got, "upstream boom")
	}
	if !errors.Is(e2, cause) {
		t.Error("errors.Is should walk Unwrap → Cause")
	}
}
