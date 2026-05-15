package unarchive

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for file.unarchive (spec-22
// phase 5 slice F).
//
// file.unarchive extracts an archive into a destination directory,
// producing N output files. The natural inverse is "delete the N
// files we extracted" — but expressing that as a single config.Step
// is impossible in today's contract: there's no "delete this list
// of paths" step. A transaction layer would need either multi-step
// reverse support (return []*config.Step) or a new variadic delete
// action.
//
// Until then this returns an explicit error pointing at the
// blocker. The compile-time `actions.Reverser` assertion is kept
// so the Reversible bit in CostEstimate accurately reports
// "irreversible" and tooling like `mooncake plan
// --check-reversible` flags transactions containing file.unarchive
// at plan time.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.FileUnarchive == nil {
		return nil, errors.New("file.unarchive Reverse: step has no FileUnarchive payload")
	}
	return nil, errors.New(
		"file.unarchive Reverse: cannot reverse extraction in a single step — " +
			"undoing requires deleting N extracted paths, which needs multi-step " +
			"reverse support (return []*config.Step) or a new variadic delete " +
			"action. Tracked in spec-22 slice F follow-up.")
}

var _ actions.Reverser = (*Handler)(nil)
