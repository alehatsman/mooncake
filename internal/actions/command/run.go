package command

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Like shell, command actions can't be
// predicted for idempotency. Plan mode surfaces the rendered argv so
// users see what would run. WouldChange is set because command steps
// are assumed to mutate state.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true

		rendered := make([]string, len(step.Command.Argv))
		for i, arg := range step.Command.Argv {
			out, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
			if err != nil {
				out = arg
			}
			rendered[i] = out
		}
		joined := strings.Join(rendered, " ")
		if len(joined) > 80 {
			joined = joined[:77] + "..."
		}
		if step.Become {
			r.Reason = fmt.Sprintf("would run (sudo): %s", joined)
		} else {
			r.Reason = fmt.Sprintf("would run: %s", joined)
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}
