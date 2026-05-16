package fleet

// result.go materialises the fleet-scope kernel surface promised by
// docs-working-v2/vision/kernel.md §"The frontends — renderings, not
// products". FleetKernelResult is to fleet apply what
// apply.KernelResult (R1.1b) is to local apply: a typed shape that
// frontends (CLI, MCP, future drift-remediation) consume directly
// without re-deriving orchestration in cmd.
//
// The locked API contract (R2.1b prompt) is:
//
//	type FleetKernelResult struct {
//	    Plan    *plan.Plan
//	    Peers   map[PeerID]*apply.KernelResult
//	    Summary FleetSummary
//	}
//
//	func (r *FleetKernelResult) Reverse() (*FleetPlan, error)
//
// We honor the SHAPE. We can't honor the *populated* per-peer
// *apply.KernelResult yet because the controller-side SSE wire
// schema doesn't carry typed Steps + ReverseData back from the
// daemon — only the per-event summaries. See Reverse() and
// PeerResult.KernelResult for the gap. This is wave-4 Option B —
// the shape lands today, the wire wiring is a future spec.

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/plan"
)

// PeerID names a fleet member. Matches fleet.Peer.Name (a non-namespaced
// string from peers.toml). Kept as a typed alias so frontends can
// branch on the map shape without re-importing fleet.Peer.
type PeerID = string

// FleetKernelResult is the fleet-scope analog of apply.KernelResult.
// Returned from Orchestrator.Run; consumed by recap renderers, MCP
// fleet tools, and any future fleet-scope Reverse / explain logic.
//
// Plan is the in-memory representation of the plan that was
// dispatched (single-phase). In multi-phase machine-mode, Plan is
// nil and the per-peer/per-phase Plans live inside the daemon — see
// the Reverse() doc for the implication.
type FleetKernelResult struct {
	// Plan is the typed plan dispatched in single-phase mode, or
	// nil in machine-manifest mode (each phase ships its own plan
	// to its own peer; the multi-phase composition lives on the
	// controller as the manifest, not as a single plan.Plan).
	Plan *plan.Plan

	// Peers maps each selected peer's name to its outcome. The
	// PeerResult shape is documented per-field; importantly,
	// PeerResult.KernelResult is nil today (wire gap — see Reverse()).
	Peers map[PeerID]*PeerResult

	// Summary aggregates the four peer outcomes (ok / run-failed /
	// unreachable / total) plus the failing peer names. Mirrors
	// what cmd's old switch on out.Unreachable / out.RunFailed
	// produced; recap renderers read this directly.
	Summary FleetSummary
}

// PeerResult is the per-peer outcome of a fleet apply. Carries
// whatever the controller-side SSE stream surfaced during the run:
//
//   - RunID, Status, EventsCount, Sync: directly from
//     fleet.ApplyResult (the existing wire-shape).
//   - KernelResult: NIL today. The daemon owns the typed Steps +
//     ReverseData snapshot per step (spec-22 phase 5), but the SSE
//     event schema only carries StepID + Name + status, not the
//     full config.Step + *executor.Result. Populating this field
//     requires either a new daemon endpoint
//     (GET /v1/runs/{id}/result) returning the daemon's
//     apply.KernelResult over JSON, or a richer run.completed
//     event payload. Tracked as a follow-on spec (call it R2.1c
//     when filed).
//
// Frontends that need the typed kernel tail must check
// KernelResult != nil before relying on it; today they should
// expect nil and fall back to Status / Summary.
type PeerResult struct {
	RunID        string
	Status       string // run terminal status; empty if Apply errored before submit
	EventsCount  int
	Sync         SyncStats
	KernelResult *apply.KernelResult
}

// FleetSummary aggregates fleet-wide peer outcome counts. The four
// counters partition the peer set: TotalPeers = OK + RunFailed +
// Unreachable + (any peers skipped before submit, which the
// orchestrator filters out before populating this).
type FleetSummary struct {
	TotalPeers  int
	OK          int
	RunFailed   int
	Unreachable int
	FailedNames []string // peers that failed or were unreachable
}

// FleetPlan is the inverse-plan shape Reverse() returns: one plan
// per peer that, if applied to that peer, would restore its
// pre-mutation state. The map shape mirrors FleetKernelResult.Peers
// so callers can iterate in any order without coordinating ordering
// with the original apply.
type FleetPlan struct {
	ByPeer map[PeerID]*plan.Plan
}

// ErrPerPeerKernelResultNotWired is the typed sentinel
// FleetKernelResult.Reverse() returns when no peer carries a
// populated apply.KernelResult. Callers can check via errors.Is to
// distinguish "wire gap" from a per-peer Reverse failure.
var ErrPerPeerKernelResultNotWired = errors.New(
	"fleet: per-peer KernelResult.Steps not carried over SSE wire yet; " +
		"fleet-scope Reverse requires daemon to surface typed Steps + ReverseData " +
		"(tracked as the wire-protocol follow-on to R2.1b)")

// Reverse composes a FleetPlan by calling Reverse() on each peer's
// populated KernelResult and assembling the results.
//
// Today (Option B, R2.1b sub-scope): every PeerResult.KernelResult
// is nil because the controller-side SSE wire doesn't carry typed
// Steps + ReverseData back from the daemon. When that's true,
// Reverse() returns ErrPerPeerKernelResultNotWired — a typed sentinel
// frontends can check via errors.Is.
//
// Once the wire catches up (daemon serialises its apply.KernelResult
// via GET /v1/runs/{id}/result or a richer run.completed payload),
// PeerResult.KernelResult will be populated and Reverse() composes
// per-peer KernelResult.Reverse() into a FleetPlan with one entry
// per peer. The composition algorithm here is forward-compatible:
// it walks Peers, calls KernelResult.Reverse() on each populated
// entry, and surfaces the first per-peer error (no swallowing).
//
// If any per-peer Reverse fails, the partial FleetPlan up to that
// point is discarded and the error is wrapped with the peer name.
// This matches apply.KernelResult.Reverse's "fail loud, don't
// silently truncate" semantics.
func (r *FleetKernelResult) Reverse() (*FleetPlan, error) {
	if r == nil {
		return nil, errors.New("fleet: Reverse on nil FleetKernelResult")
	}

	populated := 0
	for _, pr := range r.Peers {
		if pr != nil && pr.KernelResult != nil {
			populated++
		}
	}
	if populated == 0 {
		return nil, ErrPerPeerKernelResultNotWired
	}

	byPeer := make(map[PeerID]*plan.Plan, populated)
	for id, pr := range r.Peers {
		if pr == nil || pr.KernelResult == nil {
			continue
		}
		inv, err := pr.KernelResult.Reverse()
		if err != nil {
			return nil, fmt.Errorf("reverse peer %q: %w", id, err)
		}
		byPeer[id] = inv
	}
	return &FleetPlan{ByPeer: byPeer}, nil
}

// ExitError carries the cli.Exit semantics across the orchestrator
// boundary without dragging urfave/cli into internal/fleet. The cmd
// layer asserts on this via errors.As and translates to cli.Exit.
//
// Internal (MCP / SDK) callers that don't care about the cli exit
// code can just call .Error() — the message is the same shape cmd
// used to print.
type ExitError struct {
	ExitCode int
	Message  string
	Cause    error
}

// Error implements error.
func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("fleet: exit %d", e.ExitCode)
}

// Unwrap returns the underlying cause, if any.
func (e *ExitError) Unwrap() error { return e.Cause }
