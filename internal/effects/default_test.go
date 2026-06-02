package effects

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// newTestPerformer returns a Performer pinned to the given mode and with
// no sudo password (sudo-aware paths return an error when exercised).
func newTestPerformer(mode actions.Mode) actions.Performer {
	return NewPerformer(func() actions.Mode { return mode }, "", false, "")
}

// ----------------------------------------------------------------------
// Mkdir
// ----------------------------------------------------------------------

func TestMkdir_ModePlan_NonExistent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newdir")
	p := newTestPerformer(actions.ModePlan)

	got := p.Mkdir(dir, 0o755, actions.PerformerOpts{})

	if got.Performed {
		t.Error("ModePlan should not perform")
	}
	if !got.WouldChange {
		t.Error("WouldChange should be true for non-existent dir")
	}
	if got.AlreadyOk {
		t.Error("AlreadyOk should be false")
	}
	if got.Err != nil {
		t.Errorf("unexpected error: %v", got.Err)
	}
	if got.Action != actions.ActionMkdir {
		t.Errorf("Action = %v, want %v", got.Action, actions.ActionMkdir)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("ModePlan should not create directory; got stat err=%v", err)
	}
}

func TestMkdir_ModePlan_AlreadyExists(t *testing.T) {
	dir := t.TempDir() // already exists with default mode
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := newTestPerformer(actions.ModePlan)

	got := p.Mkdir(dir, 0o755, actions.PerformerOpts{})

	if !got.AlreadyOk {
		t.Errorf("AlreadyOk should be true; reason=%q", got.Reason)
	}
	if got.WouldChange {
		t.Error("WouldChange should be false when already in desired state")
	}
}

func TestMkdir_ModeApply_CreatesWithMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newdir")
	p := newTestPerformer(actions.ModeApply)

	got := p.Mkdir(dir, 0o750, actions.PerformerOpts{})

	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.Performed {
		t.Error("Performed should be true after creation")
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !info.IsDir() {
		t.Error("not a directory")
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o750); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}

// TestMkdir_DefaultModeRegressionParity is the Spec-16 mode-parity test
// for directories. Drives the SAME path + mode through ModePlan and
// ModeApply and asserts the plan prediction matches the execute reality.
// If this fails, the preview is lying about what execute does — exactly
// the bug class that motivated Spec 16.
func TestMkdir_DefaultModeRegressionParity(t *testing.T) {
	for _, mode := range []os.FileMode{0o755, 0o700, 0o775} {
		mode := mode
		t.Run(mode.String(), func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "regress")

			plan := newTestPerformer(actions.ModePlan).
				Mkdir(dir, mode, actions.PerformerOpts{})
			if !plan.WouldChange {
				t.Fatalf("plan: expected WouldChange=true")
			}

			exec := newTestPerformer(actions.ModeApply).
				Mkdir(dir, mode, actions.PerformerOpts{})
			if exec.Err != nil {
				t.Fatalf("execute: %v", exec.Err)
			}
			if !exec.Performed {
				t.Fatal("execute: expected Performed=true")
			}

			info, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat after execute: %v", err)
			}
			if got := info.Mode().Perm(); got != mode {
				t.Errorf("execute applied mode %v, plan said mode %v", got, mode)
			}
		})
	}
}

// ----------------------------------------------------------------------
// WriteFile
// ----------------------------------------------------------------------

func TestWriteFile_ModeApply_CreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	p := newTestPerformer(actions.ModeApply)

	got := p.WriteFile(path, []byte("hello"), 0o644, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.Performed {
		t.Error("Performed should be true")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", string(data), "hello")
	}
}

func TestWriteFile_AlreadyMatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestPerformer(actions.ModeApply)

	got := p.WriteFile(path, []byte("same"), 0o644, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk should be true; got %+v", got)
	}
	if got.Performed {
		t.Error("Performed should be false when content already matches")
	}
}

func TestWriteFile_ContentDiff_PlanMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newTestPerformer(actions.ModePlan)

	got := p.WriteFile(path, []byte("newer content"), 0o644, actions.PerformerOpts{})
	if !got.WouldChange {
		t.Error("WouldChange should be true")
	}
	d, ok := got.Detail.(ContentDiff)
	if !ok {
		t.Fatalf("Detail = %T, want ContentDiff", got.Detail)
	}
	if d.OldSize != 3 || d.NewSize != 13 {
		t.Errorf("Detail sizes = %+v", d)
	}
	// Plan must not have written
	cur, _ := os.ReadFile(path)
	if string(cur) != "old" {
		t.Errorf("plan mutated file: %q", string(cur))
	}
}

// Issue #95: WriteFile / CopyFile must create a missing parent
// directory through the performer's own Mkdir (which honors
// needSudoForOwnership), not a bare unprivileged os.MkdirAll. These
// tests exercise the three branches the fix has to get right.

