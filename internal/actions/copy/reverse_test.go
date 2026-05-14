package copy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// reverseRunContext mirrors the apply/plan context the file.write
// handler tests use, scoped to the copy package since it can't share
// the file package's unexported newRunContext helper. Keeps both
// handlers honest by exercising them through the same plumbing.
func reverseRunContext(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: renderer,
			PathUtil: pathutil.NewPathExpander(renderer),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

// applyCopy runs h.Run in apply mode and asserts no error. Returns
// the *executor.Result for inspection of ReverseData.
func applyCopy(t *testing.T, h *Handler, step *config.Step) *executor.Result {
	t.Helper()
	res, err := h.Run(reverseRunContext(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("expected *executor.Result, got %T", res)
	}
	return r
}

// applyReverseStep dispatches a reverse step (always a file.write)
// through the file handler. Keeps copy/reverse tests honest end-to-end:
// the inverse Step really does fold back when handed to the right
// runner, not just look reasonable on paper.
func applyReverseStep(t *testing.T, step *config.Step) {
	t.Helper()
	fh := &filehandler.Handler{}
	if _, err := fh.Run(reverseRunContext(t, false), step); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
}

// TestCopyReverse_CreateCycle covers the common case: file.copy
// drops a brand-new file at dest → Reverse must produce a step
// that deletes that file.
func TestCopyReverse_CreateCycle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "copied.txt")
	if err := os.WriteFile(src, []byte("source bytes"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  src,
			Dest: dest,
		},
	}
	h := &Handler{}

	result := applyCopy(t, h, step)
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("apply: dest not created: %v", err)
	}
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if info.Existed {
		t.Errorf("captured Existed=true for fresh dest; want false")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite == nil || reverseStep.FileWrite.State != "absent" {
		t.Fatalf("reverse step is not state=absent: %+v", reverseStep.FileWrite)
	}

	applyReverseStep(t, reverseStep)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest still exists after reverse apply: %v", err)
	}
}

// TestCopyReverse_OverwriteCycle covers the harder case: dest
// existed pre-apply with content B, file.copy replaced it with
// content A; reverse must put B back exactly, with B's pre-apply
// mode.
func TestCopyReverse_OverwriteCycle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.txt")
	dest := filepath.Join(dir, "existing.txt")
	const original = "ORIGINAL DESTINATION CONTENT\nmust come back\n"

	if err := os.WriteFile(src, []byte("NEW CONTENT (from copy)"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(dest, []byte(original), 0o640); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  src,
			Dest: dest,
		},
	}
	h := &Handler{}

	result := applyCopy(t, h, step)
	got, _ := os.ReadFile(dest)
	if !strings.HasPrefix(string(got), "NEW CONTENT") {
		t.Fatalf("apply: dest content = %q, want copy from src", got)
	}
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if !info.Existed || info.Kind != "file" {
		t.Fatalf("captured info wrong: %+v", info)
	}
	if string(info.Content) != original {
		t.Fatalf("captured Content = %q, want %q", info.Content, original)
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite.Content != original {
		t.Errorf("reverse Content = %q, want %q", reverseStep.FileWrite.Content, original)
	}

	applyReverseStep(t, reverseStep)
	got, _ = os.ReadFile(dest)
	if string(got) != original {
		t.Errorf("after reverse: content = %q, want %q", got, original)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(dest)
		if mode := st.Mode().Perm(); mode != 0o640 {
			t.Errorf("after reverse: mode = %v, want 0640 (pre-apply value)", mode)
		}
	}
}

// TestCopyReverse_OversizedRefused matches file.write's slice-B
// behavior: pre-existing dest larger than MaxReverseCaptureBytes
// triggers a "too large" refusal from Reverse.
func TestCopyReverse_OversizedRefused(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.txt")
	dest := filepath.Join(dir, "huge.bin")

	if err := os.WriteFile(src, []byte("small replacement"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	big := make([]byte, filehandler.MaxReverseCaptureBytes+1)
	if err := os.WriteFile(dest, big, 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  src,
			Dest: dest,
		},
	}
	h := &Handler{}

	result := applyCopy(t, h, step)
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if info.Content != nil {
		t.Fatalf("expected Content=nil for oversized capture; got %d bytes", len(info.Content))
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse returned nil error for oversized capture")
	}
	if reverseStep != nil {
		t.Errorf("Reverse returned a step despite refusal: %+v", reverseStep)
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error %q should mention the size cap", err.Error())
	}
}

// TestCopyReverse_NoCaptureInPlanMode — plan mode does not mutate,
// so it must not stash ReverseData either. Calling Reverse on a
// plan-mode result must therefore fail explicitly.
func TestCopyReverse_NoCaptureInPlanMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "would-be-dest.txt")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}

	step := &config.Step{
		FileCopy: &config.Copy{Src: src, Dest: dest},
	}
	h := &Handler{}

	res, err := h.Run(reverseRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.ReverseData != nil {
		t.Errorf("plan mode populated ReverseData unexpectedly: %+v", r.ReverseData)
	}
	if _, err := h.Reverse(nil, step, r); err == nil {
		t.Fatal("Reverse(plan-mode result) should error; got nil")
	}
}

// TestCopyHandler_ImplementsReverser confirms the spec-22 Reverser
// contract is wired at compile time and runtime.
func TestCopyHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}
