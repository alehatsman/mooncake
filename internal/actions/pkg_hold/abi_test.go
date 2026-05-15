//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{PkgHold: &config.PkgHold{Name: "git"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for pkg.hold; got %+v", ps)
	}
	if ps.Network {
		t.Errorf("Network must be false for pkg.hold; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "apt-mark" {
		t.Errorf("RequiredBinaries = %v, want [apt-mark]", ps.RequiredBinaries)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_HeldIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git", State: "held"}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Identifier != "git" {
		t.Errorf("Identifier = %s, want git", d.Resource.Identifier)
	}
}

func TestDiff_UnheldIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git", State: "unheld"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_EmptyStateDefaultsToHeld(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}})
	if d.Operation != actions.OpCreate {
		t.Errorf("empty state should default to held → OpCreate; got %s", d.Operation)
	}
}

func TestDiff_MultiName(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgHold: &config.PkgHold{Names: []string{"a", "b", "c"}}})
	after, ok := d.After.(*PkgHoldSnapshot)
	if !ok {
		t.Fatalf("After is not *PkgHoldSnapshot")
	}
	if len(after.Names) != 3 {
		t.Errorf("Names = %v, want 3 entries", after.Names)
	}
	if !strings.Contains(d.Resource.Identifier, "a") || !strings.Contains(d.Resource.Identifier, "+2") {
		t.Errorf("Identifier should show first name + extras count; got %s", d.Resource.Identifier)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_RiskIs3(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}})
	if c.Risk != 3 {
		t.Errorf("Risk = %d, want 3", c.Risk)
	}
}

func TestCost_ResourcesCountsNames(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgHold: &config.PkgHold{Names: []string{"a", "b"}}})
	if c.Resources != 2 {
		t.Errorf("Resources = %d, want 2", c.Resources)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
