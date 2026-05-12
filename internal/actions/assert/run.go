package assert

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Assertions don't mutate system state,
// so plan mode delegates straight to Execute: the assertion is
// evaluated, and any failure becomes a plan failure (which is the
// intent — you want plan to catch a broken assertion).
//
// Reports Checkable=true and WouldChange=false when the assertion
// passes; a failure surfaces as a plan-time error.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	res, err := h.Execute(ctx, step)
	if err != nil {
		return res, err
	}
	if ctx.Mode() == actions.ModePlan {
		if r, ok := res.(*executor.Result); ok {
			r.Checkable = true
			r.Reason = "assertion passed"
		}
	}
	return res, nil
}
