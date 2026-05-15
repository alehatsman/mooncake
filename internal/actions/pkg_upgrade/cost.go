//nolint:revive // Package name matches action name convention (pkg_upgrade)
package pkg_upgrade

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for pkg.upgrade (spec-22 phase 6 /
// spec-24 P6).
//
// Risk bands match the legacy package handler's Upgrade=true
// conclusion: pkg.upgrade is the broadest-blast-radius action in
// the family.
//
//   - Full-system upgrade (Names empty)   → Risk 9 — system-wide;
//     can introduce breaking changes across all installed packages.
//     Intentionally irreversible.
//   - Subset upgrade (Names non-empty)    → Risk 8 — bounded but
//     still upgrading software; same per-package upgrade hazards.
//
// Resources counts targeted package names (0 → 1 for full upgrade
// so consumers don't see misleading zero). Bytes=-1 (manager-
// dependent).
//
// Reversible=false — declared irreversible by design. Versioning
// every package on apply would be prohibitive across managers, and
// the typical user expectation for "upgrade" is forward motion,
// not a reversible operation. Mirrors the legacy package handler's
// Upgrade=true conclusion.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: false,
		Risk:       9,
	}
	if step == nil || step.PkgUpgrade == nil {
		return cost, nil
	}
	if len(step.PkgUpgrade.Names) > 0 {
		cost.Resources = len(step.PkgUpgrade.Names)
		cost.Risk = 8
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
