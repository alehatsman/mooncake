//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for pkg.hold (spec-22 phase 6 /
// spec-24 P6).
//
// Risk=3 — marking a package as held writes a single dpkg state
// flag. No package install/remove, no network, no service restarts.
// Below routine config writes (4) because the only effect is "future
// upgrades will skip this name"; the system as-of-now is unchanged.
//
// Resources counts targeted package names. Bytes=-1 (negligible).
//
// Reversible=true — the Reverser interface is implemented; the
// natural inverse (toggle held↔unheld on the same names) is the
// cleanest reverse in the pkg.* family. Apply-time capture lands in
// a follow-up; until then the refusal points there.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  0,
		Bytes:      -1,
		Reversible: true,
		Risk:       3,
	}
	if step == nil || step.PkgHold == nil {
		return cost, nil
	}
	cost.Resources = len(holdNames(step.PkgHold))
	if cost.Resources == 0 {
		cost.Resources = 1
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