// namedNonCurrentUser returns a username that is guaranteed to differ
// from the test runner's current user so needSudoForOwnership() is
// driven to true. "nobody" is POSIX-conventional; skip if we happen
// to be running as it.
func namedNonCurrentUser(t *testing.T) string {
	t.Helper()
	if currentUserName(t) == "nobody" {
		t.Skip("test runs as 'nobody'; can't drive the cross-user escalation branch")
	}
	return "nobody"
}

// TestWriteFile_MissingParent_UnderAsUser_OwnedViaPrivilegedMkdir — the
// #95 fix (maintainer decision): a MISSING parent under as_user is created
// through the privileged Mkdir path so the bundled chown gives it to
// as_user, NOT the invoking user — even in an otherwise-writable location.
// With no sudo password configured that path errors, surfacing as a
// "mkdir parent" failure (proving it routed through privileged mkdir) and
// the parent is not created.
func TestWriteFile_MissingParent_UnderAsUser_OwnedViaPrivilegedMkdir(t *testing.T) {
	asUser := namedNonCurrentUser(t)
	root := t.TempDir() // writable by the invoker
	parent := filepath.Join(root, "missing-sub")
	path := filepath.Join(parent, "f.txt")

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, asUser)
	got := p.WriteFile(path, []byte("hi"), 0o644, actions.PerformerOpts{})

	if got.Err == nil {
		t.Fatalf("expected error from privileged mkdir path (no sudo pass), got Performed=%v", got.Performed)
	}
	if !strings.Contains(got.Err.Error(), "mkdir parent") {
		t.Errorf("missing parent under as_user must route through privileged mkdir, got: %v", got.Err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Errorf("parent should not have been created (sudo unavailable); stat err=%v", err)
	}
}

// TestCopyFile_MissingParent_UnderAsUser_OwnedViaPrivilegedMkdir — same
// as_user ownership routing for CopyFile's parent creation.
func TestCopyFile_MissingParent_UnderAsUser_OwnedViaPrivilegedMkdir(t *testing.T) {
	asUser := namedNonCurrentUser(t)
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, "missing-sub")
	dest := filepath.Join(parent, "dest.txt")

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, asUser)
	got := p.CopyFile(src, dest, 0o644, actions.PerformerOpts{})

	if got.Err == nil {
		t.Fatalf("expected error from privileged mkdir path (no sudo pass), got Performed=%v", got.Performed)
	}
	if !strings.Contains(got.Err.Error(), "mkdir parent") {
		t.Errorf("missing parent under as_user must route through privileged mkdir, got: %v", got.Err)
	}
	if _, err := os.Stat(parent); !os.IsNotExist(err) {
		t.Errorf("parent should not have been created (sudo unavailable); stat err=%v", err)
	}
}

// TestWriteFile_ParentExists_ModePreserved — a stattable EXISTING parent
// is left completely untouched even when as_user is bound: it is never
// re-created (no "mkdir parent" escalation) and its mode is never
// re-applied. This is the guard for the os_ssh_key regression — a
// deliberately-tightened 0700 ~/.ssh must survive a write into it.
func TestWriteFile_ParentExists_ModePreserved(t *testing.T) {
	asUser := namedNonCurrentUser(t)
	root := t.TempDir()
	parent := filepath.Join(root, "sub")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(parent, "f.txt")

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, asUser)
	got := p.WriteFile(path, []byte("hi"), 0o644, actions.PerformerOpts{})

	// Existing parent: never re-created/escalated...
	if got.Err != nil && strings.Contains(got.Err.Error(), "mkdir parent") {
		t.Errorf("existing parent must not be re-created/escalated, got: %v", got.Err)
	}
	// ...and its tightened 0700 mode must be preserved.
	info, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("parent mode = %o, want 700 (existing dir must not be re-chmod'd)", info.Mode().Perm())
	}
}

// TestWriteFile_MissingParent_NoEscalation_UnprivilegedCreate — with
// no AsUser bound, parent creation falls through to the unprivileged
// path exactly as before: the missing parent is created and the file
// is written. Guards that the fix didn't regress the common path.
func TestWriteFile_MissingParent_NoEscalation_UnprivilegedCreate(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "auto", "nested")
	path := filepath.Join(parent, "f.txt")

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "")
	got := p.WriteFile(path, []byte("hello"), 0o644, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unprivileged path should succeed, got: %v", got.Err)
	}
	if !got.Performed {
		t.Error("Performed should be true")
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		t.Errorf("parent should have been auto-created; stat err=%v", err)
	}
	data, _ := os.ReadFile(path)
	if string(data) != "hello" {
		t.Errorf("content = %q, want %q", string(data), "hello")
	}
}

