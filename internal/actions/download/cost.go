package download

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for file.download (spec-22 phase 6).
// Resources is 1 (single dest). Bytes is -1 — the remote size isn't
// known without an HTTP HEAD which we intentionally don't issue from
// Cost (side-effect-free). Risk is 5: network involvement raises
// it one band above routine writes because transient HTTP errors are
// a real failure mode.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       5,
	}
	if step == nil || step.FileDownload == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
