package os_group //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.group (spec-22 phase 6 /
// spec-27 P4).
//
// Risk bands:
//   - state=present (create new): 4 — additive, low blast radius;
//     just an entry in /etc/group.
//   - state=absent: 7 — removing a group with members is refused
//     by the kernel handler, but the residual risk (orphaned file
//     ownership keyed by gid, cron jobs, sudoers entries) puts
//     this above routine config writes.
//
// Lower than os.user across the board: groups are simpler
// identities (no shadow file, no home directory).
//
// Resources=1, Bytes=-1.
//
// Reversible=true — Reverser interface declared (refusal pending
// pre-state capture).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       4,
	}
	if step == nil || step.OsGroup == nil {
		return cost, nil
	}
	if step.OsGroup.State == "absent" {
		cost.Risk = 7
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
