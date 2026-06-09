package windows_firewall_rule

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// WindowsFirewallRuleReverseInfo is stashed on Result.ReverseData at
// apply time. It captures enough of the pre-apply state so Reverse()
// can emit the inverse step without re-querying the live system.
type WindowsFirewallRuleReverseInfo struct {
	// AppliedState is the step's normalised state: "present" or "absent".
	AppliedState string

	// PriorExisted is true when a rule with the same name existed before
	// apply. False means the apply was a create (no prior state to restore).
	PriorExisted bool

	// PriorRule is the rule configuration before apply. Nil when
	// PriorExisted is false.
	PriorRule *WindowsFirewallRuleSnapshot
}

// WindowsFirewallRuleSnapshot captures the observable fields of a live
// rule as lowercased strings, matching the observedRule shape.
type WindowsFirewallRuleSnapshot struct {
	Name        string
	Description string
	Direction   string
	Protocol    string
	LocalPorts  []string
	RemotePorts []string
	Action      string
	Profiles    []string
	Enabled     bool
}

// observedRuleToSnapshot converts an in-memory observedRule to the
// exported snapshot shape stored in ReverseData.
func observedRuleToSnapshot(o *observedRule) *WindowsFirewallRuleSnapshot {
	if o == nil {
		return nil
	}
	return &WindowsFirewallRuleSnapshot{
		Name:        o.DisplayName,
		Description: o.Description,
		Direction:   o.Direction,
		Protocol:    o.Protocol,
		LocalPorts:  append([]string(nil), o.LocalPorts...),
		RemotePorts: append([]string(nil), o.RemotePorts...),
		Action:      o.Action,
		Profiles:    append([]string(nil), o.Profiles...),
		Enabled:     o.Enabled,
	}
}

// snapshotToStep constructs a state=present windows.firewall_rule step
// from a snapshot, used for both update-rollback and delete-rollback.
func snapshotToStep(stepName string, s *WindowsFirewallRuleSnapshot) *config.Step {
	enabled := s.Enabled
	return &config.Step{
		Name: stepName,
		WindowsFirewallRule: &config.WindowsFirewallRule{
			Name:        s.Name,
			State:       statePresent,
			Description: s.Description,
			Direction:   s.Direction,
			Protocol:    s.Protocol,
			LocalPort:   append([]string(nil), s.LocalPorts...),
			RemotePort:  append([]string(nil), s.RemotePorts...),
			Action:      s.Action,
			Profile:     append([]string(nil), s.Profiles...),
			Enabled:     &enabled,
		},
	}
}

// Reverse implements actions.Reverser for windows.firewall_rule.
//
// Inverse-state strategy:
//   - AppliedState=present, PriorExisted=false (create) → state=absent step.
//   - AppliedState=present, PriorExisted=true  (update) → state=present step
//     with the pre-update rule config.
//   - AppliedState=absent                      (delete) → state=present step
//     with the deleted rule config.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.WindowsFirewallRule == nil {
		return nil, errors.New("windows.firewall_rule Reverse: step has no WindowsFirewallRule payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("windows.firewall_rule Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}

	info, ok := r.ReverseData.(*WindowsFirewallRuleReverseInfo)
	if !ok {
		return nil, fmt.Errorf("windows.firewall_rule Reverse: ReverseData is %T, want *WindowsFirewallRuleReverseInfo", r.ReverseData)
	}

	ruleName := step.WindowsFirewallRule.Name

	switch info.AppliedState {
	case statePresent:
		if !info.PriorExisted {
			// Apply created the rule → delete it.
			return &config.Step{
				Name: "reverse: remove windows.firewall_rule " + ruleName,
				WindowsFirewallRule: &config.WindowsFirewallRule{
					Name:  ruleName,
					State: stateAbsent,
				},
			}, nil
		}
		// Apply updated an existing rule → restore prior config.
		if info.PriorRule == nil {
			return nil, fmt.Errorf("windows.firewall_rule Reverse: PriorExisted=true but PriorRule is nil for %q", ruleName)
		}
		return snapshotToStep("reverse: restore windows.firewall_rule "+ruleName, info.PriorRule), nil

	case stateAbsent:
		// Apply deleted the rule → restore it.
		if info.PriorRule == nil {
			return nil, fmt.Errorf("windows.firewall_rule Reverse: AppliedState=absent but PriorRule is nil for %q", ruleName)
		}
		return snapshotToStep("reverse: restore windows.firewall_rule "+ruleName, info.PriorRule), nil
	}

	return nil, fmt.Errorf("windows.firewall_rule Reverse: unexpected AppliedState %q", info.AppliedState)
}

var _ actions.Reverser = (*Handler)(nil)
