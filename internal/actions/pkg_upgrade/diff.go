//nolint:revive // Package name matches action name convention (pkg_upgrade)
package pkg_upgrade

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// PkgUpgradeSnapshot is the typed Before/After payload for
// actions.Diff when the resource kind is ResourcePackage and the
// step is pkg.upgrade. Before is nil — Diff doesn't query the
// package manager for the per-package version delta.
type PkgUpgradeSnapshot struct {
	// Names is the explicit subset; empty means full-system upgrade.
	Names []string `json:"names,omitempty"`

	// Autoremove mirrors the step flag.
	Autoremove bool `json:"autoremove,omitempty"`

	// Manager is the explicit package manager when pinned.
	Manager string `json:"manager,omitempty"`

	// FullUpgrade is true when Names is empty (the system-wide
	// flavor). Exposed as a separate field so consumers don't have
	// to inspect Names emptiness in two places.
	FullUpgrade bool `json:"full_upgrade,omitempty"`
}

// Diff implements actions.Differ for pkg.upgrade (spec-22 phase 4 /
// spec-24 P6).
//
// Operation is always OpUpdate — pkg.upgrade is partially idempotent
// per its config docstring: there's no way to predict at plan time
// whether a real upgrade would occur without invoking the manager.
// Conservative + honest: report OpUpdate always; the runtime
// produces accurate Changed=true/false based on what apt-get
// actually did.
//
// Resource.Identifier carries either the first targeted package
// name (+overflow indicator) or "<system>" for the full-upgrade
// flavor.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.PkgUpgrade == nil {
		return actions.Diff{}, errors.New("pkg.upgrade Diff: step has no PkgUpgrade payload")
	}
	p := step.PkgUpgrade

	identifier := "<system>"
	full := len(p.Names) == 0
	if !full {
		identifier = p.Names[0]
		if len(p.Names) > 1 {
			identifier = fmt.Sprintf("%s (+%d more)", p.Names[0], len(p.Names)-1)
		}
	}

	names := append([]string(nil), p.Names...)

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: identifier,
		},
		Operation: actions.OpUpdate,
		Before:    nil,
		After: &PkgUpgradeSnapshot{
			Names:       names,
			Autoremove:  p.Autoremove,
			Manager:     p.Manager,
			FullUpgrade: full,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
