//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for pkg.repo (spec-22 phase 6 /
// spec-24 P6).
//
// Risk=6 — adding or removing a third-party repository changes the
// universe of packages the host will trust. Higher than pkg.install
// (5) because the blast radius is "every future install on this
// box", not just the named package. Below the destructive band (7+)
// because the action is bounded to two files (sources.list.d entry
// + keyring) and the actual package set on disk doesn't change
// until a downstream pkg.install runs.
//
// Resources=1 — the repo is a single resource (the action manages
// one sources/keyring pair per step). Bytes=-1: the keyring is on
// the order of a few KB, not worth estimating from Cost.
//
// Reversible=true — the Reverser interface is implemented (returns
// "needs apply-time prior-state capture" refusal in v1; the natural
// inverse — toggle state present/absent — is straightforward once
// the apply pipeline threads a Result with the pre-state).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       6,
	}
	if step == nil || step.PkgRepo == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
