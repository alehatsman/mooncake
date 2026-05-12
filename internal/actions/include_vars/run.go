package include_vars

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. include_vars only mutates the
// variable scope (by reading a YAML file), not the system. Plan mode
// reports Checkable=true with WouldChange=false; execute mode
// delegates.
//
// Like vars, the planner usually handles this at plan time; Run is
// here for completeness.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.Reason = "include_vars (no system change)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
