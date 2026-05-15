package git_checkout

import (
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Cost implements actions.Coster for git.checkout (spec-26 phase 5).
//
// Risk=3 — git.checkout switches an existing working tree to a new
// ref. No network, no new repos. Lower than git.clone (4) because
// there's no download phase; higher than git.config (2) because
// switching refs rewrites working-tree files and the index.
//
// Resources=1 — one repo dest. Bytes=-1 (we don't predict working-
// tree delta size).
//
// Reversible=true — the Reverser interface is implemented (returns a
// "needs apply-time HEAD capture" refusal for v1, tracked as a
// follow-up). Mirrors the os.service / file.unarchive convention:
// implementing the interface signals "designed to be reversible";
// the transaction layer probes Reverse() at plan time and surfaces
// the runtime refusal.
func (Handler) Cost(_ actions.Context, step *config.Step) (actions.CostEstimate, error) {
	cost := actions.CostEstimate{
		Resources:  1,
		Bytes:      -1,
		Reversible: true,
		Risk:       3,
	}
	if step == nil || step.GitCheckout == nil {
		return cost, nil
	}
	return cost, nil
}

var _ actions.Coster = (*Handler)(nil)
