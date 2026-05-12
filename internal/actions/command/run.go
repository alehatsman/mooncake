package command

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Like shell, command actions can't be
// predicted without running them — plan mode always reports "not
// checkable". The executor's creates/unless idempotency guards still
// short-circuit before dispatch.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (command)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
