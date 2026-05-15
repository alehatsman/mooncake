package os_group //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// OsGroupSnapshot is the typed Before/After payload for actions.Diff
// when the resource is an os.group. Before stays nil; Diff doesn't
// shell to getent at plan time.
type OsGroupSnapshot struct {
	// Name is the group name (identity).
	Name string `json:"name,omitempty"`

	// State is the desired state: "present" or "absent". Empty maps
	// to "present" by handler convention.
	State string `json:"state,omitempty"`

	// GID, when set, is the requested numeric gid.
	GID *int `json:"gid,omitempty"`

	// System mirrors the system-group flag.
	System bool `json:"system,omitempty"`
}

// Diff implements actions.Differ for os.group (spec-22 phase 4 /
// spec-27 P4).
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
		After: &OsGroupSnapshot{
			Name:   g.Name,
			State:  state,
			GID:    g.GID,
			System: g.System,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
