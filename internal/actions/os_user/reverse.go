package os_user //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.user (spec-22 phase 5 /
// spec-27 P4).
//
// The right inverse is straightforward in shape — flip state
// present↔absent, plus for "present" modifications restore the
// pre-apply attribute set (uid, gid, shell, groups, home,
// comment). The blocker is the same shape git.checkout / pkg.repo
// hit: Run() doesn't yet thread a typed Result carrying the
// pre-apply getent snapshot, and reconstructing a faithful
// pre-state without it is unsafe (e.g. partial group restoration
// would leak existing membership). Tracked as a spec-27 P4
// follow-up.
//
// The compile-time `actions.Reverser` assertion is kept so the
// transaction planner can probe Reverse() at plan time and surface
// the refusal early.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsUser == nil {
		return nil, errors.New("os.user Reverse: step has no OsUser payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.user Reverse: not yet implemented. Apply-time pre-state " +
			"capture (getent snapshot: uid, gid, shell, groups, home, " +
			"comment) requires Run() to thread a typed Result — tracked " +
			"in spec-27 P4 follow-up. Until then, transactions containing " +
			"os.user are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
