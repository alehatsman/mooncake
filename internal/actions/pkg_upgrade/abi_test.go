//nolint:revive // Package name matches action name convention (pkg_upgrade)
package pkg_upgrade

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_SudoAndNetwork(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{PkgUpgrade: &config.PkgUpgrade{Manager: "apt"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if !ps.Network {
		t.Errorf("Network must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "apt-get" {
		t.Errorf("RequiredBinaries = %v, want [apt-get]", ps.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_FullUpgradeIsUpdate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpUpdate)
	}
	if d.Resource.Identifier != "<system>" {
		t.Errorf("Identifier = %s, want <system>", d.Resource.Identifier)
	}
	after, ok := d.After.(*actions.PkgUpgradeDiff)
	if !ok || !after.FullUpgrade {
		t.Errorf("After must be *actions.PkgUpgradeDiff with FullUpgrade=true; got %+v", d.After)
	}
}

func TestDiff_SubsetUpgrade(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx", "redis"}}})
	if d.Resource.Identifier != "nginx (+1 more)" {
		t.Errorf("Identifier = %s, want 'nginx (+1 more)'", d.Resource.Identifier)
	}
	after := d.After.(*actions.PkgUpgradeDiff)
	if after.FullUpgrade {
		t.Errorf("FullUpgrade must be false for subset upgrade")
	}
	if len(after.Names) != 2 {
		t.Errorf("Names = %v, want 2 entries", after.Names)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_FullUpgradeRiskIs9(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{}})
	if c.Risk != 9 {
		t.Errorf("Risk = %d, want 9 (full system upgrade)", c.Risk)
	}
}

func TestCost_SubsetUpgradeRiskIs8(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{Names: []string{"nginx"}}})
	if c.Risk != 8 {
		t.Errorf("Risk = %d, want 8 (subset upgrade)", c.Risk)
	}
}

func TestCost_NotReversibleByDesign(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{}})
	if c.Reversible {
		t.Errorf("Reversible = true; pkg.upgrade is irreversible by design")
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_RefusesByDesign(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{PkgUpgrade: &config.PkgUpgrade{}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "irreversible")
	// Also assert the message explains the rationale, not just the verdict.
	if !strings.Contains(err.Error(), "downgrad") && !strings.Contains(err.Error(), "version") {
		t.Errorf("refusal should explain why (version capture / downgrade); got: %s", err)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
