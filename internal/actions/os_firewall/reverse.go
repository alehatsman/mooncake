package os_firewall //nolint:revive // package name follows action convention

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// OsFirewallReverseInfo is the per-step apply-time snapshot
// os.firewall stashes on Result.ReverseData. By computePlan's
// design, only one of AddedRules / RemovedRules is non-empty per
// apply (state=present only adds, state=absent only removes), so
// the reverse is always single-direction.
type OsFirewallReverseInfo struct {
	// Backend identifies which firewall backend ran the apply (v1:
	// always "ufw"). Preserved so a future multi-backend rollout
	// reverses against the same backend.
	Backend string

	// AppliedState is the step's normalized state at apply time:
	// "present" or "absent".
	AppliedState string

	// AddedRules is the list of rules the apply actually added
	// (state=present path). Empty when state=absent.
	AddedRules []FirewallRuleSnapshot

	// RemovedRules is the list of rules the apply actually removed
	// (state=absent path). Empty when state=present.
	RemovedRules []FirewallRuleSnapshot
}

// FirewallRuleSnapshot is the exported view of a captured rule.
// Field names match config.FirewallRule so reverse step
// construction is one-to-one.
type FirewallRuleSnapshot struct {
	Port     int
	Protocol string
	Action   string
	From     string
	Comment  string
}

// ruleSliceToSnapshot lifts the package-private rule slice to the
// exported FirewallRuleSnapshot shape. Returns nil for nil/empty
// input so Reverse can distinguish "nothing to reverse" cleanly.
func ruleSliceToSnapshot(in []rule) []FirewallRuleSnapshot {
	if len(in) == 0 {
		return nil
	}
	out := make([]FirewallRuleSnapshot, len(in))
	for i, r := range in {
		// rule and FirewallRuleSnapshot share field shape; Go's
		// struct conversion lets us skip the field-by-field copy.
		out[i] = FirewallRuleSnapshot(r)
	}
	return out
}

// Reverse implements actions.Reverser for os.firewall (spec-28 P6 /
// reverse-capture v5).
//
// Inverse-state strategy:
//   - AppliedState=present (apply added rules) → reverse step
//     state=absent with the AddedRules list.
//   - AppliedState=absent  (apply removed rules) → reverse step
//     state=present with the RemovedRules list.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Both AddedRules + RemovedRules empty → noop, same.
//   - Step / result missing / wrong type → defensive error.
func (Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.OsFirewall == nil {
		return nil, errors.New("os.firewall Reverse: step has no OsFirewall payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("os.firewall Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}
	info, ok := r.ReverseData.(*OsFirewallReverseInfo)
	if !ok {
		return nil, fmt.Errorf("os.firewall Reverse: ReverseData is %T, want *OsFirewallReverseInfo", r.ReverseData)
	}
	if len(info.AddedRules) == 0 && len(info.RemovedRules) == 0 {
		return nil, nil
	}

	var rules []FirewallRuleSnapshot
	inverseState := "absent"
	switch {
	case len(info.AddedRules) > 0:
		rules = info.AddedRules
		inverseState = "absent"
	case len(info.RemovedRules) > 0:
		rules = info.RemovedRules
		inverseState = "present"
	}

	confRules := make([]config.FirewallRule, len(rules))
	for i, r := range rules {
		confRules[i] = config.FirewallRule{
			Port:     r.Port,
			Protocol: r.Protocol,
			Action:   r.Action,
			From:     r.From,
			Comment:  r.Comment,
		}
	}

	return &config.Step{
		Name: "reverse: os.firewall " + inverseState,
		OsFirewall: &config.OsFirewall{
			Backend: info.Backend,
			State:   inverseState,
			Rules:   confRules,
		},
	}, nil
}

var _ actions.Reverser = (*Handler)(nil)
