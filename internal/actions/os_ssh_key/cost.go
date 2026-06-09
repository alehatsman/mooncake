package os_ssh_key //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.ssh_key (spec-22 phase 6 /
// spec-27 P4).
//
// Risk=5 — adding a public key to authorized_keys grants login as
// the target user; removing one revokes access. Both are sensitive
// but bounded (one user account, one file). Higher than routine
// config writes because the access-control surface (who can ssh in
// as whom) is the canonical security perimeter on a Linux box. Not
// raised further because the action is per-user and one file, not
// global.
//
// Exclusive=true bumps risk to 6 — wholesale replacement of the
// file is more disruptive (revokes any keys not in our supplied
// list, possibly locking out other tooling).
//
// Resources counts targeted keys. Bytes=-1.
//
// Reversible=true — Reverser implemented (reverse.go).
// apply-time capture).
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       5,
	}
	if step == nil || step.OsSSHKey == nil {
		return cost, nil
	}
	k := step.OsSSHKey
	count := len(k.Keys)
	if k.Key != "" {
		count++
	}
	if count > 0 {
		cost.Resources = count
	}
	if k.Exclusive {
		cost.Risk = 6
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
