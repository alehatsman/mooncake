package os_firewall //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.firewall (spec-22 phase 6 /
// spec-28 P6).
//
// Risk=7 — firewall changes immediately affect network
// reachability. The classic failure mode is locking out the
// operator's own ssh connection. Above routine config (5-6)
// because the blast radius is "every inbound packet from now on";
// below destructive (8+) because the action is bounded to the
// declared rules and can be inverted.
//
// Resources counts rules. Bytes=-1.
//
// Reversible=true — Reverser interface declared (refusal pending
// apply-time capture of the prior rule set).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       7,
	}
	if step == nil || step.OsFirewall == nil {
		return cost, nil
	}
	count := len(step.OsFirewall.Rules)
	if step.OsFirewall.Rule != nil {
		count++
	}
	if count > 0 {
		cost.Resources = count
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
