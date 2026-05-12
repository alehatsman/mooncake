package unarchive

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Plan mode reports "not checkable"
// (extracted-state inspection of an archive against a dir is
// non-trivial); execute mode delegates to the legacy Execute path
// which is already idempotent via its own checks.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (unarchive)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
