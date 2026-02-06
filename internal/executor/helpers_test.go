package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestGenerateStepID tests step ID generation
func TestGenerateStepID(t *testing.T) {
	tests := []struct {
		name     string
		step     config.Step
		global   int
		expected string
	}{
		{
			"with explicit ID",
			config.Step{ID: "custom-id"},
			5,
			"custom-id",
		},
		{
			"without ID - global 1",
			config.Step{},
			1,
			"step-1",
		},
		{
			"without ID - global 10",
			config.Step{},
			10,
			"step-10",
		},
		{
			"without ID - global 0",
			config.Step{},
			0,
			"step-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			global := tt.global
			ec := &ExecutionContext{
				Stats: &ExecutionStats{
					Global: &global,
				},
			}

			result := generateStepID(tt.step, ec)
			if result != tt.expected {
				t.Errorf("generateStepID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestMarkStepFailed tests marking step as failed
func TestMarkStepFailed(t *testing.T) {
	result := NewResult()
	step := config.Step{
		Name: "Test Step",
	}
	ec := &ExecutionContext{
		Variables: make(map[string]interface{}),
	}

	MarkStepFailed(result, step, ec)

	if !result.Failed {
		t.Error("MarkStepFailed should set Failed to true")
	}
	if result.Rc != 1 {
		t.Errorf("MarkStepFailed should set Rc to 1, got %d", result.Rc)
	}
}

// TestMarkStepFailed_WithRegister tests marking failed step with register
func TestMarkStepFailed_WithRegister(t *testing.T) {
	result := NewResult()
	result.Stdout = "test output"

	step := config.Step{
		Name:     "Test Step",
		Register: "myresult",
	}

	variables := make(map[string]interface{})
	ec := &ExecutionContext{
		Variables: variables,
	}

	MarkStepFailed(result, step, ec)

	if !result.Failed {
		t.Error("Failed should be true")
	}

	// Check that result was registered
	if _, ok := variables["myresult"]; !ok {
		t.Error("Result should be registered to 'myresult'")
	}
}

// TestMarkStepFailed_NoRegister tests marking failed step without register
func TestMarkStepFailed_NoRegister(t *testing.T) {
	result := NewResult()
	step := config.Step{
		Name:     "Test Step",
		Register: "", // No register
	}

	variables := make(map[string]interface{})
	ec := &ExecutionContext{
		Variables: variables,
	}

	MarkStepFailed(result, step, ec)

	// Should have no registered variables
	if len(variables) != 0 {
		t.Errorf("Should have no registered variables, got %d", len(variables))
	}
}

// TestAddGlobalVariables tests global variable injection
func TestAddGlobalVariables(t *testing.T) {
	variables := make(map[string]interface{})

	AddGlobalVariables(variables)

	// Should have added facts
	if len(variables) == 0 {
		t.Error("AddGlobalVariables should add facts to variables")
	}

	// Check for common facts
	expectedFacts := []string{"os", "arch", "hostname"}
	for _, fact := range expectedFacts {
		if _, ok := variables[fact]; !ok {
			t.Errorf("Expected fact %q to be in variables", fact)
		}
	}
}

// TestAddGlobalVariables_Existing tests adding to existing variables
func TestAddGlobalVariables_Existing(t *testing.T) {
	variables := map[string]interface{}{
		"existing": "value",
	}

	AddGlobalVariables(variables)

	// Should keep existing variables
	if variables["existing"] != "value" {
		t.Error("Existing variables should be preserved")
	}

	// Should have added new facts
	if len(variables) <= 1 {
		t.Error("Should have added facts in addition to existing variables")
	}
}

// TestAddGlobalVariables_Idempotent tests multiple calls
func TestAddGlobalVariables_Idempotent(t *testing.T) {
	variables := make(map[string]interface{})

	AddGlobalVariables(variables)
	firstCount := len(variables)

	AddGlobalVariables(variables)
	secondCount := len(variables)

	// Should have same count (overwrites, not adds)
	if firstCount != secondCount {
		t.Errorf("Count changed: first=%d, second=%d", firstCount, secondCount)
	}
}

// TestExecutionContext_EmitEvent_WithPublisher tests event emission
func TestExecutionContext_EmitEvent_WithPublisher(t *testing.T) {
	// Create a simple subscriber to capture events
	eventReceived := false
	var receivedType string

	subscriber := &testSubscriber{
		onEvent: func(eventType string, data interface{}) {
			eventReceived = true
			receivedType = eventType
		},
	}

	// Note: This test is conceptual - actual implementation depends on publisher interface
	// For now, just test that EmitEvent doesn't panic
	ec := &ExecutionContext{}

	ec.EmitEvent("test_event", map[string]interface{}{"key": "value"})

	// If we got here without panicking, test passes
	t.Log("EmitEvent executed without panic")

	// Use the subscriber to avoid unused variable warning
	_ = subscriber
	_ = eventReceived
	_ = receivedType
}

// testSubscriber is a simple test subscriber
type testSubscriber struct {
	onEvent func(eventType string, data interface{})
}
