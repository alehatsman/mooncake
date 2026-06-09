package os_user //nolint:revive // package name follows action convention

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for os.user (spec-22 phase 6 /
// spec-27 P4).
//
// Risk bands track the destructiveness of each variant:
//   - state=present, create new user: 5 — additive, but file ownership
//     of new home content + uid assignment carry long-tail
//     consequences for backup/sync tools.
//   - state=present, modify existing user (uid/gid/shell change): 6 —
//     attribute drift on an existing identity. Cron jobs, sudoers
//     entries, file-ownership cascades all key on uid; the kernel
//     handler refuses uid renumbering, but Cost doesn't know that.
//   - state=absent: 8 — removing a user is destructive. /etc/passwd
//     entry gone; any process / file owned by that uid is orphaned.
//
// Resources=1 (the named user). Bytes=-1.
//
// Reversible=true — Reverser implemented (reverse.go).
// v1, pending apply-time getent snapshot). The natural inverse —
// recreate the user with the captured pre-apply attributes (uid,
// shell, groups, etc.) — is a clean reverse once the capture
// pipeline lands.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       5,
	}
	if step == nil || step.OsUser == nil {
		return cost, nil
	}
	if step.OsUser.State == "absent" {
		cost.Risk = 8
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
