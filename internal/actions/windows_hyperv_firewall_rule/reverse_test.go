package windows_hyperv_firewall_rule

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const testVMID = "{40E0AC32-46A5-843A-BF4E-090FE0F48888}"

func makeHVResult(info *WindowsHyperVFirewallRuleReverseInfo) *executor.Result {
	r := executor.NewResult()
	if info != nil {
		r.ReverseData = info
	}
	return r
}

func baseHVStep(name string) *config.Step {
	return &config.Step{
		Name: name,
		WindowsHyperVFirewallRule: &config.WindowsHyperVFirewallRule{
			Name:        name,
			State:       statePresent,
			VMCreatorID: testVMID,
		},
	}
}

func TestHVFirewallReverse_NilStep(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, nil, makeHVResult(nil))
	if err == nil {
		t.Fatal("expected error for nil step")
	}
}

func TestHVFirewallReverse_NilResult(t *testing.T) {
	_, err := (&Handler{}).Reverse(nil, baseHVStep("X"), nil)
	if err == nil {
		t.Fatal("expected error for nil result")
	}
}

func TestHVFirewallReverse_WrongReverseDataType(t *testing.T) {
	r := executor.NewResult()
	r.ReverseData = "not-the-right-type"
	_, err := (&Handler{}).Reverse(nil, baseHVStep("X"), r)
	if err == nil {
		t.Fatal("expected error for wrong ReverseData type")
	}
}

func TestHVFirewallReverse_NilReverseData_Noop(t *testing.T) {
	step, err := (&Handler{}).Reverse(nil, baseHVStep("X"), makeHVResult(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step != nil {
		t.Fatalf("expected nil step for noop; got %+v", step)
	}
}

func TestHVFirewallReverse_PresentCreate_ReturnsAbsent(t *testing.T) {
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        statePresent,
		PriorExisted:        false,
		ResolvedVMCreatorID: testVMID,
	}
	step, err := (&Handler{}).Reverse(nil, baseHVStep("HVRule"), makeHVResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if step == nil || step.WindowsHyperVFirewallRule == nil {
		t.Fatal("expected a WindowsHyperVFirewallRule step")
	}
	f := step.WindowsHyperVFirewallRule
	if f.State != stateAbsent {
		t.Errorf("state = %q; want %q", f.State, stateAbsent)
	}
	if f.VMCreatorID != testVMID {
		t.Errorf("vm_creator_id = %q; want %q (resolved GUID)", f.VMCreatorID, testVMID)
	}
}

func TestHVFirewallReverse_PresentUpdate_RestoresPrior(t *testing.T) {
	prior := &WindowsHyperVFirewallRuleSnapshot{
		Name:        "HVRule",
		Direction:   "inbound",
		Protocol:    "tcp",
		LocalPorts:  []string{"8080"},
		Action:      "allow",
		Enabled:     true,
		VMCreatorID: testVMID,
	}
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        statePresent,
		PriorExisted:        true,
		ResolvedVMCreatorID: testVMID,
		PriorRule:           prior,
	}
	step, err := (&Handler{}).Reverse(nil, baseHVStep("HVRule"), makeHVResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := step.WindowsHyperVFirewallRule
	if f.State != statePresent {
		t.Errorf("state = %q; want present", f.State)
	}
	if f.VMCreatorID != testVMID {
		t.Errorf("vm_creator_id = %q; want resolved GUID %q", f.VMCreatorID, testVMID)
	}
	if len(f.LocalPort) != 1 || f.LocalPort[0] != "8080" {
		t.Errorf("local_port = %v; want [8080]", f.LocalPort)
	}
	if !strings.Contains(step.Name, "restore") {
		t.Errorf("step name should mention 'restore'; got %q", step.Name)
	}
}

func TestHVFirewallReverse_PresentUpdate_NilPriorRule_Error(t *testing.T) {
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        statePresent,
		PriorExisted:        true,
		ResolvedVMCreatorID: testVMID,
		PriorRule:           nil,
	}
	_, err := (&Handler{}).Reverse(nil, baseHVStep("HVRule"), makeHVResult(info))
	if err == nil {
		t.Fatal("expected error when PriorExisted=true but PriorRule is nil")
	}
}

func TestHVFirewallReverse_AbsentDelete_RestoresPrior(t *testing.T) {
	prior := &WindowsHyperVFirewallRuleSnapshot{
		Name:        "HVRule",
		Protocol:    "udp",
		RemotePorts: []string{"53"},
		Action:      "block",
		Enabled:     false,
		VMCreatorID: testVMID,
	}
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        stateAbsent,
		PriorExisted:        true,
		ResolvedVMCreatorID: testVMID,
		PriorRule:           prior,
	}
	step, err := (&Handler{}).Reverse(nil, baseHVStep("HVRule"), makeHVResult(info))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	f := step.WindowsHyperVFirewallRule
	if f.State != statePresent {
		t.Errorf("state = %q; want present", f.State)
	}
	if f.Protocol != "udp" {
		t.Errorf("protocol = %q; want udp", f.Protocol)
	}
}

func TestHVFirewallReverse_AbsentDelete_NilPriorRule_Error(t *testing.T) {
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        stateAbsent,
		PriorExisted:        true,
		ResolvedVMCreatorID: testVMID,
		PriorRule:           nil,
	}
	_, err := (&Handler{}).Reverse(nil, baseHVStep("HVRule"), makeHVResult(info))
	if err == nil {
		t.Fatal("expected error when AppliedState=absent but PriorRule is nil")
	}
}

func TestHVFirewallReverse_UnknownState_Error(t *testing.T) {
	info := &WindowsHyperVFirewallRuleReverseInfo{
		AppliedState:        "bogus",
		ResolvedVMCreatorID: testVMID,
	}
	_, err := (&Handler{}).Reverse(nil, baseHVStep("X"), makeHVResult(info))
	if err == nil {
		t.Fatal("expected error for unknown state")
	}
}
