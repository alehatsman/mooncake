package os_mount //nolint:revive // package name follows action convention

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
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

func TestReverse_AddedAndMountedBecomesAbsent(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsMountReverseInfo{
		Dest:         "/mnt/data",
		PriorEntry:   nil,
		PriorMounted: false,
		TouchedFstab: true,
		TouchedMount: true,
	}
	rev, err := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev == nil || rev.OsMount == nil {
		t.Fatal("Reverse must return an os.mount step")
	}
	if rev.OsMount.State != "absent" {
		t.Errorf("State = %s, want absent", rev.OsMount.State)
	}
}

func TestReverse_PriorEntryAndMountedRestoresMount(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsMountReverseInfo{
		Dest: "/mnt/data",
		PriorEntry: &OsMountSnapshotEntry{
			Src:     "/dev/sdb1",
			Dest:    "/mnt/data",
			FSType:  "ext4",
			Options: []string{"defaults", "noatime"},
		},
		PriorMounted: true,
		TouchedFstab: true,
		TouchedMount: true,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if rev == nil || rev.OsMount == nil {
		t.Fatal("Reverse must return a step")
	}
	if rev.OsMount.State != "mounted" {
		t.Errorf("State = %s, want mounted", rev.OsMount.State)
	}
	if rev.OsMount.Src != "/dev/sdb1" {
		t.Errorf("Src = %s, want /dev/sdb1", rev.OsMount.Src)
	}
	if len(rev.OsMount.Options) != 2 {
		t.Errorf("Options = %v, want [defaults noatime]", rev.OsMount.Options)
	}
}

func TestReverse_PriorEntryUnmountedRestoresFstabOnly(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsMountReverseInfo{
		Dest: "/mnt/data",
		PriorEntry: &OsMountSnapshotEntry{
			Src:    "UUID=abc",
			Dest:   "/mnt/data",
			FSType: "ext4",
		},
		PriorMounted: false,
		TouchedFstab: true,
	}
	rev, _ := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if rev == nil || rev.OsMount.State != "fstab_only" {
		t.Errorf("State = %v, want fstab_only", rev)
	}
}

func TestReverse_NoEntryButMountedRefuses(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsMountReverseInfo{
		Dest:         "/mnt/data",
		PriorMounted: true,
		TouchedMount: true,
		// PriorEntry: nil — manually mounted without fstab.
	}
	_, err := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when prior state was 'mounted without fstab entry'")
	}
}

func TestReverse_NoReverseDataIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	step, _ := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if step != nil {
		t.Errorf("Reverse on no-capture must return nil; got %+v", step)
	}
}

func TestReverse_NeitherTouchedIsNoop(t *testing.T) {
	h := Handler{}
	r := executor.NewResult()
	r.ReverseData = &OsMountReverseInfo{Dest: "/mnt/data"}
	step, _ := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if step != nil {
		t.Errorf("Reverse with neither touched must return nil; got %+v", step)
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
	_, err := h.Reverse(nil, &config.Step{OsMount: &config.OsMount{Dest: "/mnt/data"}}, r)
	if err == nil {
		t.Fatal("Reverse must error when ReverseData has wrong type")
	}
}

func TestReverse_RegisteredAsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
}
