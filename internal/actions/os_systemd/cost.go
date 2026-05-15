package os_systemd //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.systemd (spec-22 phase 6 /
// spec-28 P6).
//
// Risk=6 baseline — managing a unit file + lifecycle. Above
// routine config (4) because daemon-reload and start/stop are
// user-visible. Below destructive (7+) because the action is
// scoped to one unit.
//
// Risk=7 when Started=true is requested explicitly (the unit will
// be brought up and any startup failure surfaces immediately) OR
// when the step transitions state to absent (removing a unit
// stops + disables; same blast radius as os.service stop).
//
// Reversible=true — Reverser interface declared (refusal pending
// apply-time capture of unit-file content + prior enabled/started
// state).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       6,
	}
	if step == nil || step.OsSystemd == nil {
		return cost, nil
	}
	s := step.OsSystemd
	if s.State == "absent" {
		cost.Risk = 7
		return cost, nil
	}
	if s.Started != nil && *s.Started {
		cost.Risk = 7
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
