package git_checkout

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for git.checkout (spec-26
// phase 5).
//
// git.checkout has the genuinely-right inverse: capture HEAD pre-
// apply, build a git.checkout step that switches back to that sha
// on reverse. The blocker is the same as os.service: Run() does not
// today thread an apply-time Result that carries the pre-apply HEAD
// sha. Adding that requires a Run signature touch-up out of scope
// for this phase.
//
// This stub returns (nil, err) with a clear message pointing at
// the planned refactor, while keeping the compile-time
// `actions.Reverser` assertion so:
//   - the Reversible bit in CostEstimate reports "irreversible at
//     runtime" for today's git.checkout steps (true at design
//     time, false at apply time)
//   - tooling like `mooncake plan --check-reversible` flags
//     transactions containing git.checkout at plan time
//   - downstream callers don't have to special-case "handler
//     doesn't implement Reverser at all" vs. "handler refuses"
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.GitCheckout == nil {
		return nil, errors.New("git.checkout Reverse: step has no GitCheckout payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"git.checkout Reverse: not yet implemented. Apply-time pre-state " +
			"capture (HEAD sha) requires Run() to thread a Result with the " +
			"pre-checkout sha — a refactor tracked in spec-26 phase 5 " +
			"follow-up. Until then, transactions containing git.checkout " +
			"are not reversible at runtime.")
}

var _ actions.Reverser = (*Handler)(nil)
