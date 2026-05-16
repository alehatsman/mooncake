package os_systemd //nolint:revive // package name follows action convention

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
	ps := h.Permissions(&config.Step{OsSystemd: &config.OsSystemd{Name: "myapp.service"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 1 || ps.RequiredBinaries[0] != "systemctl" {
		t.Errorf("RequiredBinaries = %v, want [systemctl]", ps.RequiredBinaries)
	}
}

func TestPermissions_FilesystemWriteHonorsPath(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsSystemd: &config.OsSystemd{Name: "myapp.service", Path: "/usr/lib/systemd/system"}})
	want := "/usr/lib/systemd/system/myapp.service"
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != want {
		t.Errorf("FilesystemWrite = %v, want [%s]", ps.FilesystemWrite, want)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/usr/bin/myapp"},
	}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Kind != actions.ResourceService {
		t.Errorf("Kind = %s, want %s", d.Resource.Kind, actions.ResourceService)
	}
	if d.Resource.Identifier != "myapp.service" {
		t.Errorf("Identifier = %s, want myapp.service", d.Resource.Identifier)
	}
	after := d.After.(*actions.ServiceDiff)
	if len(after.Sections) != 1 || after.Sections[0] != "Service" {
		t.Errorf("Sections = %v, want [Service]", after.Sections)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service", State: "absent"}})
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

func TestCost_BaseRiskIs6(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service"}})
	if c.Risk != 6 {
		t.Errorf("Risk = %d, want 6", c.Risk)
	}
}

func TestCost_AbsentRiskIs7(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service", State: "absent"}})
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7 (stops + disables)", c.Risk)
	}
}

func TestCost_StartedBumpsRisk(t *testing.T) {
	h := Handler{}
	started := true
	c, _ := h.Cost(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service", Started: &started}})
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7 (started=true)", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_CreateThenRollbackBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSystemdReverseInfo{
		Name:         "myapp.service",
		Path:         "/etc/systemd/system/myapp.service",
		PriorExisted: false,
	}
	rev, err := h.Reverse(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "myapp.service"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsSystemd == nil {
		t.Fatal("Reverse must return an os.systemd step")
	}
	if rev.OsSystemd.State != "absent" {
		t.Errorf("State = %s, want absent", rev.OsSystemd.State)
	}
	if rev.OsSystemd.Name != "myapp.service" {
		t.Errorf("Name = %s, want myapp.service", rev.OsSystemd.Name)
	}
}

func TestReverse_ModifyRollbackRefuses(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsSystemdReverseInfo{
		Name:         "myapp.service",
		Path:         "/etc/systemd/system/myapp.service",
		PriorExisted: true,
		PriorContent: "[Unit]\nDescription=old\n",
	}
	_, err := h.Reverse(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "myapp.service"}}, r)
	if err == nil {
		t.Fatal("Reverse must refuse when PriorExisted=true (v5 scope)")
	}
	if !strings.Contains(err.Error(), "modify-rollback") {
		t.Errorf("refusal should mention 'modify-rollback'; got: %s", err)
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service"}}, r)
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
	_, err := h.Reverse(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
