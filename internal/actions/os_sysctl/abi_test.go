package os_sysctl //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "sysctl" {
		t.Errorf("RequiredBinaries = %v, want [sysctl]", ps.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsUpdate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpUpdate)
	}
	after := d.After.(*OsSysctlSnapshot)
	if after.Value != "1" {
		t.Errorf("Value = %q, want '1'", after.Value)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x", State: "absent"}})
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

func TestCost_RiskIs6(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}})
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsSysctl: &config.OsSysctl{Name: "x"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
