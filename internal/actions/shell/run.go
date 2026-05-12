package shell

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Shell commands can't be predicted
// without running them, so plan mode always reports "not checkable".
// Execute mode delegates to the legacy Execute path.
//
// The executor still honors `creates:` / `unless:` idempotency before
// dispatch, so plans with those guards on shell steps will show as
// skipped (not as "not checkable"). What this Run returns only matters
// when the step would actually run.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (shell command)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
