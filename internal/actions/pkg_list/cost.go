//nolint:revive // Package name matches action name convention (pkg_list)
package pkg_list

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for pkg.list (spec-22 phase 6 /
// spec-24 P6).
//
// Risk=1 — pkg.list is read-only. dpkg-query touches no state and is
// safe to run repeatedly. Lowest band on the risk scale; sits below
// any state-changing pkg action.
//
// Resources=1 (the "query" itself). Bytes=-1: result size depends
// on installed-package count.
//
// Reversible=false — there's nothing to reverse. pkg.list
// deliberately does NOT implement Reverser (Coster comment in
// handler_abi.go says Reversible mirrors interface presence; a
// pure read action correctly reports false).
func (Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: false,
		Risk:       1,
	}, nil
}

var _ actions.Coster = (*Handler)(nil)
