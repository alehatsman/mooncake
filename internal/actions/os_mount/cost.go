package os_mount //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.mount (spec-22 phase 6 /
// spec-28 P6).
//
// Risk=7 — mount changes affect data access. Wrong mount options
// (e.g. ro vs rw, noexec, nosuid) can break running services; a
// missing destination directory leaves the fstab entry unusable;
// unmounting a busy filesystem can fail dramatically. Above
// routine config because the failure modes touch data path, not
// just the control plane.
//
// Risk=8 when state=absent (the fstab entry is removed; if the
// volume isn't also unmounted, the next reboot won't restore it).
//
// Reversible=true — Reverser interface declared (refusal pending
// pre-state capture).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       7,
	}
	if step == nil || step.OsMount == nil {
		return cost, nil
	}
	if step.OsMount.State == "absent" {
		cost.Risk = 8
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
