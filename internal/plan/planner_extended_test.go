package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestExpandVars tests variable expansion at plan time
func TestExpandVars(t *testing.T) {
	planner := &Planner{
		template: mustNewRenderer(),
	}

	vars := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	step := config.Step{
		Vars: &vars,
	}

	ctx := &ExpansionContext{
		Variables: make(map[string]interface{}),
	}

	err := planner.expandVars(step, ctx)
	if err != nil {
		t.Fatalf("expandVars failed: %v", err)
	}

	if ctx.Variables["key1"] != "value1" {
		t.Errorf("Variables[key1] = %v, want 'value1'", ctx.Variables["key1"])
	}
	if ctx.Variables["key2"] != 42 {
		t.Errorf("Variables[key2] = %v, want 42", ctx.Variables["key2"])
	}
}

// TestExpandVars_WithTemplate tests variable expansion with template rendering
func TestExpandVars_WithTemplate(t *testing.T) {
	planner := &Planner{
		template: mustNewRenderer(),
	}

	vars := map[string]interface{}{
		"greeting": "Hello {{ name }}",
		"number":   123,
	}

	step := config.Step{
		Vars: &vars,
	}

	ctx := &ExpansionContext{
		Variables: map[string]interface{}{
			"name": "World",
		},
	}

	err := planner.expandVars(step, ctx)
	if err != nil {
		t.Fatalf("expandVars failed: %v", err)
	}

	if ctx.Variables["greeting"] != "Hello World" {
		t.Errorf("Variables[greeting] = %v, want 'Hello World'", ctx.Variables["greeting"])
	}
}

// TestExpandVars_NilVars tests error handling for nil vars
func TestExpandVars_NilVars(t *testing.T) {
	planner := &Planner{
		template: mustNewRenderer(),
	}

	step := config.Step{
		Vars: nil,
	}

	ctx := &ExpansionContext{
		Variables: make(map[string]interface{}),
	}

	err := planner.expandVars(step, ctx)
	if err == nil {
		t.Error("expandVars should return error for nil Vars")
	}
}

