package copy

import (
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for file.copy (spec-22 phase 6).
// Resources is 1 (single dest). Bytes is the source file size when
// stat-able — the apply will write that many bytes. Risk is a
// routine config-write 4; raised to 5 when Force=true so plans show
// the higher-risk override.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       4,
	}
	if step == nil || step.FileCopy == nil {
		return cost, nil
	}
	if step.FileCopy.Force {
		cost.Risk = 5
	}
	if info, err := os.Stat(step.FileCopy.Src); err == nil && info.Mode().IsRegular() {
		cost.Bytes = info.Size()
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
