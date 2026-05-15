package template

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for file.template (spec-22 phase 6).
// Resources is 1 (single dest). Bytes stays -1 because the rendered
// size depends on variable expansion which we deliberately don't run
// from Cost — Cost is supposed to be cheap and side-effect-free.
// Risk is routine config-write 4.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       4,
	}
	if step == nil || step.FileTemplate == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
