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

// TestReverse_OverwriteRefusesInSliceA — when the target existed
// pre-apply, slice A cannot reverse (would need pre-write content,
// captured in slice B). Reverse must return an explicit error
// pointing at slice B so transaction implementers know what's
// coming.
func TestReverse_OverwriteRefusesInSliceA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "existing.txt")
	writeFile(t, path, []byte("ORIGINAL"), 0o644)
	step := &config.Step{
		FileWrite: &config.File{
			Path:    path,
			Content: "OVERWRITTEN",
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	info := result.ReverseData.(*FileReverseInfo)
	if !info.Existed {
		t.Fatalf("captured Existed=false; want true (we seeded the file)")
	}
	if info.Kind != "file" {
		t.Errorf("captured Kind=%q, want \"file\"", info.Kind)
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse returned nil error for existing-file overwrite; want slice-B refusal")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "phase 5b") {
		t.Errorf("error %q should mention phase 5b so transaction implementers know what's coming", err.Error())
	}
}

// TestReverse_DeleteRefusesInSliceA — reversing a deletion needs
// the captured pre-delete content (slice B). Without it Reverse
// must refuse explicitly.
func TestReverse_DeleteRefusesInSliceA(t *testing.T) {
	path := filepath.Join(t.TempDir(), "to-delete.txt")
	writeFile(t, path, []byte("doomed"), 0o644)
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
	if err == nil {
		t.Fatal("Reverse returned nil error for delete; want slice-B refusal")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step: %+v", reverseStep)
	}
}

// TestReverse_LinkRefuses — the link/hardlink family is slice C
// territory. Slice A must refuse explicitly.
func TestReverse_LinkRefuses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need admin or developer mode on Windows; not the point of this test")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link")
	writeFile(t, target, []byte("x"), 0o644)

	step := &config.Step{
		FileWrite: &config.File{
			Path:  link,
			State: "link",
			Src:   target,
		},
	}
	h := &Handler{}

	result := applyStep(t, h, step)
	_, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse returned nil error for state=link; want slice-C refusal")
	}
	if !strings.Contains(err.Error(), "phase 5c") {
		t.Errorf("error %q should mention phase 5c so future work is discoverable", err.Error())
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
