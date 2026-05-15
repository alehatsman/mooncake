//nolint:revive // Package name matches action name convention (pkg_hold)
package pkg_hold

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
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

func TestReverse_HeldFlipsToUnheld(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &PkgHoldReverseInfo{
		Manager:      "apt",
		AppliedState: "held",
		Mutated:      []string{"git", "vim"},
	}
	rev, err := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Names: []string{"git", "vim"}, State: "held"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.PkgHold == nil {
		t.Fatal("Reverse must return a pkg.hold step")
	}
	if rev.PkgHold.State != "unheld" {
		t.Errorf("rev.State = %s, want unheld", rev.PkgHold.State)
	}
	if len(rev.PkgHold.Names) != 2 || rev.PkgHold.Names[0] != "git" || rev.PkgHold.Names[1] != "vim" {
		t.Errorf("rev.Names = %v, want [git vim]", rev.PkgHold.Names)
	}
	if rev.PkgHold.Manager != "apt" {
		t.Errorf("rev.Manager = %s, want apt", rev.PkgHold.Manager)
	}
}

func TestReverse_UnheldFlipsToHeld(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &PkgHoldReverseInfo{
		Manager:      "apt",
		AppliedState: "unheld",
		Mutated:      []string{"openssh-server"},
	}
	rev, _ := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Name: "openssh-server", State: "unheld"}}, r)
	if rev == nil || rev.PkgHold.State != "held" {
		t.Errorf("rev.State = %v, want held", rev)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	// No ReverseData → apply was a noop (everything already in
	// desired state). Reverse returns (nil, nil).
	step, err := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}}, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
	}
}

func TestReverse_EmptyMutatedIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &PkgHoldReverseInfo{Manager: "apt", AppliedState: "held"}
	step, _ := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}}, r)
	if step != nil {
		t.Errorf("Reverse on empty Mutated must return nil; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong type"
	_, err := h.Reverse(nil, &config.Step{PkgHold: &config.PkgHold{Name: "git"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
