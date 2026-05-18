//nolint:revive // package name follows action convention
package os_mount

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

// stubFS redirects fstab + /proc/mounts paths to a tempdir, stubs the
// mount/umount runners so tests stay hermetic, and records all calls.
type stubFS struct {
	dir     string
	fstab   string
	mounts  string
	mounted []string // dests passed to mount(8)
	umount  []string // dests passed to umount(8)
}

func newStub(t *testing.T) *stubFS {
	t.Helper()
	dir := t.TempDir()
	s := &stubFS{
		dir:    dir,
		fstab:  filepath.Join(dir, "fstab"),
		mounts: filepath.Join(dir, "mounts"),
	}
	origPaths := mountPaths
	origMount := mountRun
	origUmount := umountRun
	origClock := clockNow

	mountPaths.fstab = s.fstab
	mountPaths.mounts = s.mounts
	mountRun = func(_ *security.Privileged, dest string) error {
		s.mounted = append(s.mounted, dest)
		return nil
	}
	umountRun = func(_ *security.Privileged, dest string) error {
		s.umount = append(s.umount, dest)
		// Remove from the mocked /proc/mounts so subsequent reads see
		// the change.
		data, err := os.ReadFile(s.mounts)
		if err != nil {
			return nil
		}
		out := []string{}
		for _, ln := range strings.Split(string(data), "\n") {
			fields := strings.Fields(ln)
			if len(fields) >= 2 && fields[1] == dest {
				continue
			}
			out = append(out, ln)
		}
		_ = os.WriteFile(s.mounts, []byte(strings.Join(out, "\n")), 0o644)
		return nil
	}
	clockNow = func() time.Time { return time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC) }

	t.Cleanup(func() {
		mountPaths = origPaths
		mountRun = origMount
		umountRun = origUmount
		clockNow = origClock
	})
	return s
}

// seedFstab writes the given content as the fstab file.
func (s *stubFS) seedFstab(t *testing.T, content string) {
	t.Helper()
	if err := os.WriteFile(s.fstab, []byte(content), 0o644); err != nil {
		t.Fatalf("seed fstab: %v", err)
	}
}

