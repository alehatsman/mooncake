package service

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.service (spec-22 phase 6).
//
// Risk bands track service-state semantics:
//   - state=stopped: 7 — taking a running service offline is
//     user-visible and a frequent incident root cause
//   - state=restarted: 7 — brief downtime
//   - state=reloaded: 5 — graceful, no downtime expected
//   - state=started or no state: 5 — additive
//
// Reversible reflects the interface assertion even though
// os.service's current Reverse refuses (slice F follow-up — see
// service/reverse.go). The cost estimate truthfully reports
// "interface implemented" so cost reporting stays consistent; the
// transaction layer surfaces the runtime refusal at plan time.
func (h *Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{Resources: 1, Bytes: -1, Reversible: true, Risk: 5}
	if step == nil || step.OsService == nil {
		return cost, nil
	}
	switch step.OsService.State {
	case ServiceStateStopped, ServiceStateRestarted:
		cost.Risk = 7
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