// TestCopyFile_MissingParent_NoEscalation_UnprivilegedCreate — the
// CopyFile counterpart: no AsUser, missing parent is auto-created and
// the copy succeeds.
func TestCopyFile_MissingParent_NoEscalation_UnprivilegedCreate(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src.txt")
	if err := os.WriteFile(src, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(root, "auto", "nested")
	dest := filepath.Join(parent, "dest.txt")

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "", false, "")
	got := p.CopyFile(src, dest, 0o644, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unprivileged path should succeed, got: %v", got.Err)
	}
	if !got.Performed {
		t.Error("Performed should be true")
	}
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		t.Errorf("parent should have been auto-created; stat err=%v", err)
	}
	data, _ := os.ReadFile(dest)
	if string(data) != "payload" {
		t.Errorf("content = %q, want %q", string(data), "payload")
	}
}

// ----------------------------------------------------------------------
// Symlink
// ----------------------------------------------------------------------

func TestSymlink_Create(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(tmp, "link")

	got := newTestPerformer(actions.ModeApply).
		Symlink(target, link, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.Performed {
		t.Error("expected Performed")
	}
	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if resolved != target {
		t.Errorf("symlink target = %q, want %q", resolved, target)
	}
}

func TestSymlink_AlreadyCorrect(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	_ = os.WriteFile(target, []byte("x"), 0o644)
	link := filepath.Join(tmp, "link")
	_ = os.Symlink(target, link)

	got := newTestPerformer(actions.ModeApply).
		Symlink(target, link, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected, got %+v", got)
	}
}

func TestSymlink_NonSymlink_NoForce(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	dir := filepath.Join(tmp, "existing-dir")
	_ = os.Mkdir(dir, 0o755)

	got := newTestPerformer(actions.ModePlan).
		Symlink(target, dir, actions.PerformerOpts{Force: false})
	if got.Err == nil {
		t.Fatal("expected error when path is a directory and Force=false")
	}
	if got.WouldChange {
		t.Error("WouldChange should be false on error")
	}
}

func TestSymlink_NonSymlinkDir_Force_PlanMode(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	dir := filepath.Join(tmp, "existing-dir")
	_ = os.Mkdir(dir, 0o755)

	got := newTestPerformer(actions.ModePlan).
		Symlink(target, dir, actions.PerformerOpts{Force: true})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.WouldChange {
		t.Error("WouldChange expected")
	}
	if !strings.Contains(got.Reason, "directory") {
		t.Errorf("reason should mention 'directory', got: %q", got.Reason)
	}
	// dir must still exist — plan mode must not remove it
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("plan mode must not remove the existing directory")
	}
}

func TestSymlink_NonSymlinkFile_Force_PlanMode(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	existing := filepath.Join(tmp, "existing-file")
	_ = os.WriteFile(existing, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModePlan).
		Symlink(target, existing, actions.PerformerOpts{Force: true})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.WouldChange {
		t.Error("WouldChange expected")
	}
	if !strings.Contains(got.Reason, "file") {
		t.Errorf("reason should mention 'file', got: %q", got.Reason)
	}
}

func TestSymlink_WrongTarget_Force_PlanMode(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target-new")
	existing := filepath.Join(tmp, "existing-link")
	_ = os.Symlink(filepath.Join(tmp, "target-old"), existing)

	got := newTestPerformer(actions.ModePlan).
		Symlink(target, existing, actions.PerformerOpts{Force: true})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.WouldChange {
		t.Error("WouldChange expected for symlink with wrong target")
	}
}

func TestSymlink_Absent_PlanMode(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target")
	link := filepath.Join(tmp, "new-link")

	got := newTestPerformer(actions.ModePlan).
		Symlink(target, link, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.WouldChange {
		t.Error("WouldChange expected for absent path")
	}
}

// ----------------------------------------------------------------------
// Remove
// ----------------------------------------------------------------------

func TestRemove_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doomed")
	_ = os.WriteFile(path, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeApply).
		Remove(path, false, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone")
	}
}

func TestRemove_AlreadyAbsent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-existed")
	got := newTestPerformer(actions.ModeApply).
		Remove(path, false, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected, got %+v", got)
	}
}

func TestRemove_Recursive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "file"), []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeApply).
		Remove(dir, true, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("tree should be gone")
	}
}

// ----------------------------------------------------------------------
// Chmod
// ----------------------------------------------------------------------

func TestChmod_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(path, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeApply).
		Chmod(path, 0o644, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected for matching mode, got %+v", got)
	}
}

func TestChmod_Changes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(path, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeApply).
		Chmod(path, 0o600, actions.PerformerOpts{})
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if !got.Performed {
		t.Error("Performed expected")
	}
	info, _ := os.Stat(path)
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode = %v, want %v", got, want)
	}
}
