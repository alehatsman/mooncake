package agent

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/executor"
)

// renderPolicyContract renders the PERMISSIONS CONTRACT block of the
// system prompt from the run's executor.Policy. This is the
// contract-visibility half of the "permissions as contract" keystone
// (#11): the executor enforces the same Policy at preflight, but unless
// the planner is *told* the contract it will keep proposing steps the
// gate refuses — every refused step wastes an iteration (the model only
// learns the rule from the next prompt's LAST ITERATION error block).
// Surfacing the contract up front lets the model plan within it from
// iteration 1.
//
// Returns "" for a nil/zero policy so an unrestricted run's prompt stays
// byte-identical to the pre-policy behavior (mirrors buildAgentPolicy's
// nil-on-no-flags contract in cmd/kernel/agent.go). The wording is kept
// deliberately close to executor.Policy.check's refusal messages so a
// model that reads the contract and a model that reads a refusal see the
// same vocabulary.
func renderPolicyContract(p *executor.Policy) string {
	if p.IsZero() {
		return ""
	}

	var b strings.Builder
	b.WriteString("PERMISSIONS CONTRACT (enforced — any step that violates this is REFUSED before it runs, wasting the iteration):\n")

	if len(p.AllowedActions) > 0 {
		fmt.Fprintf(&b, "- You may ONLY use these action types: %s\n", strings.Join(p.AllowedActions, ", "))
	}
	if len(p.DeniedActions) > 0 {
		fmt.Fprintf(&b, "- You may NOT use these action types: %s\n", strings.Join(p.DeniedActions, ", "))
	}
	if p.DenyNetwork {
		b.WriteString("- Network egress is FORBIDDEN: do not propose steps that download files, install packages, make remote requests, or clone from remote hosts.\n")
	}
	if p.MaxRisk > 0 {
		fmt.Fprintf(&b, "- Maximum risk band is %d (1..10): do not propose higher-risk/destructive steps.\n", p.MaxRisk)
	}

	b.WriteString("Plan strictly within this contract. A step that breaks it does not run — it stops the iteration with a policy-denied error.")
	return b.String()
}
