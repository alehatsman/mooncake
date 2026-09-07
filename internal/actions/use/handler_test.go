package use

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/components"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/template"
)

// mustNewRenderer creates a renderer or panics
func mustNewRenderer() template.Renderer {
	r, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return r
}

// setupTestComponents creates temporary component files for testing
func setupTestComponents(t *testing.T) (cleanup func()) {
	t.Helper()

	// Create components in ./components directory (first search path)
	componentsDir := "./components"

	// Check if components directory exists, create if not
	needsCleanup := false
	if _, err := os.Stat(componentsDir); os.IsNotExist(err) {
		if err := os.MkdirAll(componentsDir, 0755); err != nil {
			t.Fatalf("Failed to create components directory: %v", err)
		}
		needsCleanup = true
	}

	// Create a simple component with no parameters
	simpleComponent := `name: simple-test
description: Simple test component
version: 1.0.0
steps:
  - name: Print message
    log:
      msg: "Hello from component"
`
	simpleFile := filepath.Join(componentsDir, "simple-test.yml")
	if err := os.WriteFile(simpleFile, []byte(simpleComponent), 0644); err != nil {
		t.Fatalf("Failed to create simple component: %v", err)
	}

	// Create a component with parameters
	propComponent := `name: param-test
description: Test component with parameters
version: 1.0.0
props:
  message:
    type: string
    required: true
    description: Message to print
  count:
    type: string
    required: false
    default: "1"
    description: Number of times to print
steps:
  - name: Print with parameter
    log:
      msg: "{{ parameters.message }}"
  - name: Print count
    log:
      msg: "Count: {{ parameters.count }}"
`
	paramFile := filepath.Join(componentsDir, "param-test.yml")
	if err := os.WriteFile(paramFile, []byte(propComponent), 0644); err != nil {
		t.Fatalf("Failed to create param component: %v", err)
	}

	// Create a component with enum parameter
	enumComponent := `name: enum-test
description: Test component with enum parameter
version: 1.0.0
props:
  state:
    type: string
    required: true
    enum: [present, absent]
    description: Desired state
steps:
  - name: Print state
    log:
      msg: "State: {{ parameters.state }}"
`
	enumFile := filepath.Join(componentsDir, "enum-test.yml")
	if err := os.WriteFile(enumFile, []byte(enumComponent), 0644); err != nil {
		t.Fatalf("Failed to create enum component: %v", err)
	}

	// Create a component with multiple steps
	multiStepComponent := `name: multi-step-test
description: Test component with multiple steps
version: 1.0.0
steps:
  - name: Step 1
    log:
      msg: "Step 1"
  - name: Step 2
    log:
      msg: "Step 2"
  - name: Step 3
    log:
      msg: "Step 3"
`
	multiStepFile := filepath.Join(componentsDir, "multi-step-test.yml")
	if err := os.WriteFile(multiStepFile, []byte(multiStepComponent), 0644); err != nil {
		t.Fatalf("Failed to create multi-step component: %v", err)
	}

	// Return cleanup function
	return func() {
		os.Remove(simpleFile)
		os.Remove(paramFile)
		os.Remove(enumFile)
		os.Remove(multiStepFile)
		if needsCleanup {
			os.RemoveAll(componentsDir)
		}
	}
}

