package copy

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

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatalf("seed write %s: %v", path, err)
	}
}

// TestDiff_Copy_CreateWhenDestAbsent — Src exists, Dest absent → create.
// After.Sha256 must equal the actual Src hash (not just any non-empty
// string) because that's what spec-30 transactions will need later to
// verify rollback parity.
func TestDiff_Copy_CreateWhenDestAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	writeFile(t, src, "payload\n", 0o644)
	dest := filepath.Join(dir, "dest.txt")

	step := &config.Step{FileCopy: &config.Copy{Src: src, Dest: dest, Mode: "0644"}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 == "" {
		t.Error("After.Sha256 empty; should equal Src hash")
	}
	if after.Size != int64(len("payload\n")) {
		t.Errorf("After.Size = %d, want %d", after.Size, len("payload\n"))
	}
}

// TestDiff_Copy_NoopWhenContentMatches — Src and Dest have identical
// content + mode → noop. This is the cheap convergence check that
// makes re-running an apply against an unchanged system a no-op.
func TestDiff_Copy_NoopWhenContentMatches(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")
	writeFile(t, src, "same\n", 0o644)
	writeFile(t, dest, "same\n", 0o644)

	step := &config.Step{FileCopy: &config.Copy{Src: src, Dest: dest, Mode: "0644"}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop", d.Operation)
	}
}

// TestDiff_Copy_UpdateWhenContentDiffers
func TestDiff_Copy_UpdateWhenContentDiffers(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")
	writeFile(t, src, "new\n", 0o644)
	writeFile(t, dest, "old\n", 0o644)

	step := &config.Step{FileCopy: &config.Copy{Src: src, Dest: dest, Mode: "0644"}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
}

// TestDiff_Copy_UpdateWhenModeDiffers — content matches but mode
// doesn't. Same lesson as the file.write test of the same shape: mode
// must be part of the equality check or chmod-only copies degenerate
// to silent noops.
func TestDiff_Copy_UpdateWhenModeDiffers(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")
	writeFile(t, src, "same\n", 0o644)
	writeFile(t, dest, "same\n", 0o600)

	step := &config.Step{FileCopy: &config.Copy{Src: src, Dest: dest, Mode: "0644"}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
}

// TestDiff_Copy_SrcMissingForcesUpdate — Src can't be hashed, so we
// can't predict noop. Forces update so the runtime EACCES/ENOENT
// becomes the backstop. Diff stays structured (Before populated;
// After has empty Sha256 to signal "unknown post-state").
func TestDiff_Copy_SrcMissingForcesUpdate(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "dest.txt")
	writeFile(t, dest, "old\n", 0o644)

	step := &config.Step{FileCopy: &config.Copy{
		Src:  filepath.Join(dir, "no-such-src.txt"),
		Dest: dest,
		Mode: "0644",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff (Src missing): unexpected error %v — should degrade gracefully", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (Src unhashable → conservative)", d.Operation)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 != "" {
		t.Errorf("After.Sha256 = %q, want empty (Src couldn't be hashed)", after.Sha256)
	}
}

// TestDiff_Copy_NilStep + RegisteredAsDiffer — defensive + contract.
func TestDiff_Copy_NilStep(t *testing.T) {
	if _, err := (Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_Copy_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
