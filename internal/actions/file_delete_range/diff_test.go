package file_delete_range

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

// TestDiff_TextDeleteRange_UpdateWithBeforeSnapshot — same shape as the
// sibling text.* tests: Diff is conservative; the after-sha256 is
// intentionally empty because simulating the anchor-pair match against
// the existing content is too complex to merit duplicating here.
func TestDiff_TextDeleteRange_UpdateWithBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(path, []byte("BEGIN\nmiddle\nEND\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{TextDeleteRange: &config.FileDeleteRange{
		Path: path, StartAnchor: "BEGIN", EndAnchor: "END",
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
}

func TestDiff_TextDeleteRange_NilStep(t *testing.T) {
	if _, err := (&Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (&Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_TextDeleteRange_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
