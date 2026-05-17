package os_group //nolint:revive // package name follows action convention

import (
	"runtime"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsGroup: &config.OsGroup{Name: "docker"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if ps.Network {
		t.Errorf("Network must be false; got %+v", ps)
	}
}

// TestPermissions_RequiredBinariesVaryByGOOS pins the host-shaped
// permission set so spec-44 doctor reports usable findings on each
// platform. Linux needs groupadd/groupdel + /etc/group write; darwin
// needs dscl and no concrete filesystem path (the directory store
// isn't a file). Mirrors os_user's abi_test_GOOS check.
func TestPermissions_RequiredBinariesVaryByGOOS(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsGroup: &config.OsGroup{Name: "docker"}})
	var wantBins []string
	switch runtime.GOOS {
	case "darwin":
		wantBins = []string{"dscl"}
		if len(ps.FilesystemWrite) != 0 {
			t.Errorf("darwin: FilesystemWrite should be empty (no path-shaped store); got %v", ps.FilesystemWrite)
		}
	default:
		wantBins = []string{"groupadd", "groupdel"}
	}
	if len(ps.RequiredBinaries) != len(wantBins) {
		t.Fatalf("RequiredBinaries = %v, want %v", ps.RequiredBinaries, wantBins)
	}
	for i, b := range wantBins {
		if ps.RequiredBinaries[i] != b {
			t.Errorf("RequiredBinaries[%d] = %q, want %q", i, ps.RequiredBinaries[i], b)
		}
	}
}

// TestMetadata_AdvertisesLinuxAndDarwin guards SupportedPlatforms.
// If a future change drops darwin (or adds another platform), this
// test fails and forces docs / finder UX updates in lockstep.
func TestMetadata_AdvertisesLinuxAndDarwin(t *testing.T) {
	m := (&Handler{}).Metadata()
	want := map[string]bool{"linux": true, "darwin": true}
	got := map[string]bool{}
	for _, p := range m.SupportedPlatforms {
		got[p] = true
	}
	for p := range want {
		if !got[p] {
			t.Errorf("SupportedPlatforms missing %s: %v", p, m.SupportedPlatforms)
		}
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker"}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Identifier != "docker" {
		t.Errorf("Identifier = %s, want docker", d.Resource.Identifier)
	}
	if d.Resource.Attributes["kind"] != "os.group" {
		t.Errorf("Attributes[kind] = %s, want os.group", d.Resource.Attributes["kind"])
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker", State: "absent"}})
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

func TestCost_PresentRiskIs4(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker"}})
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4", c.Risk)
	}
}

func TestCost_AbsentRiskIs7(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker", State: "absent"}})
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_CreatedGroupBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsGroupReverseInfo{
		Name:         "docker",
		AppliedState: "present",
		PriorExisted: false,
	}
	rev, err := h.Reverse(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsGroup == nil {
		t.Fatal("Reverse must return an os.group step")
	}
	if rev.OsGroup.State != "absent" {
		t.Errorf("State = %s, want absent", rev.OsGroup.State)
	}
}

func TestReverse_PriorExistedRestoresGID(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsGroupReverseInfo{
		Name:         "docker",
		AppliedState: "absent",
		PriorExisted: true,
		PriorGID:     999,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker", State: "absent"}}, r)
	if rev == nil || rev.OsGroup == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.OsGroup.State != "present" {
		t.Errorf("State = %s, want present", rev.OsGroup.State)
	}
	if rev.OsGroup.GID == nil || *rev.OsGroup.GID != 999 {
		t.Errorf("GID = %v, want 999", rev.OsGroup.GID)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker"}}, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil; got %+v", step)
	}
}

func TestReverse_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Reverse", func() error { _, err := h.Reverse(nil, nil, nil); return err })
}

func TestReverse_WrongReverseDataType(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = "wrong"
	_, err := h.Reverse(nil, &config.Step{OsGroup: &config.OsGroup{Name: "docker"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
