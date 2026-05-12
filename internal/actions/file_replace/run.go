package file_replace

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Quality state inspection (compute
// the replacement result and compare to current content) is a
// follow-up; for now plan mode reports "not checkable" and execute
// mode delegates to the legacy atomic-write Execute path.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (file_replace)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
