//nolint:revive // package name follows action convention (file_delete_range)
package file_delete_range

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for text.delete_range (spec-22 phase 6).
// Risk=5 — deleting lines is one band above standard config writes
// because the deleted content is harder to reconstruct from intent
// alone if reverse refuses (oversized snapshot, etc.).
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: 1, Bytes: -1, Reversible: true, Risk: 5}
	if step == nil || step.TextDeleteRange == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
