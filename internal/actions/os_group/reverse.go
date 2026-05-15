package os_group //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.group (spec-22 phase 5 /
// spec-27 P4).
//
// The inverse shape is trivial — flip state present↔absent on the
// same name. A safe reverse still wants apply-time capture of
// whether the group already existed pre-apply (a state=present
// step that finds the group already there should not be reversed
// to delete-the-group: that group might predate our operation).
// Apply-time getent snapshot tracked in the spec-27 P4 follow-up;
// until then this refuses with a clear pointer.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsGroup == nil {
		return nil, errors.New("os.group Reverse: step has no OsGroup payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.group Reverse: not yet implemented. Apply-time pre-state " +
			"capture (whether the group existed pre-apply, prior gid, " +
			"members) requires Run() to thread a typed Result — tracked " +
			"in spec-27 P4 follow-up. Until then, transactions containing " +
			"os.group are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
