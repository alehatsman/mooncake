package os_sysctl //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.sysctl (spec-22 phase 6 /
// spec-28 P6).
//
// Risk=6 — sysctl writes change live kernel parameters. Wrong
// values can break networking (rp_filter, conntrack), virtual
// memory (vm.overcommit), or core dumps. Above routine config (4)
// because the failure modes are immediate and runtime-visible;
// below destructive (7+) because the action limits itself to one
// key per step and is easily inverted by re-setting the prior
// value.
//
// Reversible=true — Reverser interface declared (refusal pending
// apply-time capture of prior runtime value + file entry).
func (Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       6,
	}, nil
}

var _ actions.Coster = (*Handler)(nil)
