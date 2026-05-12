package artifact_capture

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Artifact capture re-runs its
// inner steps every execute (it's not idempotent — capturing is the
// point), so plan mode always reports would-change with the inner-step
// count for context.
//
// Deep inspection (running each inner step's plan-mode prediction
// recursively) is a follow-up; today the plan output for an
// artifact_capture step shows the step's name and inner count, while
// the inner steps themselves are not inspected.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	c := step.ArtifactCapture
	result := executor.NewResult()
	result.Checkable = true
	result.WouldChange = true
	result.Reason = fmt.Sprintf("would capture artifact %q (%d inner step(s))", c.Name, len(c.Steps))
	return result, nil
}
