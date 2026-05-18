package file_replace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
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

// TestRun_NoChange: file already contains replacement (or pattern absent)
// → already-ok, no write.
func TestRun_NoChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(path, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextReplace: &config.FileReplace{
			Path:         path,
			Pattern:      "absent_pattern",
			Replace:      "X",
			AllowNoMatch: true,
		},
	}
	h := &Handler{}

	// Plan: no change predicted.
	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("plan: WouldChange should be false; reason=%q", r.Reason)
	}

	// Execute: also no change.
	res, err = h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	r = res.(*executor.Result)
	if r.Changed {
		t.Errorf("execute: Changed should be false; reason=%q", r.Reason)
	}
}

// TestRun_WouldReplace: file has the pattern → plan predicts change,
// execute performs it, and the prediction matches what gets written.
func TestRun_WouldReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	original := "before old after\nmore old text\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		TextReplace: &config.FileReplace{
			Path:    path,
			Pattern: "old",
			Replace: "new",
		},
	}
	h := &Handler{}

	// Plan: change predicted; file untouched.
	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan: WouldChange should be true; reason=%q", r.Reason)
	}
	cur, _ := os.ReadFile(path)
	if string(cur) != original {
		t.Fatal("plan must not modify the file")
	}

	// Execute: replacement happens, content matches what plan predicted.
	res, err = h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	r = res.(*executor.Result)
	if !r.Changed {
		t.Error("execute: Changed should be true")
	}
	cur, _ = os.ReadFile(path)
	if got, want := string(cur), "before new after\nmore new text\n"; got != want {
		t.Errorf("execute wrote %q, want %q", got, want)
	}
}

// TestRun_NoMatchIsIdempotentSuccess: pattern absent and the action
// is read-only. MT-47 flipped the semantics — no-match is no longer
// a failure; it's the idempotent success path (the second run of a
// playbook that already replaced the pattern). Both plan and execute
// must return without error, with Changed=false.
func TestRun_NoMatchIsIdempotentSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f.txt")
	original := []byte("hello\n")
	_ = os.WriteFile(path, original, 0o644)
	step := &config.Step{
		TextReplace: &config.FileReplace{
			Path:    path,
			Pattern: "missing",
			Replace: "X",
		},
	}
	h := &Handler{}

	if _, err := h.Run(newCtx(t, true), step); err != nil {
		t.Errorf("plan: expected no error on no-match; got %v", err)
	}
	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Errorf("execute: expected no error on no-match; got %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(original) {
		t.Errorf("content mutated despite no match: %q", got)
	}
}

// TestRun_ImplementsRunner: the handler still satisfies the Runner
// interface after the upgrade.
func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}
