package include_vars

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
	"gopkg.in/yaml.v3"
)

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "vars.load" {
		t.Errorf("Name = %v, want 'include_vars'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryData {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryData)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if meta.SupportsBecome {
		t.Error("SupportsBecome should be false")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventVarsLoaded) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventVarsLoaded))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid include_vars action",
			step: &config.Step{
				VarsLoad: stringPtr("vars.yml"),
			},
			wantErr: false,
		},
		{
			name: "nil include_vars action",
			step: &config.Step{
				VarsLoad: nil,
			},
			wantErr: true,
		},
		{
			name: "empty include_vars path",
			step: &config.Step{
				VarsLoad: stringPtr(""),
			},
			wantErr: true,
		},
		{
			name: "path with tilde",
			step: &config.Step{
				VarsLoad: stringPtr("~/vars.yml"),
			},
			wantErr: false,
		},
		{
			name: "absolute path",
			step: &config.Step{
				VarsLoad: stringPtr("/tmp/vars.yml"),
			},
			wantErr: false,
		},
		{
			name: "relative path",
			step: &config.Step{
				VarsLoad: stringPtr("./vars.yml"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name         string
		fileVars     map[string]interface{}
		existingVars map[string]interface{}
		wantVars     map[string]interface{}
		wantErr      bool
	}{
		{
			name: "load single variable",
			fileVars: map[string]interface{}{
				"foo": "bar",
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"foo": "bar",
			},
			wantErr: false,
		},
		{
			name: "load multiple variables",
			fileVars: map[string]interface{}{
				"var1": "value1",
				"var2": 123,
				"var3": true,
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"var1": "value1",
				"var2": 123,
				"var3": true,
			},
			wantErr: false,
		},
		{
			name: "merge with existing variables",
			fileVars: map[string]interface{}{
				"new_var": "new_value",
			},
			existingVars: map[string]interface{}{
				"existing_var": "existing_value",
			},
			wantVars: map[string]interface{}{
				"existing_var": "existing_value",
				"new_var":      "new_value",
			},
			wantErr: false,
		},
		{
			name: "override existing variable",
			fileVars: map[string]interface{}{
				"foo": "new_value",
			},
			existingVars: map[string]interface{}{
				"foo": "old_value",
			},
			wantVars: map[string]interface{}{
				"foo": "new_value",
			},
			wantErr: false,
		},
		{
			name: "load complex types",
			fileVars: map[string]interface{}{
				"array": []interface{}{"a", "b", "c"},
				"map": map[string]interface{}{
					"nested": "value",
				},
			},
			existingVars: map[string]interface{}{},
			wantVars: map[string]interface{}{
				"array": []interface{}{"a", "b", "c"},
				"map": map[string]interface{}{
					"nested": "value",
				},
			},
			wantErr: false,
		},
		{
			name:     "load empty file",
			fileVars: map[string]interface{}{},
			existingVars: map[string]interface{}{
				"existing": "value",
			},
			wantVars: map[string]interface{}{
				"existing": "value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary YAML file
			tmpFile, err := os.CreateTemp("", "vars-*.yml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write YAML content
			yamlData, err := yaml.Marshal(tt.fileVars)
			if err != nil {
				t.Fatalf("Failed to marshal YAML: %v", err)
			}
			if _, err := tmpFile.Write(yamlData); err != nil {
				t.Fatalf("Failed to write YAML: %v", err)
			}
			tmpFile.Close()

			// Create ExecutionContext (include_vars needs PathUtil)
			renderer, err := template.NewPongo2Renderer()
			if err != nil {
				panic("Failed to create renderer: " + err.Error())
			}
			pathExpander := pathutil.NewPathExpander(renderer)
			mockCtx := testutil.NewMockContext()
			mockCtx.Vars = tt.existingVars

			scope := executor.NewVariableScope()
			for k, v := range tt.existingVars {
				scope.User[k] = v
			}
			ctx := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Template:       mockCtx.Tmpl,
					EventPublisher: mockCtx.Publisher,
					Logger:         mockCtx.Log,
					PathUtil:       pathExpander,
				},
				Scope:      scope,
				CurrentDir: filepath.Dir(tmpFile.Name()),
			}

			step := &config.Step{
				VarsLoad: stringPtr(tmpFile.Name()),
			}

			result, err := h.Run(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check result properties
			execResult, ok := result.(*executor.Result)
			if !ok {
				t.Fatalf("Execute() result is not *executor.Result")
			}

			if execResult.Changed {
				t.Error("Result.Changed should be false for include_vars action")
			}

			// Check variables were loaded correctly
			for key, want := range tt.wantVars {
				got, exists := ctx.Variables()[key]
				if !exists {
					t.Errorf("Variable %q not loaded", key)
					continue
				}

				if !compareValues(got, want) {
					t.Errorf("Variable %q = %v, want %v", key, got, want)
				}
			}

			// Check event was published
			pub := mockCtx.Publisher
			if len(pub.Events) != 1 {
				t.Errorf("Expected 1 event to be published, got %d", len(pub.Events))
				return
			}

			event := pub.Events[0]
			if event.Type != events.EventVarsLoaded {
				t.Errorf("Event.Type = %v, want %v", event.Type, events.EventVarsLoaded)
			}

			varsData, ok := event.Data.(events.VarsLoadedData)
			if !ok {
				t.Fatalf("Event.Data is not events.VarsLoadedData")
			}

			if varsData.Count != len(tt.fileVars) {
				t.Errorf("VarsLoadedData.Count = %v, want %v", varsData.Count, len(tt.fileVars))
			}

			if len(varsData.Keys) != len(tt.fileVars) {
				t.Errorf("VarsLoadedData.Keys length = %v, want %v", len(varsData.Keys), len(tt.fileVars))
			}

			if varsData.FilePath != tmpFile.Name() {
				t.Errorf("VarsLoadedData.FilePath = %v, want %v", varsData.FilePath, tmpFile.Name())
			}

			if varsData.DryRun {
				t.Error("VarsLoadedData.DryRun should be false")
			}
		})
	}
}

func TestHandler_Execute_PathExpansion(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name         string
		setupVars    map[string]interface{}
		filePath     string
		wantExpanded string
		wantErr      bool
	}{
		{
			name:         "expand tilde",
			setupVars:    map[string]interface{}{},
			filePath:     "~/test-vars.yml",
			wantExpanded: filepath.Join(os.Getenv("HOME"), "test-vars.yml"),
			wantErr:      false,
		},
		{
			name: "expand variable in path",
			setupVars: map[string]interface{}{
				"config_dir": "/tmp",
			},
			filePath:     "{{ config_dir }}/vars.yml",
			wantExpanded: "/tmp/vars.yml",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create file at the expected expanded location
			if err := os.MkdirAll(filepath.Dir(tt.wantExpanded), 0755); err != nil {
				t.Fatalf("Failed to create directory: %v", err)
			}

			tmpFile, err := os.Create(tt.wantExpanded)
			if err != nil {
				t.Fatalf("Failed to create file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write minimal YAML
			fileVars := map[string]interface{}{"test": "value"}
			yamlData, _ := yaml.Marshal(fileVars)
			tmpFile.Write(yamlData)
			tmpFile.Close()

			// Create ExecutionContext
			renderer, err := template.NewPongo2Renderer()
			if err != nil {
				panic("Failed to create renderer: " + err.Error())
			}
			pathExpander := pathutil.NewPathExpander(renderer)
			mockCtx := testutil.NewMockContext()
			mockCtx.Vars = tt.setupVars

			scope := executor.NewVariableScope()
			for k, v := range tt.setupVars {
				scope.User[k] = v
			}
			ctx := &executor.ExecutionContext{
				Svc: &executor.RunServices{
					Template:       mockCtx.Tmpl,
					EventPublisher: mockCtx.Publisher,
					Logger:         mockCtx.Log,
					PathUtil:       pathExpander,
				},
				Scope:      scope,
				CurrentDir: "/tmp",
			}

			step := &config.Step{
				VarsLoad: stringPtr(tt.filePath),
			}

			result, err := h.Run(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Verify result
			execResult, ok := result.(*executor.Result)
			if !ok {
				t.Fatalf("Execute() result is not *executor.Result")
			}

			if execResult.Changed {
				t.Error("Result.Changed should be false")
			}

			// Verify variable was loaded
			if got, exists := ctx.Variables()["test"]; !exists || got != "value" {
				t.Errorf("Variable 'test' = %v, want 'value'", got)
			}

			// Verify event contains expanded path
			pub := mockCtx.Publisher
			if len(pub.Events) > 0 {
				event := pub.Events[0]
				varsData, ok := event.Data.(events.VarsLoadedData)
				if ok && varsData.FilePath != tt.wantExpanded {
					t.Errorf("VarsLoadedData.FilePath = %v, want %v", varsData.FilePath, tt.wantExpanded)
				}
			}
		})
	}
}

func TestHandler_Execute_FileNotFound(t *testing.T) {
	h := &Handler{}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	pathExpander := pathutil.NewPathExpander(renderer)
	mockCtx := testutil.NewMockContext()

	ctx := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       mockCtx.Tmpl,
			EventPublisher: mockCtx.Publisher,
			Logger:         mockCtx.Log,
			PathUtil:       pathExpander,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}

	step := &config.Step{
		VarsLoad: stringPtr("/nonexistent/vars.yml"),
	}

	_, err = h.Run(ctx, step)
	if err == nil {
		t.Error("Execute() should error when file doesn't exist")
	}
}

func TestHandler_Execute_InvalidYAML(t *testing.T) {
	h := &Handler{}

	// Create temporary file with invalid YAML
	tmpFile, err := os.CreateTemp("", "invalid-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write invalid YAML
	tmpFile.WriteString("invalid: yaml: content: [")
	tmpFile.Close()

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	pathExpander := pathutil.NewPathExpander(renderer)
	mockCtx := testutil.NewMockContext()

	ctx := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       mockCtx.Tmpl,
			EventPublisher: mockCtx.Publisher,
			Logger:         mockCtx.Log,
			PathUtil:       pathExpander,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: filepath.Dir(tmpFile.Name()),
	}

	step := &config.Step{
		VarsLoad: stringPtr(tmpFile.Name()),
	}

	_, err = h.Run(ctx, step)
	if err == nil {
		t.Error("Execute() should error with invalid YAML")
	}
}

func TestHandler_Execute_NilIncludeVars(t *testing.T) {
	h := &Handler{}

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	pathExpander := pathutil.NewPathExpander(renderer)
	mockCtx := testutil.NewMockContext()

	ctx := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       mockCtx.Tmpl,
			EventPublisher: mockCtx.Publisher,
			Logger:         mockCtx.Log,
			PathUtil:       pathExpander,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}

	step := &config.Step{
		VarsLoad: nil,
	}

	// Note: This will panic because handler doesn't check for nil before dereferencing
	// Validate() should be called first by the executor
	defer func() {
		if r := recover(); r == nil {
			t.Error("Execute() should panic when include_vars is nil (implementation doesn't check)")
		}
	}()

	h.Run(ctx, step)
}

