package fleet

// result.go materialises the fleet-scope kernel surface promised by
// docs-working-v2/vision/kernel.md §"The frontends — renderings, not
// products". KernelResult is to fleet apply what
// apply.KernelResult (R1.1b) is to local apply: a typed shape that
// frontends (CLI, MCP, future drift-remediation) consume directly
// without re-deriving orchestration in cmd.
//
// The locked API contract (R2.1b prompt) is:
//
//	type KernelResult struct {
//	    Plan    *plan.Plan
//	    Peers   map[PeerID]*apply.KernelResult
//	    Summary Summary
//	}
//
//	func (r *KernelResult) Reverse() (*Plan, error)
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

// KernelResult is the fleet-scope analog of apply.KernelResult.
// Returned from Orchestrator.Run; consumed by recap renderers, MCP
// fleet tools, and any future fleet-scope Reverse / explain logic.
//
// Plan is the in-memory representation of the plan that was
// dispatched (single-phase). In multi-phase machine-mode, Plan is
// nil and the per-peer/per-phase Plans live inside the daemon — see
// the Reverse() doc for the implication.
type KernelResult struct {
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
	Summary Summary
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

// Summary aggregates fleet-wide peer outcome counts. The four
// counters partition the peer set: TotalPeers = OK + RunFailed +
// Unreachable + (any peers skipped before submit, which the
// orchestrator filters out before populating this).
type Summary struct {
	TotalPeers  int
	OK          int
	RunFailed   int
	Unreachable int
	FailedNames []string // peers that failed or were unreachable
}

// Plan is the inverse-plan shape Reverse() returns: one plan
// per peer that, if applied to that peer, would restore its
// pre-mutation state. The map shape mirrors KernelResult.Peers
// so callers can iterate in any order without coordinating ordering
// with the original apply.
type Plan struct {
	ByPeer map[PeerID]*plan.Plan
}

// ErrPerPeerKernelResultNotWired is the typed sentinel
// KernelResult.Reverse() returns when no peer carries a
// populated apply.KernelResult. Callers can check via errors.Is to
// distinguish "wire gap" (a peer's daemon didn't surface its
// KernelResult — pre-R2.1c version, or a daemon that hasn't
// reached the terminal state yet) from a per-peer Reverse failure.
var ErrPerPeerKernelResultNotWired = errors.New(
	"fleet: per-peer KernelResult is nil for every peer; " +
		"this typically means the daemon hasn't surfaced its " +
		"apply.KernelResult yet (run not terminal, or pre-R2.1c daemon)")

// Reverse composes a Plan by calling Reverse() on each peer's
// populated KernelResult and assembling the results.
//
// Wire state today (R2.1c phase 1+2 landed): each PeerResult.KernelResult
// arrives populated from the daemon via GET /v1/runs/{id}/result,
// carrying typed Steps and discriminator-tagged ReverseData. Per-peer
// Reverse() composes against the captured pre-state without falling
// back to the refusal stubs that an empty ReverseData would surface.
//
// ErrPerPeerKernelResultNotWired is now reserved for the "daemon
// hasn't completed yet" / "pre-R2.1c daemon" cases. Mixed-version
// fleets degrade gracefully: an unknown ReverseData discriminator
// decodes to nil and the handler's existing
// "no ReverseData captured" refusal surfaces per-step, which is
// the same shape as a true wire-gap.
//
// If any per-peer Reverse fails, the partial Plan up to that
// point is discarded and the error is wrapped with the peer name.
// This matches apply.KernelResult.Reverse's "fail loud, don't
// silently truncate" semantics.
func (r *KernelResult) Reverse() (*Plan, error) {
	if r == nil {
		return nil, errors.New("fleet: Reverse on nil KernelResult")
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
	return &Plan{ByPeer: byPeer}, nil
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
