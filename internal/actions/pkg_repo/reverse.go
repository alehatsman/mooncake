//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for pkg.repo (spec-22 phase 5 /
// spec-24 P6).
//
// pkg.repo has the genuinely-right inverse: flip state present↔absent
// and restore prior keyring/sources content. The blocker is the same
// pattern os.service / git.config hit: Run() doesn't yet thread an
// apply-time Result that captures the pre-apply file content, so a
// safe Reverse can't be built from step shape alone (a refusal lets
// users explicitly toggle state via a second pkg.repo step in
// try/catch/finally, which is the documented escape hatch).
//
// The compile-time `actions.Reverser` assertion is kept so:
//   - the Reversible bit in CostEstimate reports "designed
//     reversible" at plan time
//   - tooling like `mooncake plan --check-reversible` flags
//     transactions containing pkg.repo at plan time
//   - downstream callers don't have to special-case "handler
//     doesn't implement Reverser at all" vs. "handler refuses"
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.PkgRepo == nil {
		return nil, errors.New("pkg.repo Reverse: step has no PkgRepo payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"pkg.repo Reverse: not yet implemented. Apply-time pre-state " +
			"capture (existing sources.list.d entry + keyring content) " +
			"requires Run() to thread a typed Result — a refactor tracked " +
			"in spec-24 P6 follow-up. Until then, transactions containing " +
			"pkg.repo are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
