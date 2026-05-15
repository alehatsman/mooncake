package unarchive

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for file.unarchive (spec-22 phase 6).
// Resources=-1 (unknown until extraction). Bytes=-1 likewise. Risk=6
// — extracting an archive drops many files at once and the inverse
// is currently a refusal (multi-step reverse not yet supported);
// raised one band above standard config writes to surface that
// asymmetry in plan output.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: -1, Bytes: -1, Reversible: true, Risk: 6}
	if step == nil || step.FileUnarchive == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
