package template

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
	tmpl "github.com/alehatsman/mooncake/internal/template"
)

// reverseRunContext builds an ExecutionContext for reverse tests in
// either apply or plan mode. Standalone so reverse tests don't take
// a dependency on the broader test fixtures the legacy handler tests
// use.
func reverseRunContext(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	renderer, err := tmpl.NewPongo2Renderer()
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

func applyTemplate(t *testing.T, h *Handler, step *config.Step) *executor.Result {
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

func applyReverseStep(t *testing.T, step *config.Step) {
	t.Helper()
	fh := &filehandler.Handler{}
	if _, err := fh.Run(reverseRunContext(t, false), step); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
}

// seedTemplateFile drops a minimal pongo2 template that renders to
// the supplied static string (no variables). Keeps tests focused on
// the reverse path, not on template syntax.
func seedTemplateFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("seed template: %v", err)
	}
}

// TestTemplateReverse_CreateCycle: render a brand-new dest → reverse
// step must delete it.
func TestTemplateReverse_CreateCycle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tmpl")
	dest := filepath.Join(dir, "rendered.txt")
	seedTemplateFile(t, src, "Hello world from a template\n")

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  src,
			Dest: dest,
		},
	}
	h := &Handler{}

	result := applyTemplate(t, h, step)
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("apply: dest not rendered: %v", err)
	}
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if info.Existed {
		t.Errorf("Existed=true for fresh dest; want false")
	}

	reverseStep, err := h.Reverse(nil, step, result)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if reverseStep.FileWrite == nil || reverseStep.FileWrite.State != "absent" {
		t.Fatalf("reverse step not state=absent: %+v", reverseStep.FileWrite)
	}

	applyReverseStep(t, reverseStep)
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("dest still exists after reverse: %v", err)
	}
}

// TestTemplateReverse_OverwriteCycle: pre-existing dest with content
// B, render replaces it with content A; reverse restores B with B's
// pre-apply mode. The template src is left alone.
func TestTemplateReverse_OverwriteCycle(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "new.tmpl")
	dest := filepath.Join(dir, "existing.conf")
	const original = "[server]\nlisten = 8080\n# operator's prior config\n"

	seedTemplateFile(t, src, "[server]\nlisten = 9090\n")
	if err := os.WriteFile(dest, []byte(original), 0o640); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  src,
			Dest: dest,
		},
	}
	h := &Handler{}

	result := applyTemplate(t, h, step)
	got, _ := os.ReadFile(dest)
	if !strings.Contains(string(got), "9090") {
		t.Fatalf("apply: dest not rendered with new content: %q", got)
	}
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if !info.Existed || info.Kind != "file" {
		t.Fatalf("info: %+v", info)
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
		t.Errorf("after reverse: %q, want %q", got, original)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(dest)
		if mode := st.Mode().Perm(); mode != 0o640 {
			t.Errorf("after reverse: mode = %v, want 0640", mode)
		}
	}
}

// TestTemplateReverse_OversizedRefused: pre-existing dest above the
// snapshot cap → Reverse refuses with the size-limit message.
func TestTemplateReverse_OversizedRefused(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "small.tmpl")
	dest := filepath.Join(dir, "huge.bin")
	seedTemplateFile(t, src, "small")

	big := make([]byte, filehandler.MaxReverseCaptureBytes+1)
	if err := os.WriteFile(dest, big, 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	step := &config.Step{
		FileTemplate: &config.Template{Src: src, Dest: dest},
	}
	h := &Handler{}

	result := applyTemplate(t, h, step)
	info := result.ReverseData.(*filehandler.FileReverseInfo)
	if info.Content != nil {
		t.Fatalf("Content should be nil for oversized; got %d bytes", len(info.Content))
	}

	_, err := h.Reverse(nil, step, result)
	if err == nil {
		t.Fatal("Reverse returned nil error for oversized capture")
	}
	if !strings.Contains(err.Error(), "too large") {
		t.Errorf("error %q should mention size cap", err.Error())
	}
}

// TestTemplateReverse_NoCaptureInPlanMode mirrors the file.write
// and file.copy plan-mode checks: plan must not stash ReverseData.
func TestTemplateReverse_NoCaptureInPlanMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tmpl")
	dest := filepath.Join(dir, "would-be-dest.txt")
	seedTemplateFile(t, src, "x")

	step := &config.Step{
		FileTemplate: &config.Template{Src: src, Dest: dest},
	}
	h := &Handler{}

	res, err := h.Run(reverseRunContext(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.ReverseData != nil {
		t.Errorf("plan mode populated ReverseData: %+v", r.ReverseData)
	}
	if _, err := h.Reverse(nil, step, r); err == nil {
		t.Fatal("Reverse(plan-mode result) should error")
	}
}

func TestTemplateHandler_ImplementsReverser(t *testing.T) {
	var _ actions.Reverser = (*Handler)(nil)
	if !actions.IsReverser(&Handler{}) {
		t.Error("actions.IsReverser((*Handler)) = false; want true")
	}
}
