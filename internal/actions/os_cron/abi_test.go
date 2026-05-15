package os_cron //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsCron: &config.OsCron{Name: "backup"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.FilesystemWrite) != 1 || ps.FilesystemWrite[0] != "/etc/cron.d/backup" {
		t.Errorf("FilesystemWrite = %v, want [/etc/cron.d/backup]", ps.FilesystemWrite)
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_PresentIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsCron: &config.OsCron{
		Name:    "backup",
		Hour:    "3",
		Command: "/usr/local/bin/backup.sh",
	}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Identifier != "backup" {
		t.Errorf("Identifier = %s, want backup", d.Resource.Identifier)
	}
	after := d.After.(*OsCronSnapshot)
	if after.Schedule != "* 3 * * *" {
		t.Errorf("Schedule = %q, want '* 3 * * *'", after.Schedule)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsCron: &config.OsCron{Name: "backup", State: "absent"}})
	if d.Operation != actions.OpDelete {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpDelete)
	}
}

func TestDiff_ExplicitSchedule(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsCron: &config.OsCron{
		Name:     "backup",
		Schedule: "@daily",
		Command:  "x",
	}})
	after := d.After.(*OsCronSnapshot)
	if after.Schedule != "@daily" {
		t.Errorf("Schedule = %q, want @daily", after.Schedule)
	}
}

func TestDiff_NilStep(t *testing.T) {
	h := Handler{}
	testutil.AssertNilStepErrors(t, "Diff", func() error { _, err := h.Diff(nil, nil); return err })
}

func TestDiff_RegisteredAsDiffer(t *testing.T) {
	var _ actions.Differ = (*Handler)(nil)
}

func TestCost_RiskIs4(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsCron: &config.OsCron{Name: "x"}})
	if c.Risk != 4 {
		t.Errorf("Risk = %d, want 4", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_CreatedFileBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsCronReverseInfo{
		Path:         "/etc/cron.d/backup",
		PriorExisted: false,
	}
	rev, err := h.Reverse(nil, &config.Step{OsCron: &config.OsCron{Name: "backup"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.FileWrite == nil {
		t.Fatal("Reverse must return a file.write step")
	}
	if rev.FileWrite.State != "absent" {
		t.Errorf("State = %s, want absent", rev.FileWrite.State)
	}
	if rev.FileWrite.Path != "/etc/cron.d/backup" {
		t.Errorf("Path = %s, want /etc/cron.d/backup", rev.FileWrite.Path)
	}
}

func TestReverse_ExistingFileContentRestored(t *testing.T) {
	h := Handler{}
	prior := "# managed by mooncake\n0 3 * * * root /old/backup.sh\n"
	r := executor.NewResult()
	r.ReverseData = &OsCronReverseInfo{
		Path:         "/etc/cron.d/backup",
		PriorExisted: true,
		PriorContent: prior,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsCron: &config.OsCron{Name: "backup"}}, r)
	if rev == nil || rev.FileWrite == nil {
		t.Fatal("Reverse must return a file.write step")
	}
	if rev.FileWrite.State != "file" {
		t.Errorf("State = %s, want file", rev.FileWrite.State)
	}
	if rev.FileWrite.Content != prior {
		t.Errorf("Content mismatch; want %q, got %q", prior, rev.FileWrite.Content)
	}
	if !rev.FileWrite.Force {
		t.Error("Force must be true on reverse (overwrite whatever apply left)")
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, err := h.Reverse(nil, &config.Step{OsCron: &config.OsCron{Name: "x"}}, r)
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
	r.ReverseData = "wrong type"
	_, err := h.Reverse(nil, &config.Step{OsCron: &config.OsCron{Name: "x"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
