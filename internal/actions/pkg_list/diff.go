//nolint:revive // Package name matches action name convention (pkg_list)
package pkg_list

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for pkg.list (spec-22 phase 4 /
// spec-24 P6).
//
// pkg.list is a pure read action — it produces a result payload but
// makes no state change. Operation is always OpNoop. Resource carries
// the manager name in Identifier so multiple managers' queries show
// up distinctly in a plan listing.
//
// After is nil — there's no "would-change" intent to describe, and
// the actual installed-package list lands in the run-time Result
// rather than the plan-time Diff.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.PkgList == nil {
		return actions.Diff{}, errors.New("pkg.list Diff: step has no PkgList payload")
	}
	manager := step.PkgList.Manager
	if manager == "" {
		manager = "auto"
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: "list:" + manager,
		},
		Operation: actions.OpNoop,
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
