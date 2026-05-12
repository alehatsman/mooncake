package repo_apply_patchset

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Plan mode reports "not checkable"
// (predicting whether a patchset applies cleanly requires running git
// apply --check, which is itself a side-effecty operation we don't
// invoke in plan mode); execute mode delegates.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (repo_apply_patchset)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