// TestExpandIncludeVars tests loading variables from external file
func TestExpandIncludeVars(t *testing.T) {
	// Create temporary vars file
	tmpDir := t.TempDir()
	varsFile := filepath.Join(tmpDir, "vars.yml")

	varsContent := `
test_var: test_value
number_var: 42
`
	err := os.WriteFile(varsFile, []byte(varsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create vars file: %v", err)
	}

	planner := &Planner{
		template: mustNewRenderer(),
	}

	step := config.Step{
		VarsLoad: &varsFile,
	}

	ctx := &ExpansionContext{
		Variables:  make(map[string]interface{}),
		CurrentDir: tmpDir,
	}

	err = planner.expandIncludeVars(step, ctx)
	if err != nil {
		t.Fatalf("expandIncludeVars failed: %v", err)
	}

	if ctx.Variables["test_var"] != "test_value" {
		t.Errorf("Variables[test_var] = %v, want 'test_value'", ctx.Variables["test_var"])
	}
}

// TestExpandIncludeVars_WithTemplate tests include_vars with path template
func TestExpandIncludeVars_WithTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	varsFile := filepath.Join(tmpDir, "env-prod.yml")

	varsContent := `
environment: production
`
	err := os.WriteFile(varsFile, []byte(varsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create vars file: %v", err)
	}

	planner := &Planner{
		template: mustNewRenderer(),
	}

	// Use template in path
	varsPath := filepath.Join(tmpDir, "env-{{ env }}.yml")
	step := config.Step{
		VarsLoad: &varsPath,
	}

	ctx := &ExpansionContext{
		Variables: map[string]interface{}{
			"env": "prod",
		},
		CurrentDir: tmpDir,
	}

	err = planner.expandIncludeVars(step, ctx)
	if err != nil {
		t.Fatalf("expandIncludeVars failed: %v", err)
	}

	if ctx.Variables["environment"] != "production" {
		t.Errorf("Variables[environment] = %v, want 'production'", ctx.Variables["environment"])
	}
}

// TestExpandIncludeVars_NilIncludeVars tests error handling for nil include_vars
func TestExpandIncludeVars_NilIncludeVars(t *testing.T) {
	planner := &Planner{
		template: mustNewRenderer(),
	}

	step := config.Step{
		VarsLoad: nil,
	}

	ctx := &ExpansionContext{
		Variables: make(map[string]interface{}),
	}

	err := planner.expandIncludeVars(step, ctx)
	if err == nil {
		t.Error("expandIncludeVars should return error for nil IncludeVars")
	}
}

// TestExpandIncludeVars_FileNotFound tests error handling for missing file
func TestExpandIncludeVars_FileNotFound(t *testing.T) {
	planner := &Planner{
		template: mustNewRenderer(),
	}

	nonexistentFile := "/nonexistent/vars.yml"
	step := config.Step{
		VarsLoad: &nonexistentFile,
	}

	ctx := &ExpansionContext{
		Variables:  make(map[string]interface{}),
		CurrentDir: "/tmp",
	}

	err := planner.expandIncludeVars(step, ctx)
	if err == nil {
		t.Error("expandIncludeVars should return error for nonexistent file")
	}
}

// TestShouldProcessAtPlanTime tests plan-time condition evaluation
func TestShouldProcessAtPlanTime(t *testing.T) {
	renderer := mustNewRenderer()
	planner := &Planner{
		template: renderer,
	}

	tests := []struct {
		name      string
		when      string
		variables map[string]interface{}
		expected  bool
	}{
		{
			"no when condition",
			"",
			map[string]interface{}{},
			true,
		},
		{
			"when true",
			"true",
			map[string]interface{}{},
			true,
		},
		{
			"when false",
			"false",
			map[string]interface{}{},
			false,
		},
		{
			"when with expression",
			"{{ count }} > 5",
			map[string]interface{}{"count": 10},
			true,
		},
		{
			"when with missing variable",
			"{{ undefined }}",
			map[string]interface{}{},
			true, // Should default to true on evaluation failure
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				When: tt.when,
			}

			ctx := &ExpansionContext{
				Variables: tt.variables,
			}

			result := planner.shouldProcessAtPlanTime(step, ctx)
			if result != tt.expected {
				t.Errorf("shouldProcessAtPlanTime() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestExpandStepsWithContext_EmptySteps tests expansion with empty steps
func TestExpandStepsWithContext_EmptySteps(t *testing.T) {
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	pathExp := pathutil.NewPathExpander(tmpl)
	planner := &Planner{
		template:    tmpl,
		pathUtil:    pathExp,
		fileTree:    filetree.NewWalker(pathExp),
		seenFiles:   make(map[string]bool),
		locationMap: make(map[int]*IncludeFrame),
	}

	expandedSteps, err := planner.ExpandStepsWithContext([]config.Step{}, map[string]interface{}{}, "/tmp")
	if err != nil {
		t.Fatalf("ExpandStepsWithContext failed: %v", err)
	}

	if len(expandedSteps) != 0 {
		t.Errorf("ExpandStepsWithContext returned %d steps, want 0", len(expandedSteps))
	}
}

// TestConvertToSlice tests conversion of various types to slice
func TestConvertToSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int // expected length
		wantErr  bool
	}{
		{"slice", []interface{}{"a", "b", "c"}, 3, false},
		{"empty slice", []interface{}{}, 0, false},
		{"nil", nil, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToSlice(tt.input, "test_expr")
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToSlice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(result) != tt.expected {
				t.Errorf("convertToSlice() returned slice of length %d, want %d", len(result), tt.expected)
			}
		})
	}
}

// writeComponent writes a minimal component file (props + steps) to disk.
func writeComponent(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

// TestTryExpandUse_LocalPathFullyConcrete: a local-path component with all
// templates resolvable at plan time gets expanded inline — the parent use:
// step disappears and the component's steps land in the plan.
func TestTryExpandUse_LocalPathFullyConcrete(t *testing.T) {
	tmp := t.TempDir()
	writeComponent(t, tmp, "palette.yml", `
props:
  variant:
    type: string
    default: monokai_dark
steps:
  - name: load palette vars
    log: "loading {{ props.variant }}"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Name: "use palette",
		Use:  "./palette.yml",
		Props: map[string]interface{}{
			"variant": "monokai_dark",
		},
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: tmp,
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if !expanded {
		t.Fatal("expected expanded=true for concrete local path")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan.Steps len = %d, want 1; got: %+v", len(plan.Steps), plan.Steps)
	}
	if plan.Steps[0].Name != "load palette vars" {
		t.Errorf("emitted step name = %q, want 'load palette vars'", plan.Steps[0].Name)
	}
	// Props/parameters must NOT leak out of the use: scope.
	if _, leaked := ctx.Variables["props"]; leaked {
		t.Error("ctx.Variables[props] leaked outside the use: scope")
	}
	if _, leaked := ctx.Variables["parameters"]; leaked {
		t.Error("ctx.Variables[parameters] leaked outside the use: scope")
	}
}

// TestTryExpandUse_UnresolvedTemplateDefers: a use: path that still contains
// {{ }} after rendering (because the referenced variable hasn't been set)
// falls through with expanded=false so the default compilePlanStep path
// emits the opaque "not checkable" entry.
func TestTryExpandUse_UnresolvedTemplateDefers(t *testing.T) {
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Name: "dynamic use",
		Use:  "./{{ component_name }}.yml", // component_name absent from ctx
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{}, // empty
		CurrentDir: t.TempDir(),
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if expanded {
		t.Fatal("expected expanded=false for unresolved template")
	}
	if len(plan.Steps) != 0 {
		t.Errorf("plan.Steps should be empty (defer to caller), got %d", len(plan.Steps))
	}
}

// TestTryExpandUse_RemoteRefDefers: a remote module reference defers to apply
// time even when the string is fully concrete — the resolver is only wired up
// at apply time.
func TestTryExpandUse_RemoteRefDefers(t *testing.T) {
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Name: "remote module",
		Use:  "github.com/owner/repo@v1.0.0",
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: t.TempDir(),
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if expanded {
		t.Fatal("expected expanded=false for remote ref")
	}
}

// TestTryExpandUse_MissingComponentFile: a concrete local path that doesn't
// exist errors at plan time (loud failure beats silent deferral).
func TestTryExpandUse_MissingComponentFile(t *testing.T) {
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Name: "missing",
		Use:  "./does-not-exist.yml",
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: t.TempDir(),
	}
	plan := &Plan{}

	_, err = planner.tryExpandUse(step, ctx, plan, 0)
	if err == nil {
		t.Fatal("expected error for missing component file, got nil")
	}
}

// TestTryExpandUse_BadEnumErrorsAtPlanTime: enum/type/required violations
// surface at plan time once the use: ref + all props are concrete (instead
// of silently expanding then exploding mid-apply).
func TestTryExpandUse_BadEnumErrorsAtPlanTime(t *testing.T) {
	tmp := t.TempDir()
	writeComponent(t, tmp, "palette.yml", `
props:
  variant:
    type: string
    enum: [monokai_dark, monokai_light]
    default: monokai_dark
steps:
  - log: "{{ props.variant }}"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Use:   "./palette.yml",
		Props: map[string]interface{}{"variant": "neon_purple"},
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: tmp,
	}
	plan := &Plan{}

	_, err = planner.tryExpandUse(step, ctx, plan, 0)
	if err == nil {
		t.Fatal("expected enum-validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid value") {
		t.Errorf("error = %q, want enum failure", err.Error())
	}
}

// TestTryExpandUse_UnresolvedPropDefers: even with a concrete use: path,
// if any prop value still has {{ after rendering, defer the whole
// expansion. Avoids plan-time validation false-failing on templated
// strings (e.g. port: "{{ get_port.stdout }}").
func TestTryExpandUse_UnresolvedPropDefers(t *testing.T) {
	tmp := t.TempDir()
	writeComponent(t, tmp, "comp.yml", `
props:
  port:
    type: string
steps:
  - log: "{{ props.port }}"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Use: "./comp.yml",
		Props: map[string]interface{}{
			"port": "{{ get_port.stdout }}", // depends on register
		},
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: tmp,
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if expanded {
		t.Fatal("expected expanded=false when any prop still has {{")
	}
}

// TestTryExpandUse_PropDefaultsFillIn: caller omits an optional prop; the
// component's default is injected into the props namespace for templates
// in the component's steps.
func TestTryExpandUse_PropDefaultsFillIn(t *testing.T) {
	tmp := t.TempDir()
	writeComponent(t, tmp, "comp.yml", `
props:
  msg:
    type: string
    default: hello
steps:
  - name: greet
    log: "{{ props.msg }}"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Use: "./comp.yml",
		// No Props — caller relies on default.
	}
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: tmp,
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if !expanded {
		t.Fatal("expected expanded=true")
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan.Steps len = %d, want 1", len(plan.Steps))
	}
	if msg := plan.Steps[0].Log.Msg; msg != "hello" {
		t.Errorf("emitted log msg = %q, want 'hello' (default filled in)", msg)
	}
}