// seedMount marks `dest` as currently mounted in the stubbed
// /proc/mounts file.
func (s *stubFS) seedMount(t *testing.T, src, dest, fstype string) {
	t.Helper()
	line := src + " " + dest + " " + fstype + " rw 0 0\n"
	prev, _ := os.ReadFile(s.mounts)
	if err := os.WriteFile(s.mounts, append(prev, []byte(line)...), 0o644); err != nil {
		t.Fatalf("seed mount: %v", err)
	}
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no dest", &config.Step{OsMount: &config.OsMount{Src: "/dev/sdb1", FSType: "ext4"}}, true},
		{"present no src", &config.Step{OsMount: &config.OsMount{Dest: "/data", FSType: "ext4"}}, true},
		{"present no fstype", &config.Step{OsMount: &config.OsMount{Dest: "/data", Src: "/dev/sdb1"}}, true},
		{"bad state", &config.Step{OsMount: &config.OsMount{Dest: "/data", Src: "/dev/sdb1", FSType: "ext4", State: "weird"}}, true},
		{"ok mounted", &config.Step{OsMount: &config.OsMount{Dest: "/data", Src: "/dev/sdb1", FSType: "ext4"}}, false},
		{"ok absent without src/fstype", &config.Step{OsMount: &config.OsMount{Dest: "/data", State: "absent"}}, false},
		{"ok fstab_only", &config.Step{OsMount: &config.OsMount{Dest: "/data", Src: "UUID=abc", FSType: "ext4", State: "fstab_only"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestApply_CreatesEntryAndMounts(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{OsMount: &config.OsMount{
		Src:     "/dev/sdb1",
		Dest:    "/data",
		FSType:  "ext4",
		Options: []string{"defaults", "noatime"},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	got, err := os.ReadFile(s.fstab)
	if err != nil {
		t.Fatalf("read fstab: %v", err)
	}
	want := "/dev/sdb1 /data ext4 defaults,noatime 0 0\n"
	if string(got) != want {
		t.Errorf("fstab mismatch\n got %q\nwant %q", got, want)
	}
	if len(s.mounted) != 1 || s.mounted[0] != "/data" {
		t.Errorf("expected single mount call for /data; got %v", s.mounted)
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, "/dev/sdb1 /data ext4 defaults,noatime 0 0\n")
	s.seedMount(t, "/dev/sdb1", "/data", "ext4")
	step := &config.Step{OsMount: &config.OsMount{
		Src:     "/dev/sdb1",
		Dest:    "/data",
		FSType:  "ext4",
		Options: []string{"defaults", "noatime"},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
	if len(s.mounted) != 0 {
		t.Errorf("no mount call expected; got %v", s.mounted)
	}
}

func TestApply_DriftTriggersFstabUpdate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, "/dev/sdb1 /data ext4 defaults 0 0\n")
	s.seedMount(t, "/dev/sdb1", "/data", "ext4")
	step := &config.Step{OsMount: &config.OsMount{
		Src:     "/dev/sdb1",
		Dest:    "/data",
		FSType:  "ext4",
		Options: []string{"defaults", "noatime"},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected drift to trigger change; reason=%q", r.Reason)
	}
	got, _ := os.ReadFile(s.fstab)
	if !strings.Contains(string(got), "defaults,noatime") {
		t.Errorf("options not updated: %q", got)
	}
}

func TestApply_PreservesOtherLines(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	initial := strings.Join([]string{
		"# /etc/fstab: static file system information.",
		"",
		"UUID=root-uuid / ext4 defaults 0 1",
		"tmpfs /tmp tmpfs defaults 0 0",
		"",
	}, "\n")
	s.seedFstab(t, initial)
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(s.fstab)
	gotS := string(got)
	for _, want := range []string{
		"# /etc/fstab: static file system information.",
		"UUID=root-uuid / ext4 defaults 0 1",
		"tmpfs /tmp tmpfs defaults 0 0",
		"/dev/sdb1 /data ext4 defaults 0 0",
	} {
		if !strings.Contains(gotS, want) {
			t.Errorf("missing line %q\nfull:\n%s", want, gotS)
		}
	}
}

func TestApply_UnmountedKeepsEntryUnmountsNow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, "/dev/sdb1 /data ext4 defaults 0 0\n")
	s.seedMount(t, "/dev/sdb1", "/data", "ext4")
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
		State:  "unmounted",
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected umount; reason=%q", r.Reason)
	}
	if len(s.umount) != 1 || s.umount[0] != "/data" {
		t.Errorf("expected umount call; got %v", s.umount)
	}
	got, _ := os.ReadFile(s.fstab)
	if !strings.Contains(string(got), "/dev/sdb1 /data ext4") {
		t.Errorf("fstab entry should remain: %q", got)
	}
}

func TestApply_FstabOnly_WritesEntryDoesNotMount(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
		State:  "fstab_only",
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected fstab write; reason=%q", r.Reason)
	}
	if len(s.mounted) != 0 {
		t.Errorf("fstab_only must not mount; got %v", s.mounted)
	}
	got, _ := os.ReadFile(s.fstab)
	if !strings.Contains(string(got), "/dev/sdb1 /data ext4") {
		t.Errorf("entry not written: %q", got)
	}
}

func TestAbsent_UnmountsAndRemoves(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, strings.Join([]string{
		"UUID=root-uuid / ext4 defaults 0 1",
		"/dev/sdb1 /data ext4 defaults 0 0",
		"",
	}, "\n"))
	s.seedMount(t, "/dev/sdb1", "/data", "ext4")
	step := &config.Step{OsMount: &config.OsMount{
		Dest:  "/data",
		State: "absent",
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected delete; reason=%q", r.Reason)
	}
	if len(s.umount) != 1 {
		t.Errorf("expected one umount; got %v", s.umount)
	}
	got, _ := os.ReadFile(s.fstab)
	if strings.Contains(string(got), "/dev/sdb1 /data") {
		t.Errorf("/data entry should be removed: %q", got)
	}
	if !strings.Contains(string(got), "UUID=root-uuid /") {
		t.Errorf("root entry should remain: %q", got)
	}
}

func TestAbsent_NoopWhenMissingAndUnmounted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStub(t)
	step := &config.Step{OsMount: &config.OsMount{
		Dest:  "/data",
		State: "absent",
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent on missing should be no-op; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWriteOrMount(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(s.fstab); err == nil {
		t.Error("plan must not write fstab")
	}
	if len(s.mounted) != 0 {
		t.Errorf("plan must not call mount; got %v", s.mounted)
	}
}

func TestApply_BackupSnapshotsFstab(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	original := "UUID=root-uuid / ext4 defaults 0 1\n"
	s.seedFstab(t, original)
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
	}}
	_ = mustRun(t, false, step)
	backup := s.fstab + ".bak.20260514T120000Z"
	data, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("backup not written: %v", err)
	}
	if string(data) != original {
		t.Errorf("backup content mismatch\n got %q\nwant %q", data, original)
	}
}

func TestApply_BackupDisabledSkipsSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, "UUID=root-uuid / ext4 defaults 0 1\n")
	off := false
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "ext4",
		Backup: &off,
	}}
	_ = mustRun(t, false, step)
	backup := s.fstab + ".bak.20260514T120000Z"
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Errorf("backup should not be written; stat err=%v", err)
	}
}

func TestApply_RefusesRemountOnFstypeMismatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.seedFstab(t, "/dev/sdb1 /data ext4 defaults 0 0\n")
	s.seedMount(t, "/dev/sdb1", "/data", "ext4")
	step := &config.Step{OsMount: &config.OsMount{
		Src:    "/dev/sdb1",
		Dest:   "/data",
		FSType: "xfs", // mismatched
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error on remount with fstype mismatch")
	}
	if !strings.Contains(err.Error(), "refusing to remount") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCanonicalDest_TrailingSlash(t *testing.T) {
	if canonicalDest("/data/") != "/data" {
		t.Errorf("trailing slash not stripped")
	}
	if canonicalDest("/data") != "/data" {
		t.Errorf("canonical of /data should be /data")
	}
}

func TestSameOptions_OrderInsensitive(t *testing.T) {
	if !sameOptions([]string{"defaults", "noatime"}, []string{"noatime", "defaults"}) {
		t.Error("set comparison should ignore order")
	}
	if sameOptions([]string{"defaults"}, []string{"defaults", "noatime"}) {
		t.Error("different lengths should not match")
	}
}
