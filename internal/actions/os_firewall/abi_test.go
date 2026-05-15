package os_firewall //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsFirewall: &config.OsFirewall{Rule: &config.FirewallRule{Port: 22}}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "ufw" {
		t.Errorf("RequiredBinaries = %v, want [ufw]", ps.RequiredBinaries)
	}
	if ps.Network {
		t.Errorf("Network must be false (firewall config doesn't make outbound calls); got %+v", ps)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsFirewall: &config.OsFirewall{
		Rule: &config.FirewallRule{Port: 22, Protocol: "tcp", Action: "allow"},
	}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Attributes["kind"] != "os.firewall" {
		t.Errorf("Attributes[kind] = %s, want os.firewall", d.Resource.Attributes["kind"])
	}
	after := d.After.(*OsFirewallSnapshot)
	if after.RuleCount != 1 {
		t.Errorf("RuleCount = %d, want 1", after.RuleCount)
	}
}

func TestDiff_MultiRule(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsFirewall: &config.OsFirewall{
		Rules: []config.FirewallRule{{Port: 22}, {Port: 80}, {Port: 443}},
	}})
	after := d.After.(*OsFirewallSnapshot)
	if after.RuleCount != 3 {
		t.Errorf("RuleCount = %d, want 3", after.RuleCount)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsFirewall: &config.OsFirewall{State: "absent", Rule: &config.FirewallRule{Port: 22}}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_RiskIs7(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsFirewall: &config.OsFirewall{Rule: &config.FirewallRule{Port: 22}}})
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7", c.Risk)
	}
}

func TestCost_ResourcesCountsRules(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsFirewall: &config.OsFirewall{
		Rules: []config.FirewallRule{{Port: 22}, {Port: 80}},
	}})
	if c.Resources != 2 {
		t.Errorf("Resources = %d, want 2", c.Resources)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsFirewall: &config.OsFirewall{Rule: &config.FirewallRule{Port: 22}}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
