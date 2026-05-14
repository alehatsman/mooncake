package file

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// applyStep is a tiny test helper that runs h.Run in apply mode and
// returns the *executor.Result. Fails the test on any error.
func applyStep(t *testing.T, h *Handler, step *config.Step) *executor.Result {
	t.Helper()
	res, err := h.Run(newRunContext(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("expected *executor.Result, got %T", res)
	}
	return r
}

// TestReverse_CreateFile is the apply→reverse→verify cycle for the
// canonical slice-A case: we wrote a file that did not exist, so
// Reverse returns a state=absent step targeting the same path.
// Applying that step deletes the file.
func TestReverse_CreateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: "rollback me",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	if !result.Changed {
		t.Fatalf("apply: Changed should be true for new-file create")
	}
	if !exists(t, path) {
		t.Fatal("apply: file was not created")
	}
	if result.ReverseData == nil {
		t.Fatal("ReverseData should be populated in ModeApply")
	}
	info, ok := result.ReverseData.(*FileReverseInfo)
	if !ok {
		t.Fatalf("ReverseData type = %T, want *FileReverseInfo", result.ReverseData)
	}
	if info.Existed {
		t.Errorf("captured info.Existed = true; want false (path was empty pre-apply)")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep == nil {
		t.Fatal("Reverse returned (nil, nil); want a delete step")
	}
	if reverseStep.FileWrite == nil {
		t.Fatal("reverse step has no FileWrite payload")
	}
	if reverseStep.FileWrite.Path != path {
		t.Errorf("reverse path = %q, want %q", reverseStep.FileWrite.Path, path)
	}
	if reverseStep.FileWrite.State != "absent" {
		t.Errorf("reverse state = %q, want \"absent\"", reverseStep.FileWrite.State)
	}

	// Apply the reverse step — the file must be gone after this.
	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	if exists(t, path) {
		t.Error("file still exists after applying reverse step")
	}
}

// TestReverse_CreateDirectory mirrors the file case for mkdir.
func TestReverse_CreateDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "newdir")
	step := &config.Step{
		FileWrite: &config.File{
			Path:  path,
			State: "directory",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	if !exists(t, path) {
		t.Fatal("apply: directory was not created")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.State != "absent" {
		t.Errorf("reverse state = %q, want \"absent\"", reverseStep.FileWrite.State)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	if exists(t, path) {
		t.Error("directory still exists after applying reverse step")
	}
}

// TestReverse_OverwriteCycle exercises slice B's content-snapshot
// path: we pre-seed a file with ORIGINAL, run an apply that writes
// OVERWRITTEN, then verify Reverse returns a step that — when
// applied — restores ORIGINAL with the original mode.
func TestReverse_OverwriteCycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.txt")
	const original = "ORIGINAL content lines\nthat we want back\n"
	writeFile(t, path, []byte(original), 0o640)

	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: "OVERWRITTEN",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	info := result.ReverseData.(*FileReverseInfo)
	if !info.Existed || info.Kind != "file" {
		t.Fatalf("captured info wrong: %+v", info)
	}
	if string(info.Content) != original {
		t.Fatalf("captured content = %q, want %q", info.Content, original)
	}
	if got, _ := os.ReadFile(path); string(got) != "OVERWRITTEN" {
		t.Fatalf("apply: file content = %q, want %q", got, "OVERWRITTEN")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep == nil || reverseStep.FileWrite == nil {
		t.Fatal("Reverse returned nil step")
	}
	if reverseStep.FileWrite.Content != original {
		t.Errorf("reverse step Content = %q, want %q", reverseStep.FileWrite.Content, original)
	}
	if reverseStep.FileWrite.Mode != "0640" {
		t.Errorf("reverse step Mode = %q, want \"0640\" (preserved from pre-apply)", reverseStep.FileWrite.Mode)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("after reverse: content = %q, want %q", got, original)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(path)
		if mode := st.Mode().Perm(); mode != 0o640 {
			t.Errorf("after reverse: mode = %v, want 0640", mode)
		}
	}
}

// TestReverse_DeleteCycle pre-seeds a file, applies state=absent
// to delete it, and verifies Reverse re-creates the file with the
// original bytes + mode.
func TestReverse_DeleteCycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "to-delete.txt")
	const original = "this should come back after rollback\n"
	writeFile(t, path, []byte(original), 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:  path,
			State: "absent",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	if exists(t, path) {
		t.Fatal("apply: file still exists after state=absent")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.State != actionTypeFile {
		t.Errorf("reverse step State = %q, want \"file\"", reverseStep.FileWrite.State)
	}
	if reverseStep.FileWrite.Content != original {
		t.Errorf("reverse step Content = %q, want %q", reverseStep.FileWrite.Content, original)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("after reverse: file missing: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("after reverse: content = %q, want %q", got, original)
	}
}

// TestReverse_PermsCycle pre-seeds a file with mode 0644, applies
// state=perms changing it to 0600, then verifies Reverse restores
// 0644. The content must remain untouched both ways.
func TestReverse_PermsCycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits are not meaningful on NTFS")
	}
	path := filepath.Join(t.TempDir(), "perms.txt")
	const original = "do not touch my content\n"
	writeFile(t, path, []byte(original), 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:  path,
			State: "perms",
			Mode:  "0600",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	st, _ := os.Stat(path)
	if mode := st.Mode().Perm(); mode != 0o600 {
		t.Fatalf("apply: mode = %v, want 0600", mode)
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.State != "perms" {
		t.Errorf("reverse step State = %q, want \"perms\"", reverseStep.FileWrite.State)
	}
	if reverseStep.FileWrite.Mode != "0644" {
		t.Errorf("reverse step Mode = %q, want \"0644\"", reverseStep.FileWrite.Mode)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	st, _ = os.Stat(path)
	if mode := st.Mode().Perm(); mode != 0o644 {
		t.Errorf("after reverse: mode = %v, want 0644", mode)
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("after reverse: content changed unexpectedly: %q", got)
	}
}

// TestReverse_OversizedFileRefused — a pre-existing file larger
// than MaxReverseCaptureBytes cannot be snapshotted. CaptureReverseInfo
// records the stat fields but leaves Content nil; Reverse must then
// refuse with an explicit "too large to snapshot" error rather than
// pretending to roll back.
func TestReverse_OversizedFileRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "huge.bin")
	// One byte over the cap is enough — what matters is the branch.
	big := make([]byte, MaxReverseCaptureBytes+1)
	writeFile(t, path, big, 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: "small replacement",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	info := result.ReverseData.(*FileReverseInfo)
	if info.Content != nil {
		t.Fatalf("expected Content=nil for oversized capture; got %d bytes", len(info.Content))
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse returned nil error for oversized capture; want refusal")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error %q should mention size limit so callers know why", err.Error())
	}
}

// TestReverse_CreateLinkCycle — slice C's primary case: an apply
// that creates a fresh symlink should reverse cleanly by deleting
// the link path. The target file is unaffected.
func TestReverse_CreateLinkCycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need admin or developer mode on Windows; not the point of this test")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "fresh-link")
	writeFile(t, target, []byte("target content"), 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:  link,
			State: "link",
			Src:   target,
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	if !exists(t, link) {
		t.Fatal("apply: link was not created")
	}
	info := result.ReverseData.(*FileReverseInfo)
	if info.Existed {
		t.Fatalf("captured Existed=true; want false for fresh link")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.State != "absent" {
		t.Errorf("reverse state = %q, want \"absent\"", reverseStep.FileWrite.State)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	if exists(t, link) {
		t.Error("link still exists after applying reverse step")
	}
	if !exists(t, target) {
		t.Error("reverse must not touch the link target — but target is gone")
	}
}

// TestReverse_CreateHardlinkCycle mirrors CreateLinkCycle for
// state=hardlink. Hardlinks share inodes so the "delete the link"
// reverse is the same shape.
func TestReverse_CreateHardlinkCycle(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("hardlink semantics on NTFS differ enough that the kernel-level test would diverge")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "fresh-hardlink")
	writeFile(t, target, []byte("shared inode payload"), 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:  link,
			State: "hardlink",
			Src:   target,
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	if !exists(t, link) {
		t.Fatal("apply: hardlink was not created")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.State != "absent" {
		t.Errorf("reverse state = %q, want \"absent\"", reverseStep.FileWrite.State)
	}

	if _, err := h.Run(newRunContext(t, false), reverseStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	if exists(t, link) {
		t.Error("hardlink path still exists after reverse")
	}
	if !exists(t, target) {
		t.Error("reverse deleted the target file — hardlink reverse must not break the source")
	}
}

// TestReverse_NoCaptureInPlanMode — plan mode must not stash
// ReverseData. Reverse called with a plan-mode result must fail
// loudly so transaction layers never accidentally reverse a noop.
func TestReverse_NoCaptureInPlanMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plan-only.txt")
	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: "would be written",
		},
	}
	h := &Handler{}

	res, err := h.Run(newRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.ReverseData != nil {
		t.Errorf("plan mode should not populate ReverseData; got %+v", r.ReverseData)
	}

	if _, err := h.Reverse(nil, step, r); err == nil {
		t.Fatal("Reverse(plan-mode result) should error; got nil")
	}
}

// TestHandler_ImplementsReverser is the compile-time + runtime
// capability check. Mirrors TestHandler_ImplementsRunner above.
func TestHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}

// --- tiny local helpers (kept here, not in handler test files,
// because they're only meaningful to reverse_test) ---

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	return err == nil
}
