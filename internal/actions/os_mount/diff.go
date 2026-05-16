package os_mount //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for os.mount (spec-22 phase 4 /
// spec-28 P6). The typed Before/After payload is actions.MountDiff
// (spec-66 wave 5); see internal/actions/diff_payloads.go. Note
// that OsMountSnapshotEntry in reverse.go is a different type used
// for ReverseData and is unrelated to this Diff payload.
//
// Operation by state:
//
//	mounted / fstab_only → OpCreate (adds an fstab entry + maybe mounts)
//	unmounted             → OpUpdate (keeps fstab entry but unmounts)
//	absent                → OpDelete (removes fstab entry + unmounts)
//
// Conservative — Diff doesn't read /etc/fstab or /proc/mounts.
//
// Resource.Kind = ResourceOther; Identifier = the mount point
// (consistent with the action's idempotency key).
func (Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.OsMount == nil {
		return actions.Diff{}, errors.New("os.mount Diff: step has no OsMount payload")
	}
	m := step.OsMount
	state := m.State
	if state == "" {
		state = "mounted"
	}

	op := actions.OpCreate
	switch state {
	case "absent":
		op = actions.OpDelete
	case "unmounted":
		op = actions.OpUpdate
	}

	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: m.Dest,
			Attributes: map[string]string{"kind": "os.mount"},
		},
		Operation: op,
		Before:    nil,
		After: &actions.MountDiff{
			Src:     m.Src,
			Dest:    m.Dest,
			FSType:  m.FSType,
			State:   state,
			Options: append([]string(nil), m.Options...),
		},
	}, nil
}

var _ actions.Differ = (*Handler)(nil)
