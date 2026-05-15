//nolint:revive // Package name matches action name convention (pkg_repo)
package pkg_repo

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{PkgRepo: &config.PkgRepo{Name: "nodesource"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true for pkg.repo; got %+v", ps)
	}
}

func TestPermissions_NetworkOnlyWithGPGKeyURL(t *testing.T) {
	h := Handler{}
	withKey := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "nodesource",
		State: "present",
		Apt:   &config.PkgRepoApt{URI: "https://x", GPGKeyURL: "https://x.key"},
	}}
	withoutKey := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "nodesource",
		State: "present",
		Apt:   &config.PkgRepoApt{URI: "https://x"},
	}}
	if !h.Permissions(withKey).Network {
		t.Error("Network must be true when GPGKeyURL is set")
	}
	if h.Permissions(withoutKey).Network {
		t.Error("Network must be false when GPGKeyURL is empty")
	}
}

func TestPermissions_NetworkFalseOnAbsent(t *testing.T) {
	h := Handler{}
	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name:  "nodesource",
		State: "absent",
		Apt:   &config.PkgRepoApt{URI: "https://x", GPGKeyURL: "https://x.key"},
	}}
	if h.Permissions(step).Network {
		t.Error("Network must be false when state=absent (no key fetch)")
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: &config.PkgRepoApt{URI: "https://x"}}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Kind != actions.ResourcePackage {
		t.Errorf("Kind = %s, want %s", d.Resource.Kind, actions.ResourcePackage)
	}
	if d.Resource.Identifier != "x" {
		t.Errorf("Identifier = %s, want x", d.Resource.Identifier)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x", State: "absent"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_AfterCarriesDriver(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x", Apt: &config.PkgRepoApt{URI: "https://x"}}})
	after, ok := d.After.(*PkgRepoSnapshot)
	if !ok {
		t.Fatalf("After is not *PkgRepoSnapshot; got %T", d.After)
	}
	if after.Driver != "apt" {
		t.Errorf("Driver = %s, want apt", after.Driver)
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
	c, err := h.Cost(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x"}})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6", c.Risk)
	}
}

func TestCost_OneResourceReversibleByInterface(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x"}})
	if c.Resources != 1 {
		t.Errorf("Resources = %d, want 1", c.Resources)
	}
	if !c.Reversible {
		t.Errorf("Reversible = false; pkg.repo opts into Reverser interface")
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_RefusesPendingCapture(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{PkgRepo: &config.PkgRepo{Name: "x"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
