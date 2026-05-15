package os_mount //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestPermissions_AlwaysSudo(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsMount: &config.OsMount{Dest: "/mnt/data", Src: "/dev/sdb1", FSType: "ext4"}})
	if !ps.Sudo {
		t.Errorf("Sudo must be true; got %+v", ps)
	}
	if len(ps.RequiredBinaries) != 2 {
		t.Errorf("RequiredBinaries = %v, want [mount, umount]", ps.RequiredBinaries)
	}
}

func TestPermissions_FilesystemWriteIncludesFstabAndDest(t *testing.T) {
	h := Handler{}
	ps := h.Permissions(&config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}})
	if len(ps.FilesystemWrite) != 2 {
		t.Fatalf("FilesystemWrite = %v, want [/etc/fstab, /mnt/data]", ps.FilesystemWrite)
	}
	if ps.FilesystemWrite[0] != "/etc/fstab" {
		t.Errorf("FilesystemWrite[0] = %s, want /etc/fstab", ps.FilesystemWrite[0])
	}
	if ps.FilesystemWrite[1] != "/mnt/data" {
		t.Errorf("FilesystemWrite[1] = %s, want /mnt/data", ps.FilesystemWrite[1])
	}
}

func TestPermissions_RegisteredAsPermitter(t *testing.T) {
	var _ actions.Permitter = (*Handler)(nil)
}

func TestDiff_MountedIsCreate(t *testing.T) {
	h := Handler{}
	d, err := h.Diff(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data", Src: "/dev/sdb1", FSType: "ext4"}})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %s, want %s", d.Operation, actions.OpCreate)
	}
	if d.Resource.Identifier != "/mnt/data" {
		t.Errorf("Identifier = %s, want /mnt/data", d.Resource.Identifier)
	}
}

func TestDiff_UnmountedIsUpdate(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data", State: "unmounted"}})
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %s, want %s (unmounted but fstab kept)", d.Operation, actions.OpUpdate)
	}
}

func TestDiff_AbsentIsDelete(t *testing.T) {
	h := Handler{}
	d, _ := h.Diff(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data", State: "absent"}})
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

func TestCost_BaseRiskIs7(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}})
	if c.Risk != 7 {
		t.Errorf("Risk = %d, want 7", c.Risk)
	}
}

func TestCost_AbsentRiskIs8(t *testing.T) {
	h := Handler{}
	c, _ := h.Cost(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data", State: "absent"}})
	if c.Risk != 8 {
		t.Errorf("Risk = %d, want 8 (absent removes fstab entry)", c.Risk)
	}
}

func TestCost_RegisteredAsCoster(t *testing.T) {
	var _ actions.Coster = (*Handler)(nil)
}

func TestReverse_Refuses(t *testing.T) {
	h := Handler{}
	step, err := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, nil)
	testutil.AssertReverseRefuses(t, step, err, "not yet implemented")
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
