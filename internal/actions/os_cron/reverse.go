package os_cron //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.cron (spec-22 phase 5 /
// spec-28 P6).
//
// The inverse is straightforward in shape — flip present↔absent, and
// for content updates restore the prior file. A safe reverse needs
// apply-time capture of (a) whether the file existed pre-apply and
// (b) its prior content. Tracked in spec-28 P6 follow-up; until
// then this refuses.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsCron == nil {
		return nil, errors.New("os.cron Reverse: step has no OsCron payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.cron Reverse: not yet implemented. Apply-time pre-state " +
			"capture (file existence + prior content) requires Run() to " +
			"thread a typed Result — tracked in spec-28 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
