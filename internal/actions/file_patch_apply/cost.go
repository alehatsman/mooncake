//nolint:revive // package name follows action convention (file_patch_apply)
package file_patch_apply

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for text.patch (spec-22 phase 6).
// Risk=5 — applying a unified diff can produce surprising results
// when hunks fail; one band above straight replacement to reflect
// that.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: 1, Bytes: -1, Reversible: true, Risk: 5}
	if step == nil || step.TextPatch == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
