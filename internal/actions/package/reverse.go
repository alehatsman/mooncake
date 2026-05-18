//nolint:revive,staticcheck // package_handler name required to avoid conflict with Go keyword
package package_handler

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// PkgReverseInfo is the per-step apply-time snapshot pkg stashes on
// Result.ReverseData. It captures *which* packages were actually
// mutated — the packages that flowed through `installPackages` /
// `removePackages` `toInstall` / `toRemove` lists — so Reverse can
// undo only the changes the apply genuinely made.
//
// Crucially, this excludes packages that were already in the
// desired state pre-apply: a state=present step that lists 10
// packages and finds 8 already installed only adds the 2 that
// needed installation to Mutated, so reverse removes exactly those
// 2 and leaves the pre-existing 8 alone.
type PkgReverseInfo struct {
	// AppliedState is the step's State at apply time, normalised
	// to either statePresent or stateAbsent. Determines the
	// inverse direction.
	AppliedState string

	// Manager is the package manager that ran the apply. The
	// reverse step pins to the same manager so a transaction
	// running across heterogeneous fleets doesn't auto-detect a
	// different one mid-rollback.
	Manager string

	// Cask mirrors pkg.Cask so the reverse step reinstates or
	// removes the package through the cask channel it was
	// installed from.
	Cask bool

	// Mutated is the list of package names that the apply
	// actually installed (when AppliedState=present) or actually
	// removed (when AppliedState=absent). nil/empty means the
	// step was a no-op and Reverse will return (nil, nil).
	Mutated []string
}

// Reverse implements actions.Reverser for pkg (spec-22 phase 5
// slice F).
//
// Scope and refusals:
//   - state=present with Upgrade=false → reverse is pkg{state=absent,
//     names=Mutated} (remove only what we actually installed)
//   - state=absent → reverse is pkg{state=present, names=Mutated}
//     (re-install only what we actually removed)
//   - state=latest → refuse. A "latest" step that upgrades an
//     already-installed package would need the prior version to
//     downgrade cleanly. Capturing per-package versions across
//     every package manager is out of scope for this slice; the
//     refusal points future slices at that question.
//   - Upgrade=true → refuse. Can't reverse a system-wide
//     "upgrade all packages" operation without a full pre-apply
//     version snapshot.
//   - No mutation captured (apply was a no-op) → return (nil, nil)
//     per the Reverser contract: nothing to undo.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.Pkg == nil {
		return nil, errors.New("pkg Reverse: step has no Pkg payload")
	}

	if step.Pkg.Upgrade {
		return nil, errors.New(
			"pkg Reverse: cannot reverse upgrade=true (would need a pre-apply " +
				"version snapshot of every installed package — out of scope)")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("pkg Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, errors.New("pkg Reverse: no ReverseData captured — Run must " +
			"set PkgReverseInfo before mutating package state")
	}
	info, ok := r.ReverseData.(*PkgReverseInfo)
	if !ok {
		return nil, fmt.Errorf("pkg Reverse: ReverseData is %T, want *PkgReverseInfo", r.ReverseData)
	}

	if len(info.Mutated) == 0 {
		// Apply was a no-op — every listed package was already in
		// the desired state. Nothing to reverse. (nil, nil) is the
		// Reverser contract for this.
		return nil, nil
	}

	switch info.AppliedState {
	case statePresent:
		// We installed Mutated; reverse removes them.
		return &config.Step{
			Name: fmt.Sprintf("reverse: remove %d package(s) installed by apply", len(info.Mutated)),
			Pkg: &config.Package{
				Names:   append([]string(nil), info.Mutated...),
				State:   stateAbsent,
				Manager: info.Manager,
				Cask:    info.Cask,
			},
		}, nil
	case stateAbsent:
		// We removed Mutated; reverse installs them.
		return &config.Step{
			Name: fmt.Sprintf("reverse: install %d package(s) removed by apply", len(info.Mutated)),
			Pkg: &config.Package{
				Names:   append([]string(nil), info.Mutated...),
				State:   statePresent,
				Manager: info.Manager,
				Cask:    info.Cask,
			},
		}, nil
	case stateLatest:
		// Reaching this branch implies installPackages captured
		// the latest-upgrade path. Refuse — downgrade requires a
		// version snapshot we don't take.
		return nil, errors.New(
			"pkg Reverse: cannot reverse state=latest (downgrade requires a " +
				"pre-apply version snapshot per package — out of scope)")
	default:
		return nil, fmt.Errorf("pkg Reverse: unknown captured AppliedState %q", info.AppliedState)
	}
}

var _ actions.Reverser = (*Handler)(nil)
