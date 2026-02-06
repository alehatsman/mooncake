package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestExecutionContext_GetEvaluator tests GetEvaluator method
func TestExecutionContext_GetEvaluator(t *testing.T) {
	ctx := &ExecutionContext{
		Evaluator: expression.NewGovaluateEvaluator(),
	}

	evaluator := ctx.GetEvaluator()
	if evaluator == nil {
		t.Error("GetEvaluator() should return non-nil evaluator")
	}
}

// TestExecutionContext_GetEvaluator_Nil tests GetEvaluator with nil evaluator
func TestExecutionContext_GetEvaluator_Nil(t *testing.T) {
	ctx := &ExecutionContext{
		Evaluator: nil,
	}

	evaluator := ctx.GetEvaluator()
	// Should return nil when evaluator is not set
	if evaluator != nil {
		t.Error("GetEvaluator() should return nil when evaluator is not set")
	}
}

// TestExecutionContext_GetTemplate tests GetTemplate method
func TestExecutionContext_GetTemplate(t *testing.T) {
	tmpl := template.NewPongo2Renderer()
	ctx := &ExecutionContext{
		Template: tmpl,
	}

	result := ctx.GetTemplate()
	if result == nil {
		t.Error("GetTemplate() should return non-nil template")
	}
}

// TestExecutionContext_EmitEvent_NilPublisher tests EmitEvent with nil publisher
func TestExecutionContext_EmitEvent_NilPublisher(t *testing.T) {
	ctx := &ExecutionContext{}

	// Should not panic with nil publisher
	ctx.EmitEvent("test_event", map[string]interface{}{"key": "value"})
}

// TestExecutionContext_HandleDryRun_WithLogging tests HandleDryRun with logging
func TestExecutionContext_HandleDryRun_WithLogging(t *testing.T) {
	testLogger := logger.NewTestLogger()
	ctx := &ExecutionContext{
		DryRun: true,
		Logger: testLogger,
	}

	called := false
	result := ctx.HandleDryRun(func(dryRun *DryRunLogger) {
		called = true
		dryRun.LogPrintMessage("test")
	})

	if !result {
		t.Error("HandleDryRun should return true when dry-run is enabled")
	}

	if !called {
		t.Error("HandleDryRun should call the provided function")
	}
}

// TestExecutionContext_HandleDryRun_Disabled tests HandleDryRun when disabled
func TestExecutionContext_HandleDryRun_Disabled(t *testing.T) {
	testLogger := logger.NewTestLogger()
	ctx := &ExecutionContext{
		DryRun: false,
		Logger: testLogger,
	}

	called := false
	result := ctx.HandleDryRun(func(dryRun *DryRunLogger) {
		called = true
	})

	if result {
		t.Error("HandleDryRun should return false when dry-run is disabled")
	}

	if called {
		t.Error("HandleDryRun should not call the function when disabled")
	}
}

// TestExecutionContext_HandleDryRun_MultipleOperations tests multiple dry-run ops
func TestExecutionContext_HandleDryRun_MultipleOperations(t *testing.T) {
	testLogger := logger.NewTestLogger()
	ctx := &ExecutionContext{
		DryRun: true,
		Logger: testLogger,
	}

	operationCount := 0
	ctx.HandleDryRun(func(dryRun *DryRunLogger) {
		dryRun.LogPrintMessage("op1")
		operationCount++
	})

	ctx.HandleDryRun(func(dryRun *DryRunLogger) {
		dryRun.LogPrintMessage("op2")
		operationCount++
	})

	if operationCount != 2 {
		t.Errorf("Expected 2 operations, got %d", operationCount)
	}
}

// TestNewExecutionContext tests context creation
func TestNewExecutionContext(t *testing.T) {
	testLogger := logger.NewTestLogger()
	tmpl := template.NewPongo2Renderer()
	eval := expression.NewGovaluateEvaluator()

	ctx := &ExecutionContext{
		Logger:    testLogger,
		Template:  tmpl,
		Evaluator: eval,
		DryRun:    false,
	}

	if ctx.Logger == nil {
		t.Error("ExecutionContext Logger should not be nil")
	}
	if ctx.Template == nil {
		t.Error("ExecutionContext Template should not be nil")
	}
	if ctx.Evaluator == nil {
		t.Error("ExecutionContext Evaluator should not be nil")
	}
	if ctx.DryRun {
		t.Error("ExecutionContext DryRun should be false by default")
	}
}

// TestExecutionContext_IsDryRun_EdgeCases tests IsDryRun edge cases
func TestExecutionContext_IsDryRun_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   bool
		expected bool
	}{
		{"dry-run enabled", true, true},
		{"dry-run disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ExecutionContext{
				DryRun: tt.dryRun,
			}

			if ctx.IsDryRun() != tt.expected {
				t.Errorf("IsDryRun() = %v, want %v", ctx.IsDryRun(), tt.expected)
			}
		})
	}
}
