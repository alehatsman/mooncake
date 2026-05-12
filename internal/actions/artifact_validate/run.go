package artifact_validate

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Artifact validation doesn't mutate
// state — plan mode delegates to Execute so the validation runs (a
// failing validation should fail the plan), reporting Checkable=true
// when successful.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	res, err := h.Execute(ctx, step)
	if err != nil {
		return res, err
	}
	if ctx.Mode() == actions.ModePlan {
		if r, ok := res.(*executor.Result); ok {
			r.Checkable = true
			r.Reason = "artifact validated"
		}
	}
	return res, nil
}
