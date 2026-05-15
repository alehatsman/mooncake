package os_systemd //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
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
	after := d.After.(*OsSystemdSnapshot)
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

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
