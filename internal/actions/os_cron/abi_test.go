package os_cron //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
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

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsCron: &config.OsCron{Name: "x"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
