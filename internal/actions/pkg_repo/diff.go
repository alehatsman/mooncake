//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// PkgRepoSnapshot is the typed Before/After payload for actions.Diff
// when the resource kind is ResourcePackage and the step is pkg.repo.
// Same convention as PkgSnapshot in internal/actions/package: the
// snapshot describes user INTENT (which repo, which state, which
// driver), not measured pre-state. Before stays nil because reading
// the existing sources list and matching it against the requested
// shape is non-trivial across managers and would couple Diff to disk
// I/O — out of scope for the cheap-Diff contract.
type PkgRepoSnapshot struct {
	// Name is the repo identifier (used as the on-disk filename).
	Name string `json:"name,omitempty"`

	// State is the desired state: "present" or "absent". Empty maps
	// to "present" by handler convention.
	State string `json:"state,omitempty"`

	// Driver names which manager block is populated: "apt" / "dnf" /
	// "brew" / "" (none). Lets consumers branch on the driver
	// without inspecting the typed struct.
	Driver string `json:"driver,omitempty"`
}

// Diff implements actions.Differ for pkg.repo (spec-22 phase 4 / spec-24 P6).
//
// Operation:
//
//	state=present (or empty) → OpCreate
//	state=absent             → OpDelete
//
// Resource.Kind = ResourcePackage (the repo is a package-manager
// concept), Resource.Identifier = the repo name.
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
		},
		Operation: op,
		Before:    nil,
		After: &PkgRepoSnapshot{
			Name:   r.Name,
			State:  state,
			Driver: driver,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
