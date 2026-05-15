package os_ssh_key //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// OsSSHKeySnapshot is the typed Before/After payload for actions.Diff
// when the resource is an os.ssh_key step. Mirrors the other os.*
// snapshots: describes user INTENT (which keys for which user, in
// which file), not measured pre-state.
type OsSSHKeySnapshot struct {
	// User is the target account.
	User string `json:"user,omitempty"`

	// State is the desired state: "present" or "absent". Empty maps
	// to "present" by handler convention.
	State string `json:"state,omitempty"`

	// KeyCount is the number of keys this step targets. The
	// material itself isn't surfaced — it's user-supplied secret-
	// adjacent content and we don't want to echo it into Diff
	// outputs that the agent/UI may render.
	KeyCount int `json:"key_count"`

	// Path, when set, is the custom authorized_keys location.
	// Empty means the action will use ~user/.ssh/authorized_keys.
	Path string `json:"path,omitempty"`

	// Exclusive mirrors the "remove other keys" flag.
	Exclusive bool `json:"exclusive,omitempty"`
}

// Diff implements actions.Differ for os.ssh_key (spec-22 phase 4 /
// spec-27 P4).
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
// same user.
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
		After: &OsSSHKeySnapshot{
			User:      k.User,
			State:     state,
			KeyCount:  keyCount,
			Path:      k.Path,
			Exclusive: k.Exclusive,
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
