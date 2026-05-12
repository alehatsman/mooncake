package wait

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. wait actions block until some
// condition holds; predicting their outcome requires actually
// performing the wait, so plan mode reports "not checkable".
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (wait)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
