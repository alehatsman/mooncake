package effects

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// newTestPerformer returns a Performer pinned to the given mode and with
// no sudo password (sudo-aware paths return an error when exercised).
func newTestPerformer(mode actions.Mode) actions.Performer {
	return NewPerformer(func() actions.Mode { return mode }, "")
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

func TestMkdir_ModeExecute_CreatesWithMode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "newdir")
	p := newTestPerformer(actions.ModeExecute)

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
// ModeExecute and asserts the plan prediction matches the execute reality.
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

			exec := newTestPerformer(actions.ModeExecute).
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

func TestWriteFile_ModeExecute_CreatesNew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	p := newTestPerformer(actions.ModeExecute)

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
	p := newTestPerformer(actions.ModeExecute)

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

	got := newTestPerformer(actions.ModeExecute).
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

	got := newTestPerformer(actions.ModeExecute).
		Symlink(target, link, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected, got %+v", got)
	}
}

// ----------------------------------------------------------------------
// Remove
// ----------------------------------------------------------------------

func TestRemove_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doomed")
	_ = os.WriteFile(path, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeExecute).
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
	got := newTestPerformer(actions.ModeExecute).
		Remove(path, false, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected, got %+v", got)
	}
}

func TestRemove_Recursive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	_ = os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "sub", "file"), []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeExecute).
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

	got := newTestPerformer(actions.ModeExecute).
		Chmod(path, 0o644, actions.PerformerOpts{})
	if !got.AlreadyOk {
		t.Errorf("AlreadyOk expected for matching mode, got %+v", got)
	}
}

func TestChmod_Changes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	_ = os.WriteFile(path, []byte("x"), 0o644)

	got := newTestPerformer(actions.ModeExecute).
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

