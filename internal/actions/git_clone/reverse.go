package git_clone

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for git.clone (spec-26 phase 5).
//
// git.clone is declared irreversible BY DESIGN. The natural inverse
// is "delete the cloned repo" — but rmtree-on-rollback is a
// foot-gun: an operator who clones into ~/myproject and hits a
// transaction failure does not want their workspace deleted. Users
// who genuinely need rollback cleanup are expected to build it via
// `try/catch/finally` + an explicit `file.write state: absent` step
// (per spec-26 design rationale).
//
// The compile-time `actions.Reverser` assertion is kept so:
//   - tooling like `mooncake plan --check-reversible` flags
//     transactions containing git.clone at plan time
//   - downstream callers don't have to special-case "handler
//     doesn't implement Reverser at all" vs. "handler refuses"
//
// CostEstimate.Reversible is set to false (see cost.go) to surface
// the design intent in plan output; that's the canonical signal for
// "would this rollback do anything useful?".
func (Handler) Reverse(_ actions.Context, step *config.Step, _ actions.Result) (*config.Step, error) {
	if step == nil || step.GitClone == nil {
		return nil, errors.New("git.clone Reverse: step has no GitClone payload")
	}
	return nil, errors.New( //nolint:staticcheck
		"git.clone Reverse: irreversible by design. rmtree-on-rollback would " +
			"delete user workspaces on transaction failure; spec-26 routes that " +
			"workflow through try/catch/finally + an explicit file.write absent " +
			"step.")
}

var _ actions.Reverser = (*Handler)(nil)
