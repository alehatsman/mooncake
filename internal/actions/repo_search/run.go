package repo_search

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. repo_search reads repository state
// without mutating, so plan mode delegates to Execute and surfaces
// Checkable=true.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	res, err := h.Execute(ctx, step)
	if err != nil {
		return res, err
	}
	if ctx.Mode() == actions.ModePlan {
		if r, ok := res.(*executor.Result); ok {
			r.Checkable = true
			r.Reason = "repo_search (read-only)"
		}
	}
	return res, nil
}
