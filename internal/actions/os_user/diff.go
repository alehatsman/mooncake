package os_user //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.user (spec-22 phase 4 /
// spec-27 P4). The typed Before/After payload is actions.UserDiff
// (spec-66 wave 3); see internal/actions/diff_payloads.go for the
// wire shape.
//
// Operation classification by state:
//
//	state=present (or empty) → OpCreate  (idempotency catches noop at apply)
//	state=absent             → OpDelete
//
// Conservative on noop prediction — Diff doesn't getent the user at
// plan time (would couple plan to /etc/passwd reads and root perms).
// The runtime path produces accurate Changed=false on converged
// systems.
//
// Resource.Kind = ResourceOther (os.user isn't a file, package, or
// service); Resource.Identifier = the account name.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsUser == nil {
		return actions.Diff{}, errors.New("os.user Diff: step has no OsUser payload")
	}
	u := step.OsUser
	state := u.State
	if state == "" {
		state = "present"
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	groups := make([]string, 0, len(u.Groups)+1)
	if u.Group != "" {
		groups = append(groups, u.Group)
	}
	groups = append(groups, u.Groups...)

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: u.Name,
			Attributes: map[string]string{"kind": "os.user"},
		},
		Operation: op,
		Before:    nil,
		After: &actions.UserDiff{
			Name:   u.Name,
			State:  state,
			UID:    u.UID,
			Shell:  u.Shell,
			Home:   u.Home,
			Groups: groups,
			System: u.System,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
