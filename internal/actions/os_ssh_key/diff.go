package os_ssh_key //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.ssh_key (spec-22 phase 4 /
// spec-27 P4 / spec-66 wave 8).
//
// Operation by state:
//
//	state=present (or empty) → OpCreate
//	state=absent             → OpDelete
//
// Conservative — Diff doesn't read the existing authorized_keys file
// at plan time (would couple to /home traversal + root reads). The
// runtime check produces accurate Changed=false on already-converged
// keys.
//
// Resource.Kind = ResourceOther, Identifier = "<user>:<key-count>"
// so plan listings distinguish multiple os.ssh_key steps for the
// same user. Attributes["kind"] = "os.ssh_key" dispatches the
// render_ssh_key matcher.
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsSSHKey == nil {
		return actions.Diff{}, errors.New("os.ssh_key Diff: step has no OsSSHKey payload")
	}
	k := step.OsSSHKey
	state := k.State
	if state == "" {
		state = "present"
	}

	keyCount := len(k.Keys)
	if k.Key != "" {
		keyCount++
	}

	op := actions.OpCreate
	if state == "absent" {
		op = actions.OpDelete
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: fmt.Sprintf("%s:%dkeys", k.User, keyCount),
			Attributes: map[string]string{"kind": "os.ssh_key", "user": k.User},
		},
		Operation: op,
		Before:    nil,
		After: &actions.SSHKeyDiff{
			User:      k.User,
			State:     state,
			KeyCount:  keyCount,
			Path:      k.Path,
			Exclusive: k.Exclusive,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
