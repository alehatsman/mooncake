package os_systemd //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.systemd (spec-22 phase
// 5 / spec-28 P6).
//
// A safe inverse needs apply-time capture of: (a) whether the unit
// file existed pre-apply and its prior content, (b) the prior
// `systemctl is-enabled` / `is-active` results. Without those, a
// transaction rollback can't tell "delete the unit" from "restore
// the prior content" and can't restore Enabled/Started to their
// real prior state. Tracked in spec-28 P6 follow-up.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsSystemd == nil {
		return nil, errors.New("os.systemd Reverse: step has no OsSystemd payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.systemd Reverse: not yet implemented. Apply-time pre-state " +
			"capture (unit-file existence + content + is-enabled / " +
			"is-active) requires Run() to thread a typed Result — tracked " +
			"in spec-28 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
