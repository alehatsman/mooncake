package vars

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. vars steps only mutate the variable
// scope, not the system. Plan mode reports Checkable=true with
// WouldChange=false; execute mode delegates.
//
// Note: the planner already evaluates vars at plan time and strips
// them from the step list, so this handler rarely runs in practice.
// Run is here for completeness and to satisfy the Runner contract.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.Reason = "vars (no system change)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
