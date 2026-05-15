//nolint:revive // Package name matches action name convention (pkg_upgrade)
package pkg_upgrade

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for pkg.upgrade (spec-22 phase
// 5 / spec-24 P6).
//
// pkg.upgrade is declared irreversible BY DESIGN, mirroring the
// legacy package handler's Upgrade=true refusal. A safe reverse
// would need a pre-apply version snapshot of every package the
// upgrade touched — across whichever package manager — and then
// the ability to downgrade each one. That's prohibitive to capture
// reliably (`apt-cache policy` output is fragile, version-pinning
// arbitrary back-versions has its own failure modes), and the user
// expectation for "upgrade" is forward motion.
//
// The compile-time `actions.Reverser` assertion is kept so the
// transaction planner can probe Reverse() at plan time and surface
// the refusal early — same convention as git.clone.
// CostEstimate.Reversible is false (see cost.go); that's the
// canonical "would rollback do anything useful?" signal.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.PkgUpgrade == nil {
		return nil, errors.New("pkg.upgrade Reverse: step has no PkgUpgrade payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"pkg.upgrade Reverse: irreversible by design. Capturing per-package " +
			"pre-apply versions across every supported manager is prohibitive, " +
			"and downgrading arbitrary back-versions is its own failure mode. " +
			"Users needing rollback should wrap the upgrade in try/catch/finally " +
			"with explicit pkg.install steps pinning known-good versions.")
}

var _ actions.Reverser = (*Handler)(nil)
