package os_ssh_key //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.ssh_key (spec-22 phase
// 5 / spec-27 P4).
//
// Same pattern as os.user / os.group: the inverse shape is clear
// (flip state present↔absent on the same keys, or for exclusive
// steps restore the prior authorized_keys content) but a safe
// Reverse needs apply-time capture of which keys were genuinely
// added/removed. Without that, a state=present step would
// unconditionally remove every supplied key on rollback — even
// keys that pre-existed and weren't ours to add. That's worse
// than the original drift.
//
// Tracked as a spec-27 P4 follow-up. Until then this refuses
// explicitly; the compile-time `actions.Reverser` assertion stays
// so the transaction planner can probe at plan time.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsSSHKey == nil {
		return nil, errors.New("os.ssh_key Reverse: step has no OsSSHKey payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.ssh_key Reverse: not yet implemented. Apply-time pre-state " +
			"capture (which supplied keys were genuinely new vs. already " +
			"present) requires Run() to thread a typed Result — tracked " +
			"in spec-27 P4 follow-up. Until then, transactions containing " +
			"os.ssh_key are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
