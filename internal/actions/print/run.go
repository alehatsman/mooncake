package print

import (
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. print doesn't mutate system state.
// Plan mode renders the message and surfaces the first line so the
// preview shows what would be printed. Execute mode delegates.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true

		rendered, err := ctx.GetTemplate().Render(step.Print.Msg, ctx.GetVariables())
		if err != nil {
			rendered = step.Print.Msg
		}
		// Single-line preview: take first line, trim, truncate.
		preview := rendered
		if i := strings.IndexByte(preview, '\n'); i >= 0 {
			preview = preview[:i]
		}
		preview = strings.TrimSpace(preview)
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		if preview == "" {
			r.Reason = "would print message"
		} else {
			r.Reason = "would print: " + preview
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}
