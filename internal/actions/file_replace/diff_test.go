package file_replace

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"

	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
)

func diffTestCtx(t *testing.T) *executor.ExecutionContext {
	t.Helper()
	mock := testutil.NewMockContext()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      mock.Evaluator(),
			Logger:         mock.Log,
			EventPublisher: mock.Publisher,
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Mode:           actions.ModePlan,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: mock.CurrentStepID,
		CurrentDir:    "/tmp",
	}
}

// TestDiff_TextReplace_UpdateWithBeforeSnapshot — when Path exists, Diff
// returns OpUpdate with a populated Before snapshot. After.Sha256 is
// intentionally empty because text.replace's actual output content
// depends on regex semantics we deliberately don't simulate at plan
// time.
func TestDiff_TextReplace_UpdateWithBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{TextReplace: &config.FileReplace{
		Path: path, Pattern: "hello", Replace: "hi",
	}}
	d, err := (&Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
	before := d.Before.(*filehandler.FileSnapshot)
	if !before.Exists {
		t.Error("Before.Exists = false; want true (file was seeded)")
	}
	if before.Sha256 == "" {
		t.Error("Before.Sha256 empty; want populated for existing file")
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 != "" {
		t.Errorf("After.Sha256 = %q, want empty (Diff is conservative — content not simulated)", after.Sha256)
	}
	if d.Resource.Kind != actions.ResourceFile || d.Resource.Identifier != path {
		t.Errorf("Resource = %+v, want kind=file, ident=%q", d.Resource, path)
	}
}

// TestDiff_TextReplace_PathMissingStillReturnsStructured — even if the
// target file doesn't exist (text.replace would error at runtime), Diff
// returns a meaningful structured response. The intent — "mutate this
// path" — is honest even when the path is missing.
func TestDiff_TextReplace_PathMissingStillReturnsStructured(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "not-there.txt")

	step := &config.Step{TextReplace: &config.FileReplace{
		Path: path, Pattern: "x", Replace: "y",
	}}
	d, err := (&Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (intent is mutate, runtime surfaces missing-file)", d.Operation)
	}
	before := d.Before.(*filehandler.FileSnapshot)
	if before.Exists {
		t.Error("Before.Exists = true; want false (path is absent)")
	}
}

// TestDiff_TextReplace_NilStep — defensive coverage.
func TestDiff_TextReplace_NilStep(t *testing.T) {
	if _, err := (&Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (&Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

// TestDiff_TextReplace_RegisteredAsDiffer locks in interface
// satisfaction so a future receiver narrowing breaks the build.
func TestDiff_TextReplace_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