func TestHandler_Execute_NoPublisher(t *testing.T) {
	h := &Handler{}

	// Create temporary YAML file
	tmpFile, err := os.CreateTemp("", "vars-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	fileVars := map[string]interface{}{"foo": "bar"}
	yamlData, _ := yaml.Marshal(fileVars)
	tmpFile.Write(yamlData)
	tmpFile.Close()

	// Create context without publisher
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	pathExpander := pathutil.NewPathExpander(renderer)
	mockCtx := testutil.NewMockContext()

	ctx := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       mockCtx.Tmpl,
			EventPublisher: nil, // No publisher,
			Logger:         mockCtx.Log,
			PathUtil:       pathExpander,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: filepath.Dir(tmpFile.Name()),
	}

	step := &config.Step{
		VarsLoad: stringPtr(tmpFile.Name()),
	}

	result, err := h.Run(ctx, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if execResult.Changed {
		t.Error("Result.Changed should be false")
	}

	// Check variable was still loaded
	if got, exists := ctx.Variables()["foo"]; !exists || got != "bar" {
		t.Errorf("Variable 'foo' = %v, want 'bar'", got)
	}
}

func TestHandler_Execute_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	// Use MockContext directly (not ExecutionContext)
	mockCtx := testutil.NewMockContext()

	step := &config.Step{
		VarsLoad: stringPtr("/tmp/vars.yml"),
	}

	_, err := h.Run(mockCtx, step)
	if err == nil {
		t.Error("Execute() should error when context is not an ExecutionContext")
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

// compareValues compares two values for equality (simplified version)
func compareValues(a, b interface{}) bool {
	switch av := a.(type) {
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case int:
		bv, ok := b.(int)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !compareValues(av[i], bv[i]) {
				return false
			}
		}
		return true
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k := range av {
			if !compareValues(av[k], bv[k]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}
