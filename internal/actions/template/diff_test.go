package template

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

// diffTestCtx builds a minimal ExecutionContext in ModePlan so Diff
// gets the template engine + variable scope it needs.
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

// TestDiff_Template_CreateWhenAbsent — Dest missing, Src present →
// OpCreate with After.Sha256 populated from rendered template content.
func TestDiff_Template_CreateWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tpl")
	writeFile(t, src, "hello {{ name }}\n", 0o644)
	dest := filepath.Join(dir, "out.txt")

	step := &config.Step{FileTemplate: &config.Template{
		Src:  src,
		Dest: dest,
		Mode: "0644",
	}}
	ctx := diffTestCtx(t)
	ctx.Scope.User["name"] = "world"

	d, err := (Handler{}).Diff(ctx, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 == "" {
		t.Error("After.Sha256 empty; template render should have produced bytes")
	}
	if after.Size != int64(len("hello world\n")) {
		t.Errorf("After.Size = %d, want %d (template '{{ name }}' should render to 'world')",
			after.Size, len("hello world\n"))
	}
}

// TestDiff_Template_NoopWhenRenderedMatches — Dest exists and its
// content/mode match the rendered template → OpNoop. Confirms the
// "have I already converged" check works for templates too.
func TestDiff_Template_NoopWhenRenderedMatches(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tpl")
	writeFile(t, src, "value={{ v }}\n", 0o644)
	dest := filepath.Join(dir, "out.txt")
	writeFile(t, dest, "value=42\n", 0o644)

	step := &config.Step{FileTemplate: &config.Template{Src: src, Dest: dest, Mode: "0644"}}
	ctx := diffTestCtx(t)
	ctx.Scope.User["v"] = "42"

	d, err := (Handler{}).Diff(ctx, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (rendered == existing)", d.Operation)
	}
}

// TestDiff_Template_UpdateWhenRenderedDiffers — change a variable;
// rendered output differs; OpUpdate.
func TestDiff_Template_UpdateWhenRenderedDiffers(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tpl")
	writeFile(t, src, "value={{ v }}\n", 0o644)
	dest := filepath.Join(dir, "out.txt")
	writeFile(t, dest, "value=42\n", 0o644)

	step := &config.Step{FileTemplate: &config.Template{Src: src, Dest: dest, Mode: "0644"}}
	ctx := diffTestCtx(t)
	ctx.Scope.User["v"] = "99"

	d, err := (Handler{}).Diff(ctx, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update", d.Operation)
	}
	before := d.Before.(*filehandler.FileSnapshot)
	after := d.After.(*filehandler.FileSnapshot)
	if before.Sha256 == after.Sha256 {
		t.Error("Sha256 should differ between before/after on update")
	}
}

// TestDiff_Template_RenderErrorPropagates — a template referencing a
// missing variable / bad syntax must propagate the error from Diff so
// the user sees the problem at plan time, not as a mid-run failure.
func TestDiff_Template_RenderErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "broken.tpl")
	writeFile(t, src, "{% if x %}unterminated", 0o644) // missing {% endif %}
	dest := filepath.Join(dir, "out.txt")

	step := &config.Step{FileTemplate: &config.Template{Src: src, Dest: dest, Mode: "0644"}}
	if _, err := (Handler{}).Diff(diffTestCtx(t), step); err == nil {
		t.Error("Diff should propagate template render errors at plan time")
	}
}

// TestDiff_Template_SrcMissingDegradesGracefully — Src can't be read,
// but Diff still returns a structured response (Update + Before
// snapshot). The runtime path will surface the read error; Diff is
// "best-effort planning information".
func TestDiff_Template_SrcMissingDegradesGracefully(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.txt")

	step := &config.Step{FileTemplate: &config.Template{
		Src:  filepath.Join(dir, "does-not-exist.tpl"),
		Dest: dest,
		Mode: "0644",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err == nil {
		t.Error("Diff should report the missing-Src error to caller")
	}
	// Even on error the Resource field should be populated so a caller
	// rendering "what step?" still gets a meaningful identifier.
	if d.Resource.Identifier != dest {
		t.Errorf("Resource.Identifier = %q, want %q (must populate even on error)", d.Resource.Identifier, dest)
	}
}

// TestDiff_Template_NilStep — defensive coverage.
func TestDiff_Template_NilStep(t *testing.T) {
	if _, err := (Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

// TestDiff_Template_RegisteredAsDiffer locks in interface satisfaction.
func TestDiff_Template_RegisteredAsDiffer(t *testing.T) {
	// Use *Handler — *Handler satisfies actions.Handler (Run is on
	// pointer receiver) which is what IsDiffer's argument type
	// requires. The compile-time `var _ actions.Differ = Handler{}`
	// check in diff.go already verifies value-receiver satisfaction.
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}

// TestDiff_Template_StepVarsEvaluated — regression for #166: Diff must apply
// step-level vars (with template expressions) so the desired SHA matches what
// handler.Run will actually write.
func TestDiff_Template_StepVarsEvaluated(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.tpl")
	writeFile(t, src, `CONCURRENCY="{{ flood_concurrency }}"`, 0o644)
	dest := filepath.Join(dir, "out.txt")
	// File on disk already has the correctly-rendered content.
	writeFile(t, dest, `CONCURRENCY="8"`, 0o644)

	stepVars := map[string]interface{}{
		"flood_concurrency": "{{ props.flood_concurrency }}",
	}
	step := &config.Step{FileTemplate: &config.Template{
		Src:  src,
		Dest: dest,
		Mode: "0644",
		Vars: &stepVars,
	}}
	ctx := diffTestCtx(t)
	ctx.Scope.Props = map[string]interface{}{"flood_concurrency": "8"}

	d, err := (Handler{}).Diff(ctx, step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop — step vars must be evaluated so desired SHA matches existing content", d.Operation)
	}
}
