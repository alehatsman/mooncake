package unarchive

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

// TestDiff_Unarchive_CreateWhenDestAbsent — Dest doesn't exist →
// OpCreate. After.Kind = "directory" because that's what unarchive
// produces.
func TestDiff_Unarchive_CreateWhenDestAbsent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "extracted")

	step := &config.Step{FileUnarchive: &config.Unarchive{
		Src: filepath.Join(dir, "bundle.tar"), Dest: dest, Mode: "0755",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Kind != "directory" {
		t.Errorf("After.Kind = %q, want directory", after.Kind)
	}
}

// TestDiff_Unarchive_UpdateWhenDestExists — Dest dir already there
// (no Creates marker). Extraction will add/replace files → OpUpdate.
func TestDiff_Unarchive_UpdateWhenDestExists(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "extracted")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileUnarchive: &config.Unarchive{
		Src: filepath.Join(dir, "bundle.tar"), Dest: dest, Mode: "0755",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
}

// TestDiff_Unarchive_NoopWhenCreatesMarkerExists — the canonical
// idempotency-marker case. step.Creates points at a path that exists
// → OpNoop, even when Dest is empty. Mirrors the runtime handler's
// "skip extraction if Creates exists" behaviour so plan and apply
// agree.
func TestDiff_Unarchive_NoopWhenCreatesMarkerExists(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "extracted")
	marker := filepath.Join(dir, "marker.txt")
	if err := os.WriteFile(marker, []byte("done\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileUnarchive: &config.Unarchive{
		Src:     filepath.Join(dir, "bundle.tar"),
		Dest:    dest,
		Creates: marker,
		Mode:    "0755",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (Creates marker exists)", d.Operation)
	}
}

// TestDiff_Unarchive_CreatesMarkerMissingDoesNotForceNoop — the
// complement of the test above. When Creates is set but the marker
// is missing, fall through to the regular Operation logic (Create
// or Update based on Dest existence).
func TestDiff_Unarchive_CreatesMarkerMissingDoesNotForceNoop(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "extracted")

	step := &config.Step{FileUnarchive: &config.Unarchive{
		Src:     filepath.Join(dir, "bundle.tar"),
		Dest:    dest,
		Creates: filepath.Join(dir, "no-such-marker.txt"),
		Mode:    "0755",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create (Creates missing → fall through, Dest also missing)", d.Operation)
	}
}

// TestDiff_Unarchive_ResourceIsDestNotSrc — Src is a read-only input;
// it MUST NOT appear in the Diff's Resource ref. Locks in the spec-22
// symmetry with PermissionSet (which already excludes Src from
// FilesystemWrite).
func TestDiff_Unarchive_ResourceIsDestNotSrc(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "bundle.tar")
	dest := filepath.Join(dir, "extracted")

	step := &config.Step{FileUnarchive: &config.Unarchive{Src: src, Dest: dest}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Resource.Identifier == src {
		t.Errorf("Resource.Identifier = Src %q; must be Dest %q", src, dest)
	}
	if d.Resource.Identifier != dest {
		t.Errorf("Resource.Identifier = %q, want %q", d.Resource.Identifier, dest)
	}
}

func TestDiff_Unarchive_NilStep(t *testing.T) {
	if _, err := (Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_Unarchive_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
