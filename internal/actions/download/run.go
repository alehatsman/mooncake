package download

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Plan mode reports "not checkable"
// because the download handler already short-circuits on existing
// files at execute time (via checksum/size match); the deeper
// state-inspection migration is tracked for a follow-up.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Reason = "not checkable (download)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
