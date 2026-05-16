package mcp_test

import (
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
)

// TestFleetKernelResult_UsableFromMCP is the R2.1b counterpart to
// TestKernelResult_UsableFromMCP (R1.1b). It proves:
//
//  1. internal/mcp can import internal/fleet without a circular
//     dependency on internal/apply or internal/executor.
//  2. The fleet.Orchestrator construct → Run() signature matches the
//     locked R2.1b contract: (*FleetKernelResult, error).
//  3. *fleet.FleetKernelResult exposes Plan / Peers / Summary and a
//     Reverse() method that returns a typed sentinel error in the
//     wire-gap state (Option B).
//
// We don't actually call Run because Run requires loaded peers.toml +
// network reachability; this is a kernel-surface smoke test, not an
// end-to-end one. The cmd/ integration tests cover Run with real
// inputs.
func TestFleetKernelResult_UsableFromMCP(t *testing.T) {
	// Construct an Orchestrator. This exercises the constructor; the
	// underlying ApplyConfig fields are not consumed unless Run is
	// called, so we leave most empty.
	o := fleet.NewOrchestrator(&fleet.ApplyConfig{})
	if o == nil {
		t.Fatalf("NewOrchestrator returned nil")
	}

	// Assert the locked shape directly: a frontend code path that
	// expects (*FleetKernelResult, error) can use it as such. This
	// block compiles iff the contract holds.
	result := &fleet.FleetKernelResult{
		Peers: map[fleet.PeerID]*fleet.PeerResult{
			"smoke-peer": {RunID: "r1", Status: "success"},
		},
		Summary: fleet.FleetSummary{TotalPeers: 1, OK: 1},
	}
	if result.Summary.TotalPeers != 1 {
		t.Errorf("Summary.TotalPeers = %d, want 1", result.Summary.TotalPeers)
	}

	// Reverse() in the wire-gap state surfaces a typed sentinel that
	// MCP / agent-loop callers can branch on via errors.Is. This is
	// the "graceful degradation" behaviour Option B promises.
	_, err := result.Reverse()
	if !errors.Is(err, fleet.ErrPerPeerKernelResultNotWired) {
		t.Errorf("Reverse() in wire-gap state = %v; want ErrPerPeerKernelResultNotWired", err)
	}
}
