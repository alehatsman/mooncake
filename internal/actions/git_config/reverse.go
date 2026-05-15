package git_config

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for git.config (spec-26 phase 5).
//
// git.config has the genuinely-right inverse: capture each key's
// pre-apply value, build a git.config step that restores those
// values (or unsets keys that had no prior value) on reverse.
//
// Same blocker as git.checkout / os.service: Run() does not thread
// an apply-time Result carrying the per-key pre-apply observations.
// Adding that needs a Run signature touch-up out of scope for this
// phase.
//
// This stub returns (nil, err) with a clear message pointing at
// the planned refactor, while keeping the compile-time
// `actions.Reverser` assertion so:
//   - the Reversible bit in CostEstimate reports "designed
//     reversible" for today's git.config steps
//   - tooling like `mooncake plan --check-reversible` flags
//     transactions containing git.config at plan time
//   - downstream callers don't have to special-case "handler
//     doesn't implement Reverser at all" vs. "handler refuses"
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.GitConfig == nil {
		return nil, errors.New("git.config Reverse: step has no GitConfig payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"git.config Reverse: not yet implemented. Apply-time pre-state " +
			"capture (per-key observed values) requires Run() to thread a " +
			"Result with the prior key/value pairs — a refactor tracked in " +
			"spec-26 phase 5 follow-up. Until then, transactions containing " +
			"git.config are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
