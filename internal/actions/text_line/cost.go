//nolint:revive // package name follows action convention
package text_line

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for text.line (spec-22 phase 6).
// Resources=1 (single file). Bytes=-1 (delta unknown — could be a
// new line, a replacement, or a removal). Risk=4 routine.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: 1, Bytes: -1, Reversible: true, Risk: 4}
	if step == nil || step.TextLine == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
