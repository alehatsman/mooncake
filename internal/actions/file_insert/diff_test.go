package file_insert

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
			Evaluator:      mock.GetEvaluator(),
			Logger:         mock.Log,
			EventPublisher: mock.Publisher,
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Mode:           actions.ModePlan,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: mock.StepID,
		CurrentDir:    "/tmp",
	}
}

// TestDiff_TextInsert_UpdateWithBeforeSnapshot — file exists, anchor-
// based insert is planned. Diff stays conservative (After.Sha256 empty)
// because predicting the anchor match + insert outcome requires
// running the action logic, which Diff intentionally skips.
func TestDiff_TextInsert_UpdateWithBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("# section start\n\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{TextInsert: &config.FileInsert{
		Path: path, Anchor: "section start", Position: "after", Content: "new line\n",
	}}
	d, err := (&Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
	if !d.Before.(*filehandler.FileSnapshot).Exists {
		t.Error("Before.Exists = false; want true")
	}
	if d.After.(*filehandler.FileSnapshot).Sha256 != "" {
		t.Error("After.Sha256 must be empty; Diff does not simulate the insert")
	}
}

func TestDiff_TextInsert_NilStep(t *testing.T) {
	if _, err := (&Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (&Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_TextInsert_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
