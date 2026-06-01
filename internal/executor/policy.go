package executor

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// defaultPolicyRisk mirrors the actions package Coster default
// (defaultCoster.Cost → Risk: 5 in internal/actions/registry_abi.go).
// A step whose handler implements no Coster is treated as this band
// when MaxRisk is enforced, so the risk cap behaves the same way the
// rest of the kernel treats an unscored action.
const defaultPolicyRisk = 5

// Policy is a per-run allow/deny contract enforced at executor
// preflight, before any step's side effects run. It is the
// "permissions as contract" keystone (#11): the actor spawning a run —
// an operator, or moongit launching an unattended agent — declares what
// the run may do, and the executor refuses any step that exceeds it.
// This is what lets a shell-less agent run be *safe* rather than merely
// structured: it replaces the host permission wall the caller gives up
// when execution moves into the kernel.
//
// Scope is deliberately a flat struct — action allow/deny lists, a
// network switch, and a risk cap. It is NOT an expression language:
// docs-working/vision/non_goals.md forbids an expressive policy DSL
// (the OPA/Rego sprawl trap). The richer `deny: agent.touches(...)`
// framing belongs to the agent-safety spec, not here.
//
// A nil *Policy (and the zero value) enforces nothing: every step is
// allowed. Callers opt in by populating fields, so every existing run
// path — CLI apply, fleet, the pilot loop, tests — is unchanged when no
// policy is set.
//
// Gating draws only on the spec-22 ABI a handler already declares
// (PermissionSet and Cost.Risk — see internal/actions/handler_abi.go),
// so no handler needs to change for a run to be policy-gated.
type Policy struct {
	// AllowedActions, when non-empty, is an allowlist: a step's action
	// type must appear here or the step is denied. Empty disables the
	// allowlist (any action is permitted unless denied below). Entries
	// are action types as config.Step.DetermineActionType reports them —
	// e.g. "shell", "file.write", "pkg".
	//
	// The json tags make Policy wire-ready: agentd's POST /v1/runs and
	// the MCP run_plan `policy` arg both deserialize straight into it.
	AllowedActions []string `json:"allowed_actions,omitempty"`

	// DeniedActions is a denylist applied after the allowlist; an action
	// type listed here is always denied (denylist wins over allowlist).
	// This is the primary lever for unattended agent runs:
	// DeniedActions ["shell", "cmd"] removes the arbitrary-command escape
	// hatch while leaving the typed actions usable.
	DeniedActions []string `json:"denied_actions,omitempty"`

	// DenyNetwork rejects any step whose declared PermissionSet has
	// Network: true (package installs, downloads, http.request, remote
	// git clone). Handlers that declare no PermissionSet are treated as
	// Network:false and pass — gate those by action name if needed.
	DenyNetwork bool `json:"deny_network,omitempty"`

	// MaxRisk, when > 0, caps the Cost.Risk band (1..10) a step may
	// carry; a step whose estimated risk exceeds it is denied. Handlers
	// without a Coster default to defaultPolicyRisk. 0 = no cap.
	MaxRisk int `json:"max_risk,omitempty"`
}

// IsZero reports whether the policy enforces nothing. A nil *Policy is
// also "enforces nothing"; dispatch guards on `policy != nil` before
// calling check, so this is mainly for tests and callers that want to
// skip wiring an empty policy.
func (p *Policy) IsZero() bool {
	return p == nil ||
		(len(p.AllowedActions) == 0 &&
			len(p.DeniedActions) == 0 &&
			!p.DenyNetwork &&
			p.MaxRisk <= 0)
}

// check enforces the policy against a single step that is about to be
// dispatched. perms is the step's resolved PermissionSet (empty when
// the handler declares none); risk is its resolved Cost.Risk band
// (defaultPolicyRisk when the handler implements no Coster). A non-nil
// error means the step is denied and the run must stop before that
// step's side effects.
//
// Order is denylist → allowlist → network → risk: most-explicit
// operator intent first. The first violation wins, and the error names
// the rule and the action so an agent loop can feed it back to the
// model verbatim (the next iteration then proposes a permitted step).
func (p *Policy) check(step *config.Step, perms actions.PermissionSet, risk int) error {
	if p == nil {
		return nil
	}
	action := step.DetermineActionType()

	if containsFold(p.DeniedActions, action) {
		return fmt.Errorf(
			"policy denied step %q: action %q is on the denylist",
			stepLabel(step), action,
		)
	}

	if len(p.AllowedActions) > 0 && !containsFold(p.AllowedActions, action) {
		return fmt.Errorf(
			"policy denied step %q: action %q is not in the allowlist %v",
			stepLabel(step), action, p.AllowedActions,
		)
	}

	if p.DenyNetwork && perms.Network {
		return fmt.Errorf(
			"policy denied step %q: action %q requires network egress but the run policy denies network",
			stepLabel(step), action,
		)
	}

	if p.MaxRisk > 0 && risk > p.MaxRisk {
		return fmt.Errorf(
			"policy denied step %q: action %q has risk %d which exceeds the run policy max risk %d",
			stepLabel(step), action, risk, p.MaxRisk,
		)
	}

	return nil
}

// containsFold reports whether want is in list, case-insensitively and
// ignoring surrounding whitespace. Action types are lower-case by
// convention ("file.write"), but a hand-written policy might use any
// case; fold-matching avoids a silent allow/deny miss from a casing
// typo turning into an unintended permit.
func containsFold(list []string, want string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), want) {
			return true
		}
	}
	return false
}

// enforcePolicy applies the per-run Policy gate to a step about to be
// dispatched. It is a no-op when no policy is set (the common path), so
// callers can invoke it unconditionally. perms is the step's already-
// resolved PermissionSet; risk is resolved from the handler's Coster
// (defaultPolicyRisk fallback) only when a MaxRisk cap is in effect, so
// the no-cap path skips the extra handler call.
//
// Extracted from dispatchRunner so the hot dispatch path stays under
// the gocyclo budget as the policy surface grows.
func enforcePolicy(ec *ExecutionContext, step *config.Step, runner actions.Runner, perms actions.PermissionSet) error {
	if ec.Svc == nil || ec.Svc.Policy == nil {
		return nil
	}
	risk := defaultPolicyRisk
	if ec.Svc.Policy.MaxRisk > 0 {
		if c, ok := runner.(actions.Coster); ok {
			if est, costErr := c.Cost(ec, step); costErr == nil {
				risk = est.Risk
			}
		}
	}
	return ec.Svc.Policy.check(step, perms, risk)
}
