package git_config

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for git.config (spec-26 phase 5).
//
// Risk=2 — git.config writes plain text to a config file. No
// network, no service restarts, no working-tree touch. One of the
// lowest-risk mutations in the action surface; safer than routine
// config writes (which can be 4–5 for /etc files) because git's
// config format is well-understood and the action limits itself to
// key/value edits.
//
// Resources = len(Set) + len(Unset). Each key is one resource —
// distinct from git.clone's "one repo per step" because each
// key/value pair is independently auditable.
//
// Bytes=-1 (negligible; not worth predicting).
//
// Reversible=true — the Reverser interface is implemented (returns
// a "needs apply-time pre-state capture" refusal for v1; tracked
// as a follow-up). Same convention as git.checkout / os.service.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  0,
		Bytes:      -1,
		Reversible: true,
		Risk:       2,
	}
	if step == nil || step.GitConfig == nil {
		return cost, nil
	}
	cost.Resources = len(step.GitConfig.Set) + len(step.GitConfig.Unset)
	if cost.Resources == 0 {
		// Degenerate step shape — Validate rejects this; report 1
		// so consumers don't see a misleading zero.
		cost.Resources = 1
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
