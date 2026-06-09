package windows_hyperv_firewall_rule

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// WindowsHyperVFirewallRuleReverseInfo is stashed on Result.ReverseData
// at apply time. It captures enough pre-apply state so Reverse() can
// emit the inverse step without re-querying the live system.
type WindowsHyperVFirewallRuleReverseInfo struct {
	// AppliedState is the step's normalised state: "present" or "absent".
	AppliedState string

	// PriorExisted is true when a rule existed before apply.
	PriorExisted bool

	// ResolvedVMCreatorID is the concrete GUID used during apply.
	// Stored here so the reverse step can reference the same VM scope
	// without re-running the WSL auto-discovery (which may not be
	// deterministic on a partially-configured box during rollback).
	ResolvedVMCreatorID string

	// PriorRule is the pre-apply rule config. Nil when PriorExisted=false.
	PriorRule *WindowsHyperVFirewallRuleSnapshot
}

// WindowsHyperVFirewallRuleSnapshot captures the observable fields of a
// live Hyper-V rule as lowercased strings, matching the observedRule shape.
type WindowsHyperVFirewallRuleSnapshot struct {
	Name        string
	Description string
	VMCreatorID string
	Direction   string
	Protocol    string
	LocalPorts  []string
	RemotePorts []string
	Action      string
	Enabled     bool
}

func observedRuleToSnapshot(o *observedRule) *WindowsHyperVFirewallRuleSnapshot {
	if o == nil {
		return nil
	}
	return &WindowsHyperVFirewallRuleSnapshot{
		Name:        o.DisplayName,
		Description: o.Description,
		VMCreatorID: o.VMCreatorID,
		Direction:   o.Direction,
		Protocol:    o.Protocol,
		LocalPorts:  append([]string(nil), o.LocalPorts...),
		RemotePorts: append([]string(nil), o.RemotePorts...),
		Action:      o.Action,
		Enabled:     o.Enabled,
	}
}

func snapshotToStep(stepName string, s *WindowsHyperVFirewallRuleSnapshot) *config.Step {
	enabled := s.Enabled
	return &config.Step{
		Name: stepName,
		WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
			Name:        s.Name,
			State:       statePresent,
			Description: s.Description,
			VMCreatorID: s.VMCreatorID,
			Direction:   s.Direction,
			Protocol:    s.Protocol,
			LocalPort:   append([]string(nil), s.LocalPorts...),
			RemotePort:  append([]string(nil), s.RemotePorts...),
			Action:      s.Action,
			Enabled:     &enabled,
		},
	}
}

// Reverse implements actions.Reverser for windows.hyperv_firewall_rule.
//
// Inverse-state strategy mirrors windows.firewall_rule:
//   - AppliedState=present, PriorExisted=false (create) → state=absent step.
//   - AppliedState=present, PriorExisted=true  (update) → state=present step
//     with the pre-update rule config.
//   - AppliedState=absent                      (delete) → state=present step
//     with the deleted rule config.
//
// The reverse step uses the ResolvedVMCreatorID (the concrete GUID from
// apply time) rather than "auto" so rollback doesn't re-run WSL discovery.
//
// Edge cases:
//   - ReverseData nil → apply was a noop, return (nil, nil).
//   - Step / result missing / wrong type → defensive error.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.WindowsHyperVFirewallRule == nil {
		return nil, errors.New("windows.hyperv_firewall_rule Reverse: step has no WindowsHyperVFirewallRule payload")
	}

	r, ok := result.(*executor.Result)
	if !ok || r == nil {
		return nil, fmt.Errorf("windows.hyperv_firewall_rule Reverse: expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, nil
	}

	info, ok := r.ReverseData.(*WindowsHyperVFirewallRuleReverseInfo)
	if !ok {
		return nil, fmt.Errorf("windows.hyperv_firewall_rule Reverse: ReverseData is %T, want *WindowsHyperVFirewallRuleReverseInfo", r.ReverseData)
	}

	ruleName := step.WindowsHyperVFirewallRule.Name

	switch info.AppliedState {
	case statePresent:
		if !info.PriorExisted {
			// Apply created the rule → delete it.
			return &config.Step{
				Name: "reverse: remove windows.hyperv_firewall_rule " + ruleName,
				WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
					Name:        ruleName,
					State:       stateAbsent,
					VMCreatorID: info.ResolvedVMCreatorID,
				},
			}, nil
		}
		// Apply updated an existing rule → restore prior config.
		if info.PriorRule == nil {
			return nil, fmt.Errorf("windows.hyperv_firewall_rule Reverse: PriorExisted=true but PriorRule is nil for %q", ruleName)
		}
		snap := *info.PriorRule
		snap.VMCreatorID = info.ResolvedVMCreatorID
		return snapshotToStep("reverse: restore windows.hyperv_firewall_rule "+ruleName, &snap), nil

	case stateAbsent:
		// Apply deleted the rule → restore it.
		if info.PriorRule == nil {
			return nil, fmt.Errorf("windows.hyperv_firewall_rule Reverse: AppliedState=absent but PriorRule is nil for %q", ruleName)
		}
		snap := *info.PriorRule
		snap.VMCreatorID = info.ResolvedVMCreatorID
		return snapshotToStep("reverse: restore windows.hyperv_firewall_rule "+ruleName, &snap), nil
	}

	return nil, fmt.Errorf("windows.hyperv_firewall_rule Reverse: unexpected AppliedState %q", info.AppliedState)
}

var _ actions.Reverser = (*Handler)(nil)
