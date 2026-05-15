package os_mount //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.mount (spec-22 phase 5 /
// spec-28 P6).
//
// The inverse needs apply-time capture of the prior /etc/fstab
// state (entry present or absent; if present, full field set) plus
// the prior mount state (mounted or unmounted, with which options).
// Without that snapshot, a transaction rollback can't restore
// /etc/fstab faithfully or know whether to remount a volume that
// was incidentally unmounted by the original apply. Tracked in
// spec-28 P6 follow-up.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsMount == nil {
		return nil, errors.New("os.mount Reverse: step has no OsMount payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.mount Reverse: not yet implemented. Apply-time capture of " +
			"prior /etc/fstab entry + prior mount state requires Run() to " +
			"thread a typed Result — tracked in spec-28 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
