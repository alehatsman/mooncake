package os_group //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.group (spec-22 phase 4 /
// spec-27 P4). The typed Before/After payload is actions.GroupDiff
// (spec-66 wave 3); see internal/actions/diff_payloads.go.
//
// Operation by state: present→OpCreate, absent→OpDelete. Conservative
// noop prediction — the runtime check produces accurate
// Changed=false on already-converged groups.
//
// Resource.Kind = ResourceOther, Identifier = group name,
// Attributes carries the canonical "kind" key so consumers can
// route on it without a per-action type switch.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsGroup == nil {
		return actions.Diff{}, errors.New("os.group Diff: step has no OsGroup payload")
	}
	g := step.OsGroup
	state := g.State
	if state == "" {
		state = "present"
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: g.Name,
			Attributes: map[string]string{"kind": "os.group"},
		},
		Operation: op,
		Before:    nil,
		After: &actions.GroupDiff{
			Name:   g.Name,
			State:  state,
			GID:    g.GID,
			System: g.System,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
