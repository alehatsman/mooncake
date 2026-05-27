//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for pkg.repo (spec-22 phase 4 /
// spec-24 P6 / spec-66 wave 6).
//
// Operation:
//
//	state=present (or empty) → OpCreate
//	state=absent             → OpDelete
//
// Resource.Kind = ResourcePackage (the repo is a package-manager
// concept), Resource.Identifier = the repo name,
// Resource.Attributes["kind"] = "pkg.repo" so internal/diff can
// dispatch the render_repo matcher.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.PkgRepo == nil {
		return actions.Diff{}, errors.New("pkg.repo Diff: step has no PkgRepo payload")
	}
	r := step.PkgRepo
	state := r.State
	if state == "" {
		state = statePresent
	}

	driver := ""
	switch {
	case r.Apt != nil:
		driver = "apt"
	case r.Dnf != nil:
		driver = "dnf"
	case r.Brew != nil:
		driver = "brew"
	}

	op := actions.OpCreate
	if state == stateAbsent {
		op = actions.OpDelete
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: r.Name,
			Attributes: map[string]string{"kind": "pkg.repo"},
		},
		Operation: op,
		Before:    nil,
		After: &actions.RepoDiff{
			Name:   r.Name,
			State:  state,
			Driver: driver,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
