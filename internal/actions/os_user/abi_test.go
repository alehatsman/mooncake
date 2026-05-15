package os_user //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsUser: &config.OsUser{Name: "deploy"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if ps.Network {
		t.Errorf("Network must be false; got %+v", ps)
	}
	wantBins := []string{"useradd", "usermod", "userdel"}
	if len(ps.RequiredBinaries) != len(wantBins) {
		t.Errorf("RequiredBinaries = %v, want %v", ps.RequiredBinaries, wantBins)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Kind != actions.ResourceOther {
		t.Errorf("Kind = %s, want %s", d.Resource.Kind, actions.ResourceOther)
	}
	if d.Resource.Identifier != "deploy" {
		t.Errorf("Identifier = %s, want deploy", d.Resource.Identifier)
	}
	if d.Resource.Attributes["kind"] != "os.user" {
		t.Errorf("Attributes[kind] = %s, want os.user", d.Resource.Attributes["kind"])
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy", State: "absent"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_AfterFoldsPrimaryGroupFirst(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsUser: &config.OsUser{
		Name:   "deploy",
		Group:  "deploy",
		Groups: []string{"docker", "sudo"},
	}})
	after, ok := d.After.(*OsUserSnapshot)
	if !ok {
		t.Fatalf("After is not *OsUserSnapshot; got %T", d.After)
	}
	want := []string{"deploy", "docker", "sudo"}
	if len(after.Groups) != len(want) {
		t.Fatalf("Groups len = %d, want %d", len(after.Groups), len(want))
	}
	for i, g := range want {
		if after.Groups[i] != g {
			t.Errorf("Groups[%d] = %s, want %s", i, after.Groups[i], g)
		}
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_PresentRiskIs5(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}})
	if c.Risk != 5 {
		t.Errorf("Risk = %d, want 5 (present)", c.Risk)
	}
}

func TestCost_AbsentRiskIs8(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy", State: "absent"}})
	if c.Risk != 8 {
		t.Errorf("Risk = %d, want 8 (absent — destructive)", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_RefusesPendingCapture(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
