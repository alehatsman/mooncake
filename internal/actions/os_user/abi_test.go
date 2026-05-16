package os_user //nolint:revive // package name follows action convention

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
	ps := h.Permissions(&config.Step{OsUser: &config.OsUser{Name: "deploy"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if ps.Network {
		t.Errorf("Network must be false; got %+v", ps)
	}
	var wantBins []string
	switch runtime.GOOS {
	case "darwin":
		wantBins = []string{"dscl"}
	default: // linux
		wantBins = []string{"useradd", "usermod", "userdel"}
	}
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

func TestReverse_CreatedUserBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsUserReverseInfo{
		Name:         "deploy",
		AppliedState: "present",
		PriorExisted: false,
	}
	rev, err := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsUser == nil {
		t.Fatal("Reverse must return an os.user step")
	}
	if rev.OsUser.State != "absent" {
		t.Errorf("State = %s, want absent", rev.OsUser.State)
	}
	if rev.OsUser.Name != "deploy" {
		t.Errorf("Name = %s, want deploy", rev.OsUser.Name)
	}
}

func TestReverse_ModifiedUserRestoresFields(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsUserReverseInfo{
		Name:         "deploy",
		AppliedState: "present",
		PriorExisted: true,
		Prior: OsUserSnapshotState{
			UID:     1001,
			GID:     1001,
			Shell:   "/bin/bash",
			Home:    "/home/deploy",
			Comment: "Deploy User",
			Groups:  []string{"docker", "sudo"},
		},
	}
	rev, _ := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}}, r)
	if rev == nil || rev.OsUser == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.OsUser.State != "present" {
		t.Errorf("State = %s, want present", rev.OsUser.State)
	}
	if rev.OsUser.UID == nil || *rev.OsUser.UID != 1001 {
		t.Errorf("UID = %v, want 1001", rev.OsUser.UID)
	}
	if rev.OsUser.Shell != "/bin/bash" {
		t.Errorf("Shell = %s, want /bin/bash", rev.OsUser.Shell)
	}
	if len(rev.OsUser.Groups) != 2 {
		t.Errorf("Groups = %v, want 2 entries", rev.OsUser.Groups)
	}
	if rev.OsUser.AppendGroups == nil || *rev.OsUser.AppendGroups != false {
		t.Errorf("AppendGroups must be set to false for exact-restore semantics; got %v", rev.OsUser.AppendGroups)
	}
}

func TestReverse_DeletedUserGetsRecreated(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsUserReverseInfo{
		Name:         "deploy",
		AppliedState: "absent",
		PriorExisted: true,
		Prior:        OsUserSnapshotState{UID: 1001, GID: 1001, Shell: "/bin/zsh"},
	}
	rev, _ := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy", State: "absent"}}, r)
	if rev == nil || rev.OsUser == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.OsUser.State != "present" {
		t.Errorf("State = %s, want present (recreate)", rev.OsUser.State)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}}, r)
	if err != nil {
		t.Fatalf("Reverse on no-capture must not error; got: %v", err)
	}
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil step; got %+v", step)
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
	_, err := h.Reverse(nil, &config.Step{OsUser: &config.OsUser{Name: "deploy"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
