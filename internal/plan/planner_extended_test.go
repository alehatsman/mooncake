package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestExpandVars tests variable expansion at plan time
func TestExpandVars(t *testing.T) {
	planner := &Planner{
		template: template.NewPongo2Renderer(),
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
		template: template.NewPongo2Renderer(),
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
		template: template.NewPongo2Renderer(),
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
		template: template.NewPongo2Renderer(),
	}

	step := config.Step{
		IncludeVars: &varsFile,
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
		template: template.NewPongo2Renderer(),
	}

	// Use template in path
	varsPath := filepath.Join(tmpDir, "env-{{ env }}.yml")
	step := config.Step{
		IncludeVars: &varsPath,
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
		template: template.NewPongo2Renderer(),
	}

	step := config.Step{
		IncludeVars: nil,
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
		template: template.NewPongo2Renderer(),
	}

	nonexistentFile := "/nonexistent/vars.yml"
	step := config.Step{
		IncludeVars: &nonexistentFile,
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
	planner := &Planner{
		template: template.NewPongo2Renderer(),
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
	tmpl := template.NewPongo2Renderer()
	pathExp := pathutil.NewPathExpander(tmpl)
	planner := &Planner{
		template: tmpl,
		pathUtil: pathExp,
		fileTree: filetree.NewWalker(pathExp),
		seenFiles: make(map[string]bool),
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
