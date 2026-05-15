package git_clone

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for git.clone (spec-26 phase 5,
// using the spec-22 cost framework).
//
// Risk=4 — git.clone writes a new directory and downloads tree
// objects over the network, but the writes are scoped to a single
// dest path the user nominated. One band above routine config
// writes (5 is the default; network involvement nudges toward
// "non-trivial" but stops short of "high impact": clone failures
// don't take services offline).
//
// Resources=1 — one repo dest. The N files inside the working
// tree don't multiply Resources; the action's API is "one repo at
// dest", not "N files".
//
// Bytes=-1 — repo size is unknown without an HTTP HEAD / ls-remote
// (and even then approximate). Cost is side-effect-free; we don't
// pay a network round-trip for an estimate.
//
// Reversible=false — git.clone declares itself irreversible by
// design (no rmtree-on-rollback to avoid foot-guns). The Reverser
// interface is implemented as an explicit refusal so the
// transaction layer surfaces this at plan time; this Reversible
// field mirrors the design intent rather than the interface
// presence.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: false,
		Risk:       4,
	}
	if step == nil || step.GitClone == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
