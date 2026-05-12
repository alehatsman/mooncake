package shell

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Shell commands can't be predicted
// for idempotency without running them (the executor's creates/unless
// guards already short-circuit before dispatch, so anything reaching
// Run will execute in normal mode).
//
// Plan mode surfaces the *rendered command text* so users can see
// what would run. WouldChange is set to true because we assume shell
// steps mutate state (matching the legacy Execute which always sets
// Changed=true).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true

		cmd := strings.TrimSpace(step.Shell.Cmd)
		rendered, err := ctx.GetTemplate().Render(cmd, ctx.GetVariables())
		if err != nil {
			rendered = cmd + " (template render would fail)"
		}
		preview := strings.ReplaceAll(rendered, "\n", " ")
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		if step.Become {
			r.Reason = fmt.Sprintf("would run (sudo): %s", preview)
		} else {
			r.Reason = fmt.Sprintf("would run: %s", preview)
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}
