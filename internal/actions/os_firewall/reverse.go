package os_firewall //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.firewall (spec-22 phase 5
// / spec-28 P6).
//
// A safe inverse needs apply-time capture of the prior ufw rule set
// (`ufw status numbered`) so a rollback restores the exact pre-apply
// configuration — not a guess. Without that capture, naively
// removing the rules listed in the step risks dropping rules that
// existed pre-apply but happen to match the same shape. Tracked in
// spec-28 P6 follow-up.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsFirewall == nil {
		return nil, errors.New("os.firewall Reverse: step has no OsFirewall payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.firewall Reverse: not yet implemented. Apply-time capture of " +
			"the prior ufw rule set requires Run() to thread a typed Result " +
			"— tracked in spec-28 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
