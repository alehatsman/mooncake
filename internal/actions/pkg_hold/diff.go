//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// PkgHoldSnapshot is the typed Before/After payload for actions.Diff
// when the resource kind is ResourcePackage and the step is pkg.hold.
// Mirrors the convention of PkgSnapshot / PkgRepoSnapshot — describes
// user INTENT (names + state + manager), not measured pre-state.
// Before is nil; Diff doesn't shell to apt-mark at plan time.
type PkgHoldSnapshot struct {
	// Names targeted by the step.
	Names []string `json:"names,omitempty"`

	// State is "held" or "unheld"; empty defaults to "held".
	State string `json:"state,omitempty"`

	// Manager is the explicit package manager when pinned.
	Manager string `json:"manager,omitempty"`
}

// Diff implements actions.Differ for pkg.hold (spec-22 phase 4 /
// spec-24 P6).
//
// Operation:
//
//	state=held (or empty) → OpCreate  (adds the hold marker)
//	state=unheld          → OpDelete  (removes the hold marker)
//
// Conservative on noop prediction — Diff doesn't query apt-mark
// showhold; the runtime idempotency check collapses already-held
// inputs.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.PkgHold == nil {
		return actions.Diff{}, errors.New("pkg.hold Diff: step has no PkgHold payload")
	}
	p := step.PkgHold
	names := holdNames(p)
	state := p.State
	if state == "" {
		state = stateHeld
	}

	op := actions.OpCreate
	if state == stateUnheld {
		op = actions.OpDelete
	}

	identifier := "<unnamed>"
	if len(names) > 0 {
		identifier = names[0]
		if len(names) > 1 {
			identifier = fmt.Sprintf("%s (+%d more)", names[0], len(names)-1)
		}
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourcePackage,
			Identifier: identifier,
		},
		Operation: op,
		Before:    nil,
		After: &PkgHoldSnapshot{
			Names:   names,
			State:   state,
			Manager: p.Manager,
		},
	}, nil
}

func holdNames(p *config.PkgHold) []string {
	if len(p.Names) > 0 {
		out := make([]string, len(p.Names))
		copy(out, p.Names)
		return out
	}
	if p.Name != "" {
		return []string{p.Name}
	}
	return nil
}

var _ actions.Differ = (*Handler)(nil)
