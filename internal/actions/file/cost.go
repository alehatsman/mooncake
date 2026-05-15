package file

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for file.write (spec-22 phase 6).
// Risk varies by state — deletes are the highest-risk variant since
// they're the hardest to roll back even with content snapshot
// (slice B caps at 4 MiB, large pre-deletes refuse reverse). Other
// states bucket as routine config writes.
//
// Resources is always 1 (single path target). Bytes reflects the
// declared Content for state=file when present — accurate for the
// common "agent writes a config file" path; -1 for path-only states
// (touch, perms, link, hardlink, absent, directory) where there's
// no content payload to measure.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       4,
	}
	if step == nil || step.FileWrite == nil {
		return cost, nil
	}
	switch step.FileWrite.State {
	case "absent":
		cost.Risk = 8 // hardest to reverse — content cap can refuse
	case "directory", "touch", "perms", stateLink, stateHardlink:
		cost.Risk = 4
	default:
		// state="" or "file" — content write.
		if step.FileWrite.Content != "" {
			cost.Bytes = int64(len(step.FileWrite.Content))
		}
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
