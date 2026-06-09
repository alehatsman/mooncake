package os_cron //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.cron (spec-22 phase 6 /
// spec-28 P6).
//
// Risk=4 — adding/removing a cron entry writes a single file in
// /etc/cron.d. Routine config write. The downstream blast radius
// (commands the cron entry runs) is an attribute of that command,
// not the action itself.
//
// Reversible=true — Reverser implemented (reverse.go).
// apply-time capture).
func (Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       4,
	}, nil
}

var _ actions.Coster = (*Handler)(nil)
