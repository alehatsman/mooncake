package windows_firewall_rule

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func makeFirewallResult(info *WindowsFirewallRuleReverseInfo) *executor.Result {
	r := executor.NewResult()
	if info != nil {
		r.ReverseData = info
	}
	return r
}

func baseFirewallStep(name string) *config.Step {
	return &config.Step{
		Name: name,
		WindowsFirewallRule: &config.WindowsFirewallRule{
			Name:  name,
			State: statePresent,
		},
	}
}

func TestFirewallReverse_NilStep(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, nil, makeFirewallResult(nil))
	if err == nil {
		t.Fatal("expected error for nil step")
	}
}

func TestFirewallReverse_NilResult(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, baseFirewallStep("X"), nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestFirewallReverse_WrongReverseDataType(t *testing.T) {
	r := executor.NewResult()
	r.ReverseData = "not-the-right-type"
	_, err := (&Handler{}).Reverse(nil, baseFirewallStep("X"), r)
	if err == nil {
		t.Fatal("expected error for wrong ReverseData type")
	}
}

func TestFirewallReverse_NilReverseData_Noop(t *testing.T) {
	step, err := (&Handler{}).Reverse(nil, baseFirewallStep("X"), makeFirewallResult(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step != nil {
		t.Fatalf("expected nil step for noop; got %+v", step)
	}
}

func TestFirewallReverse_PresentCreate_ReturnsAbsent(t *testing.T) {
	info := &WindowsFirewallRuleReverseInfo{
		AppliedState: statePresent,
		PriorExisted: false,
		PriorRule:    nil,
	}
	step, err := (&Handler{}).Reverse(nil, baseFirewallStep("MyRule"), makeFirewallResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step == nil || step.WindowsFirewallRule == nil {
		t.Fatal("expected a WindowsFirewallRule step")
	}
	if step.WindowsFirewallRule.State != stateAbsent {
		t.Errorf("state = %q; want %q", step.WindowsFirewallRule.State, stateAbsent)
	}
	if step.WindowsFirewallRule.Name != "MyRule" {
		t.Errorf("name = %q; want MyRule", step.WindowsFirewallRule.Name)
	}
}

func TestFirewallReverse_PresentUpdate_RestoresPrior(t *testing.T) {
	prior := &WindowsFirewallRuleSnapshot{
		Name:        "MyRule",
		Description: "before",
		Direction:   "inbound",
		Protocol:    "tcp",
		LocalPorts:  []string{"80"},
		Action:      "allow",
		Profiles:    []string{"domain"},
		Enabled:     true,
	}
	info := &WindowsFirewallRuleReverseInfo{
		AppliedState: statePresent,
		PriorExisted: true,
		PriorRule:    prior,
	}
	step, err := (&Handler{}).Reverse(nil, baseFirewallStep("MyRule"), makeFirewallResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := step.WindowsFirewallRule
	if f.State != statePresent {
		t.Errorf("state = %q; want present", f.State)
	}
	if f.Description != "before" {
		t.Errorf("description = %q; want 'before'", f.Description)
	}
	if len(f.LocalPort) != 1 || f.LocalPort[0] != "80" {
		t.Errorf("local_port = %v; want [80]", f.LocalPort)
	}
	if !strings.Contains(step.Name, "restore") {
		t.Errorf("step name should mention 'restore'; got %q", step.Name)
	}
}

func TestFirewallReverse_PresentUpdate_NilPriorRule_Error(t *testing.T) {
	info := &WindowsFirewallRuleReverseInfo{
		AppliedState: statePresent,
		PriorExisted: true,
		PriorRule:    nil,
	}
	_, err := (&Handler{}).Reverse(nil, baseFirewallStep("MyRule"), makeFirewallResult(info))
	if err == nil {
		t.Fatal("expected error when PriorExisted=true but PriorRule is nil")
	}
}

func TestFirewallReverse_AbsentDelete_RestoresPrior(t *testing.T) {
	prior := &WindowsFirewallRuleSnapshot{
		Name:       "MyRule",
		Direction:  "outbound",
		Protocol:   "udp",
		LocalPorts: []string{"443"},
		Action:     "block",
		Enabled:    false,
	}
	info := &WindowsFirewallRuleReverseInfo{
		AppliedState: stateAbsent,
		PriorExisted: true,
		PriorRule:    prior,
	}
	step, err := (&Handler{}).Reverse(nil, baseFirewallStep("MyRule"), makeFirewallResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := step.WindowsFirewallRule
	if f.State != statePresent {
		t.Errorf("state = %q; want present", f.State)
	}
	if f.Protocol != "udp" {
		t.Errorf("protocol = %q; want udp", f.Protocol)
	}
}

func TestFirewallReverse_AbsentDelete_NilPriorRule_Error(t *testing.T) {
	info := &WindowsFirewallRuleReverseInfo{
		AppliedState: stateAbsent,
		PriorExisted: true,
		PriorRule:    nil,
	}
	_, err := (&Handler{}).Reverse(nil, baseFirewallStep("MyRule"), makeFirewallResult(info))
	if err == nil {
		t.Fatal("expected error when AppliedState=absent but PriorRule is nil")
	}
}

func TestFirewallReverse_UnknownState_Error(t *testing.T) {
	info := &WindowsFirewallRuleReverseInfo{AppliedState: "bogus"}
	_, err := (&Handler{}).Reverse(nil, baseFirewallStep("X"), makeFirewallResult(info))
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}
