package preset

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Presets compose other steps; the
// planner expands them at plan time so this handler rarely runs.
// Plan mode reports "not checkable"; execute mode delegates.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (preset; usually expanded at plan time)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