// mockExecutionContext creates a mock ExecutionContext for testing
func mockExecutionContext(variables map[string]interface{}) *executor.ExecutionContext {
	scope := executor.NewVariableScope()
	if variables != nil {
		scope.User = variables
	}

	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       mustNewRenderer(),
			EventPublisher: &testutil.MockPublisher{Events: []events.Event{}},
			Logger:         &testutil.MockLogger{Logs: []string{}},
			Evaluator:      expression.NewExprEvaluator(),
			Mode:           actions.ModeApply,
		},
		Scope:      scope,
		CurrentDir: ".",
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "use" {
		t.Errorf("Name = %v, want 'component'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != "system" {
		t.Errorf("Category = %v, want 'system'", meta.Category)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
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
			name: "valid component action with name only",
			step: &config.Step{
				Use: "test-component",
			},
			wantErr: false,
		},
		{
			name: "valid component action with parameters",
			step: &config.Step{
				Use: "test-component",
				Props: map[string]interface{}{
					"param1": "value1",
				},
			},
			wantErr: false,
		},
		{
			name: "unset component action",
			step: &config.Step{
				Use: "",
			},
			wantErr: true,
		},
		{
			name: "empty component name",
			step: &config.Step{
				Use: "",
			},
			wantErr: true,
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

func TestHandler_Execute_InvalidContextType(t *testing.T) {
	h := &Handler{}

	// Use MockContext instead of ExecutionContext
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Name: "Test invalid context",
		Use:  "simple-test",
	}

	_, err := h.Run(ctx, step)
	if err == nil {
		t.Fatal("Execute() should error for invalid context type")
	}

	if !contains(err.Error(), "invalid context type") {
		t.Errorf("Error message should mention invalid context type, got: %v", err)
	}
}

func TestHandler_Execute_NonexistentComponent(t *testing.T) {
	cleanup := setupTestComponents(t)
	defer cleanup()

	h := &Handler{}
	ec := mockExecutionContext(nil)

	step := &config.Step{
		Name: "Test nonexistent component",
		Use:  "does-not-exist",
	}

	_, err := h.Run(ec, step)
	if err == nil {
		t.Fatal("Execute() should error for nonexistent component")
	}

	if !contains(err.Error(), "does-not-exist") {
		t.Errorf("Error message should mention component name, got: %v", err)
	}
}

// TestComponentExpansion tests that components can be expanded without executing them
func TestComponentExpansion(t *testing.T) {
	cleanup := setupTestComponents(t)
	defer cleanup()

	tests := []struct {
		name           string
		componentName  string
		props          map[string]interface{}
		wantSteps      int
		wantErr        bool
		checkParameter string
		expectedValue  interface{}
	}{
		{
			name:          "simple component expansion",
			componentName: "simple-test",
			wantSteps:     1,
			wantErr:       false,
		},
		{
			name:          "component with parameters",
			componentName: "param-test",
			props: map[string]interface{}{
				"message": "Test message",
				"count":   "5",
			},
			wantSteps:      2,
			wantErr:        false,
			checkParameter: "message",
			expectedValue:  "Test message",
		},
		{
			name:          "component with default parameter",
			componentName: "param-test",
			props: map[string]interface{}{
				"message": "Test message",
				// count should use default "1"
			},
			wantSteps:      2,
			wantErr:        false,
			checkParameter: "count",
			expectedValue:  "1",
		},
		{
			name:          "component with enum parameter - valid",
			componentName: "enum-test",
			props: map[string]interface{}{
				"state": "present",
			},
			wantSteps: 1,
			wantErr:   false,
		},
		{
			name:          "component with enum parameter - invalid",
			componentName: "enum-test",
			props: map[string]interface{}{
				"state": "invalid",
			},
			wantSteps: 0,
			wantErr:   true,
		},
		{
			name:          "missing required parameter",
			componentName: "param-test",
			props: map[string]interface{}{
				// Missing "message" which is required
				"count": "3",
			},
			wantSteps: 0,
			wantErr:   true,
		},
		{
			name:          "multi-step component",
			componentName: "multi-step-test",
			wantSteps:     3,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			steps, params, _, err := components.ExpandComponent(tt.componentName, tt.props)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandComponent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			if len(steps) != tt.wantSteps {
				t.Errorf("ExpandComponent() returned %d steps, want %d", len(steps), tt.wantSteps)
			}

			if tt.checkParameter != "" {
				if paramsMap, ok := params["props"].(map[string]interface{}); ok {
					if val, exists := paramsMap[tt.checkParameter]; !exists {
						t.Errorf("Parameter %s not found in expanded parameters", tt.checkParameter)
					} else if val != tt.expectedValue {
						t.Errorf("Parameter %s = %v, want %v", tt.checkParameter, val, tt.expectedValue)
					}
				} else {
					t.Error("props namespace not found or invalid type")
				}
			}
		})
	}
}

func TestCaptureContext(t *testing.T) {
	ec := mockExecutionContext(map[string]interface{}{
		"var1": "value1",
		"var2": 42,
	})
	ec.CurrentDir = "/test/dir"

	saved := captureContext(ec)

	// Verify captured state
	if len(saved.variables) != 2 {
		t.Errorf("Saved variables count = %d, want 2", len(saved.variables))
	}
	if saved.variables["var1"] != "value1" {
		t.Errorf("Saved variable var1 = %v, want 'value1'", saved.variables["var1"])
	}
	if saved.currentDir != "/test/dir" {
		t.Errorf("Saved currentDir = %s, want '/test/dir'", saved.currentDir)
	}
}

func TestSavedContext_Restore(t *testing.T) {
	// Create context with original state
	ec := mockExecutionContext(map[string]interface{}{
		"original": "value",
	})
	ec.CurrentDir = "/original/dir"

	// Capture state
	saved := captureContext(ec)

	// Modify context (simulate component execution)
	ec.Scope.User["original"] = "modified"
	ec.Scope.User["new_var"] = "new_value"
	parametersNamespace := map[string]interface{}{
		"parameters": map[string]interface{}{
			"param1": "value1",
		},
	}
	for k, v := range parametersNamespace {
		ec.Scope.User[k] = v
	}
	ec.CurrentDir = "/modified/dir"

	// Restore state
	saved.restore(ec, parametersNamespace)

	// Verify restoration
	if len(ec.Scope.User) != 1 {
		t.Errorf("Scope.User count after restore = %d, want 1", len(ec.Scope.User))
	}
	if ec.Scope.User["original"] != "value" {
		t.Errorf("Variable 'original' = %v, want 'value'", ec.Scope.User["original"])
	}
	if _, exists := ec.Scope.User["parameters"]; exists {
		t.Error("'parameters' namespace should be removed")
	}
	if _, exists := ec.Scope.User["new_var"]; exists {
		t.Error("'new_var' should be removed")
	}
	if ec.CurrentDir != "/original/dir" {
		t.Errorf("CurrentDir after restore = %s, want '/original/dir'", ec.CurrentDir)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Additional tests for improved coverage
//
// Note: Full Execute() tests are omitted because they require complex planner
// context setup that's beyond the scope of unit tests. The core functionality
// is tested through integration tests.

// DryRun tests removed - they require complex planner context setup
// that's beyond the scope of unit tests
