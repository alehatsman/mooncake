package os_sysctl //nolint:revive // package name follows action convention

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.sysctl (spec-22 phase 5 /
// spec-28 P6).
//
// The inverse is conceptually clean — re-set the key to its
// pre-apply runtime value and restore the corresponding line in
// /etc/sysctl.d/99-mooncake.conf. A safe reverse needs both pieces
// captured at apply time (the runtime value via `sysctl -n <key>`
// and the file-line state); reconstructing either from the step
// shape alone risks setting an arbitrary default. Tracked in
// spec-28 P6 follow-up.
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsSysctl == nil {
		return nil, errors.New("os.sysctl Reverse: step has no OsSysctl payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.sysctl Reverse: not yet implemented. Apply-time pre-state " +
			"capture (prior runtime value + file-line state) requires Run() " +
			"to thread a typed Result — tracked in spec-28 P6 follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
