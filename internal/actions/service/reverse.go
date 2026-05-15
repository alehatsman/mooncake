package service

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for os.service (spec-22 phase
// 5 slice F).
//
// os.service has the genuinely-right inverse: capture pre-apply
// (State, Enabled) for the service, build an os.service step that
// restores those values on reverse. The blocker is that Run() in
// apply mode delegates to a legacy HandleService that returns just
// an `error` (no Result) and stashes its own Result via
// ec.CurrentResult as a side effect. Bolting ReverseData onto that
// shape cleanly requires a refactor of HandleService's signature
// that's out of scope for slice F.
//
// This stub returns (nil, err) with a clear message pointing at
// the planned refactor, while keeping the compile-time
// `actions.Reverser` assertion so:
//   - the Reversible bit in CostEstimate reports "irreversible" for
//     today's os.service steps
//   - tooling like `mooncake plan --check-reversible` flags
//     transactions containing os.service at plan time
//   - downstream callers don't have to special-case "handler
//     doesn't implement Reverser at all" vs. "handler refuses"
func (h *Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.OsService == nil {
		return nil, errors.New("os.service Reverse: step has no OsService payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"os.service Reverse: not yet implemented. Apply-time pre-state capture " +
			"(active/enabled) requires Run() in apply mode to thread a Result " +
			"through HandleService — a refactor tracked in spec-22 slice F " +
			"follow-up. Until then, transactions containing os.service steps " +
			"are not reversible.")
}

var _ actions.Reverser = (*Handler)(nil)
