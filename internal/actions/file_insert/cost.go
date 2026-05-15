//nolint:revive // package name follows action convention (file_insert)
package file_insert

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for text.insert (spec-22 phase 6).
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: 1, Bytes: -1, Reversible: true, Risk: 4}
	if step == nil || step.TextInsert == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
