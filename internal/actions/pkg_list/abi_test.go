//nolint:revive // Package name matches action name convention (pkg_list)
package pkg_list

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_NoSudoNoNetwork(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{PkgList: &config.PkgList{}})
	if ps.Sudo {
		t.Errorf("Sudo must be false; pkg.list is read-only. got %+v", ps)
	}
	if ps.Network {
		t.Errorf("Network must be false. got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "dpkg-query" {
		t.Errorf("RequiredBinaries = %v, want [dpkg-query]", ps.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_AlwaysNoop(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{PkgList: &config.PkgList{}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %s, want %s for read-only action", d.Operation, actions.OpNoop)
	}
}

func TestDiff_IdentifierCarriesManager(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgList: &config.PkgList{Manager: "apt"}})
	if d.Resource.Identifier != "list:apt" {
		t.Errorf("Identifier = %s, want list:apt", d.Resource.Identifier)
	}
	d2, _ := h.Diff(nil, &config.Step{PkgList: &config.PkgList{}})
	if d2.Resource.Identifier != "list:auto" {
		t.Errorf("Identifier with empty manager = %s, want list:auto", d2.Resource.Identifier)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_RiskIs1NotReversible(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgList: &config.PkgList{}})
	if c.Risk != 1 {
		t.Errorf("Risk = %d, want 1 (read-only)", c.Risk)
	}
	if c.Reversible {
		t.Errorf("Reversible must be false; pkg.list is read-only — nothing to reverse")
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

// pkg.list deliberately does NOT implement Reverser — assert that
// to lock in the design: pure read actions don't reverse.
func TestHandler_DoesNotImplementReverser(t *testing.T) {
	var h interface{} = Handler{}
	if _, ok := h.(actions.Reverser); ok {
		t.Errorf("pkg.list must NOT implement Reverser; read-only actions are not reversible")
	}
}
