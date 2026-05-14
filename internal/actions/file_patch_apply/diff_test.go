package file_patch_apply

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

// TestDiff_TextPatch_UpdateWithBeforeSnapshot — Diff is conservative.
// Note that PatchFile (when used) is a read-only input on the
// controller's FS and MUST NOT appear in the Resource ref (matches
// the spec-22 PermissionSet contract for text.patch, which omits
// PatchFile from FilesystemWrite).
func TestDiff_TextPatch_UpdateWithBeforeSnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(path, []byte("alpha\nbravo\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{TextPatch: &config.FilePatchApply{
		Path:  path,
		Patch: "--- a\n+++ b\n@@\n-alpha\n+ALPHA\n",
	}}
	d, err := (&Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
	if d.Resource.Identifier != path {
		t.Errorf("Resource.Identifier = %q, want %q (mutation target — NOT PatchFile)", d.Resource.Identifier, path)
	}
	if !d.Before.(*filehandler.FileSnapshot).Exists {
		t.Error("Before.Exists = false; want true")
	}
}

// TestDiff_TextPatch_PatchFileNotInResource — when PatchFile is set,
// the Diff's Resource still refers only to Path (the mutation target),
// never PatchFile. Locks in symmetry with the spec-22 PermissionSet
// contract that excludes PatchFile from FilesystemWrite.
func TestDiff_TextPatch_PatchFileNotInResource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.txt")
	patchFile := filepath.Join(dir, "input.patch")
	if err := os.WriteFile(path, []byte("alpha\n"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if err := os.WriteFile(patchFile, []byte("--- a\n+++ b\n"), 0o644); err != nil {
		t.Fatalf("seed patch: %v", err)
	}

	step := &config.Step{TextPatch: &config.FilePatchApply{
		Path:      path,
		PatchFile: patchFile,
	}}
	d, err := (&Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Resource.Identifier == patchFile {
		t.Errorf("Resource.Identifier = PatchFile %q; must be Path %q", patchFile, path)
	}
	if d.Resource.Identifier != path {
		t.Errorf("Resource.Identifier = %q, want %q", d.Resource.Identifier, path)
	}
}

func TestDiff_TextPatch_NilStep(t *testing.T) {
	if _, err := (&Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (&Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_TextPatch_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
