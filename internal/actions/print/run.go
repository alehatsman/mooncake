package print

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 entry point. Print is a no-op for state, so plan
// mode reports Checkable=true with WouldChange=false; execute mode
// delegates to the legacy Execute path which renders the message.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.Reason = "print (no state change)"
		return r, nil
	}
	return h.Execute(ctx, step)
}
