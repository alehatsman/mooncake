package service

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Service state inspection (querying
// systemctl etc.) is non-trivial and not yet implemented — for now,
// plan mode reports "not checkable" and execute mode delegates to the
// legacy Execute path. Quality migration tracked as a follow-up.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (service state inspection not yet implemented)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
