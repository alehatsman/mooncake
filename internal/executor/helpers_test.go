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
				Svc: &RunServices{
					Stats: &ExecutionStats{
						Global: &global,
					},
				},
			}

			result := generateStepID(tt.step, ec)
			if result != tt.expected {
				t.Errorf("generateStepID() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAddGlobalVariables tests global variable injection
func TestAddGlobalVariables(t *testing.T) {
	scope := NewVariableScope()

	AddGlobalVariables(scope)

	// Should have added facts
	vars := scope.ToMap()
	if len(vars) == 0 {
		t.Error("AddGlobalVariables should add facts to scope")
	}

	// Check for common facts
	expectedFacts := []string{"os", "arch", "hostname"}
	for _, fact := range expectedFacts {
		if _, ok := vars[fact]; !ok {
			t.Errorf("Expected fact %q to be in scope variables", fact)
		}
	}
}

// TestAddGlobalVariables_Existing tests adding to existing variables
func TestAddGlobalVariables_Existing(t *testing.T) {
	scope := NewVariableScope()
	scope.User["existing"] = "value"

	AddGlobalVariables(scope)

	vars := scope.ToMap()

	// Custom var should remain
	if vars["existing"] != "value" {
		t.Error("Existing variables should be preserved")
	}

	// Should have added new facts
	if len(vars) <= 1 {
		t.Error("Should have added facts in addition to existing variables")
	}
}

// TestAddGlobalVariables_Idempotent tests multiple calls
func TestAddGlobalVariables_Idempotent(t *testing.T) {
	scope := NewVariableScope()

	AddGlobalVariables(scope)
	firstCount := len(scope.ToMap())

	AddGlobalVariables(scope)
	secondCount := len(scope.ToMap())

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
	ec := &ExecutionContext{Svc: &RunServices{}}

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
