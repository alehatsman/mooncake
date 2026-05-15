//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for pkg (spec-22 phase 6).
//
// Risk bands track the destructiveness of each variant:
//   - state=present (install): 5 — adds software, but additive
//   - state=absent (remove): 7 — removing system packages is
//     genuinely risky (deps cascade, services break)
//   - state=latest (upgrade existing): 8 — upgrading can introduce
//     breaking changes and isn't reversible without version capture
//   - Upgrade=true (system-wide upgrade): 9 — broadest blast radius;
//     intentionally irreversible
//
// Resources counts declared package names. Bytes stays -1 since we
// don't query the package manager for sizes (Cost is cheap by
// contract).
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: -1, Bytes: -1, Reversible: true, Risk: 5}
	if step == nil || step.Pkg == nil {
		return cost, nil
	}
	if step.Pkg.Upgrade {
		cost.Risk = 9
		return cost, nil
	}
	cost.Resources = len(h.buildPackageList(step.Pkg))
	switch step.Pkg.State {
	case stateAbsent:
		cost.Risk = 7
	case stateLatest:
		cost.Risk = 8
	default:
		cost.Risk = 5
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
