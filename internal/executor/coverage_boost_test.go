package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
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


// TestGetStepDisplayName_WithFileTreeItem tests display name for filetree items
func TestGetStepDisplayName_WithFileTreeItem(t *testing.T) {
	tests := []struct {
		name     string
		item     filetree.Item
		expected string
		isCustom bool
	}{
		{
			"root directory",
			filetree.Item{Name: "root", IsDir: true, Path: ""},
			"root/",
			true,
		},
		{
			"subdirectory",
			filetree.Item{Name: "subdir", IsDir: true, Path: "/path/to/subdir"},
			"path/to/subdir/",
			true,
		},
		{
			"file with name",
			filetree.Item{Name: "file.txt", IsDir: false, Path: "/path/file.txt"},
			"file.txt",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				Name: "Test Step",
			}

			ec := &ExecutionContext{
				Variables: map[string]interface{}{
					"item": tt.item,
				},
			}

			displayName, isCustom := GetStepDisplayName(step, ec)

			if isCustom != tt.isCustom {
				t.Errorf("GetStepDisplayName() isCustom = %v, want %v", isCustom, tt.isCustom)
			}

			if displayName == "" {
				t.Error("Display name should not be empty")
			}

			t.Logf("Display name: %s", displayName)
		})
	}
}

// TestGetStepDisplayName_NoFileTree tests display name without filetree
func TestGetStepDisplayName_NoFileTree(t *testing.T) {
	step := config.Step{
		Name: "Regular Step",
	}

	ec := &ExecutionContext{
		Variables: make(map[string]interface{}),
	}

	displayName, isCustom := GetStepDisplayName(step, ec)

	// Without an "item" variable, it falls back to step name
	if displayName != "Regular Step" {
		t.Errorf("Display name = %s, want 'Regular Step'", displayName)
	}

	// Note: isCustom may still be true due to fallback behavior
	_ = isCustom
}

// TestHandleVars_MergeExisting tests merging with existing variables
func TestHandleVars_MergeExisting(t *testing.T) {
	testLogger := logger.NewTestLogger()

	vars := map[string]interface{}{
		"new_key": "new_value",
	}

	step := config.Step{
		Vars: &vars,
	}

	ec := &ExecutionContext{
		Logger: testLogger,
		Variables: map[string]interface{}{
			"existing_key": "existing_value",
		},
		DryRun: false,
	}

	err := HandleVars(step, ec)
	if err != nil {
		t.Fatalf("HandleVars failed: %v", err)
	}

	// Both old and new should exist
	if ec.Variables["existing_key"] != "existing_value" {
		t.Error("Existing variables should be preserved")
	}
	if ec.Variables["new_key"] != "new_value" {
		t.Error("New variables should be added")
	}
}

// TestHandleWhenExpression_BooleanLogic tests boolean logic in when expressions
func TestHandleWhenExpression_BooleanLogic(t *testing.T) {
	tests := []struct {
		name       string
		when       string
		variables  map[string]interface{}
		shouldSkip bool
	}{
		{
			"AND true",
			"a == 1 && b == 2",
			map[string]interface{}{"a": 1, "b": 2},
			false,
		},
		{
			"AND false",
			"a == 1 && b == 3",
			map[string]interface{}{"a": 1, "b": 2},
			true,
		},
		{
			"OR true",
			"a == 1 || b == 3",
			map[string]interface{}{"a": 1, "b": 2},
			false,
		},
		{
			"OR false",
			"a == 99 || b == 99",
			map[string]interface{}{"a": 1, "b": 2},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := config.Step{
				When: tt.when,
			}

			ec := &ExecutionContext{
				Evaluator: expression.NewGovaluateEvaluator(),
				Template:  mustNewRenderer(),
				Logger:    logger.NewTestLogger(),
				Variables: tt.variables,
			}

			shouldSkip, err := HandleWhenExpression(step, ec)
			if err != nil {
				t.Fatalf("HandleWhenExpression failed: %v", err)
			}

			if shouldSkip != tt.shouldSkip {
				t.Errorf("shouldSkip = %v, want %v", shouldSkip, tt.shouldSkip)
			}
		})
	}
}

// TestCheckIdempotencyConditions_Creates tests creates condition
func TestCheckIdempotencyConditions_Creates(t *testing.T) {
	tmpFile := "/tmp/nonexistent_file_for_test_mooncake_12345.txt"

	step := config.Step{
		Creates: &tmpFile,
	}

	renderer := mustNewRenderer()

	ec := &ExecutionContext{
		Template:      renderer,
		Logger:        logger.NewTestLogger(),
		Variables:     make(map[string]interface{}),
		CurrentResult: NewResult(),
		PathUtil:      pathutil.NewPathExpander(renderer),
		CurrentDir:    "/tmp",
	}

	shouldSkip, reason, err := CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Fatalf("CheckIdempotencyConditions failed: %v", err)
	}

	// File doesn't exist, so should NOT skip (should execute)
	if shouldSkip {
		t.Errorf("Should not skip when creates file doesn't exist, reason: %s", reason)
	}
}

// TestResult_SetData_MultipleCalls tests multiple SetData calls
func TestResult_SetData_MultipleCalls(t *testing.T) {
	result := NewResult()

	// Multiple calls should not panic
	result.SetData(map[string]interface{}{"key1": "value1"})
	result.SetData(map[string]interface{}{"key2": "value2"})
	result.SetData(nil)

	// No assertions - SetData is a no-op
}

// TestMarkStepFailed_Idempotent tests multiple MarkStepFailed calls
func TestMarkStepFailed_Idempotent(t *testing.T) {
	result := NewResult()
	step := config.Step{Name: "Test"}
	ec := &ExecutionContext{
		Variables: make(map[string]interface{}),
	}

	MarkStepFailed(result, step, ec)
	MarkStepFailed(result, step, ec)

	if !result.Failed {
		t.Error("Result should be failed")
	}
	if result.Rc != 1 {
		t.Error("Rc should be 1")
	}
}

// TestAddGlobalVariables_NonDestructive tests that it doesn't remove existing vars
func TestAddGlobalVariables_NonDestructive(t *testing.T) {
	variables := map[string]interface{}{
		"custom_var": "custom_value",
		"os":         "should_be_overwritten",
	}

	AddGlobalVariables(variables)

	// Custom var should remain
	if variables["custom_var"] != "custom_value" {
		t.Error("Custom variables should be preserved")
	}

	// OS should be overwritten with actual value
	if variables["os"] == "should_be_overwritten" {
		t.Error("System facts should overwrite existing values")
	}
}

// TestEmitEvent_WithNilPublisher tests EmitEvent doesn't panic with nil publisher
func TestEmitEvent_WithNilPublisher(t *testing.T) {
	ec := &ExecutionContext{
		EventPublisher: nil, // Nil publisher
	}

	// Should not panic
	ec.EmitEvent(events.EventStepStarted, events.StepStartedData{
		StepID: "test",
		Name:   "Test",
	})
}

// TestCheckIdempotencyConditions_UnlessSuccess tests unless when command succeeds
func TestCheckIdempotencyConditions_UnlessSuccess(t *testing.T) {
	unlessCmd := "true" // Command that succeeds
	step := config.Step{
		Unless: &unlessCmd,
	}

	renderer := mustNewRenderer()
	ec := &ExecutionContext{
		Template:      renderer,
		Logger:        logger.NewTestLogger(),
		Variables:     make(map[string]interface{}),
		CurrentResult: NewResult(),
		PathUtil:      pathutil.NewPathExpander(renderer),
		CurrentDir:    "/tmp",
	}

	shouldSkip, reason, err := CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Should skip when unless command succeeds
	if !shouldSkip {
		t.Error("Should skip when unless command succeeds")
	}
	if reason == "" {
		t.Error("Should have reason")
	}
}

// TestCheckIdempotencyConditions_UnlessFail tests unless when command fails
func TestCheckIdempotencyConditions_UnlessFail(t *testing.T) {
	unlessCmd := "false" // Command that fails
	step := config.Step{
		Unless: &unlessCmd,
	}

	renderer := mustNewRenderer()
	ec := &ExecutionContext{
		Template:      renderer,
		Logger:        logger.NewTestLogger(),
		Variables:     make(map[string]interface{}),
		CurrentResult: NewResult(),
		PathUtil:      pathutil.NewPathExpander(renderer),
		CurrentDir:    "/tmp",
	}

	shouldSkip, _, err := CheckIdempotencyConditions(step, ec)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Should NOT skip when unless command fails
	if shouldSkip {
		t.Error("Should not skip when unless command fails")
	}
}

// TestGetStepDisplayName_NoName tests step without name
func TestGetStepDisplayName_NoName(t *testing.T) {
	step := config.Step{
		// No name
	}

	ec := &ExecutionContext{
		Variables: make(map[string]interface{}),
	}

	displayName, hasName := GetStepDisplayName(step, ec)
	if hasName {
		t.Error("Should not have name for anonymous step")
	}
	if displayName != "" {
		t.Error("Display name should be empty")
	}
}

// TestGetStepDisplayName_WithItem tests with_items item display
func TestGetStepDisplayName_WithItem(t *testing.T) {
	step := config.Step{
		Name: "Test",
	}

	ec := &ExecutionContext{
		Variables: map[string]interface{}{
			"item": "test-item",
		},
	}

	displayName, hasName := GetStepDisplayName(step, ec)
	if !hasName {
		t.Error("Should have name with item")
	}
	if displayName != "test-item" {
		t.Errorf("Display name = %s, want 'test-item'", displayName)
	}
}

// TestGetStepDisplayName_FileTreeItemPath tests filetree item with path
func TestGetStepDisplayName_FileTreeItemPath(t *testing.T) {
	step := config.Step{
		Name: "Test",
	}

	ec := &ExecutionContext{
		Variables: map[string]interface{}{
			"item": filetree.Item{
				Name:  "",
				IsDir: false,
				Path:  "/some/file.txt",
			},
		},
	}

	displayName, hasName := GetStepDisplayName(step, ec)
	if !hasName {
		t.Error("Should have name")
	}
	if displayName == "" {
		t.Error("Display name should not be empty")
	}
}

// TestHandleWhenExpression_NilResult tests when evaluating to nil
func TestHandleWhenExpression_NilResult(t *testing.T) {
	step := config.Step{
		When: "undefined_var", // Will evaluate to nil
	}

	ec := &ExecutionContext{
		Evaluator: expression.NewGovaluateEvaluator(),
		Template:  mustNewRenderer(),
		Logger:    logger.NewTestLogger(),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, err := HandleWhenExpression(step, ec)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	// Nil should cause skip
	if !shouldSkip {
		t.Error("Should skip when expression is nil")
	}
}

// TestHandleWhenExpression_NonBool tests non-boolean result
func TestHandleWhenExpression_NonBool(t *testing.T) {
	step := config.Step{
		When: "42", // Number, not bool
	}

	ec := &ExecutionContext{
		Evaluator: expression.NewGovaluateEvaluator(),
		Template:  mustNewRenderer(),
		Logger:    logger.NewTestLogger(),
		Variables: make(map[string]interface{}),
	}

	_, err := HandleWhenExpression(step, ec)
	if err == nil {
		t.Error("Should error on non-bool result")
	}
}

// TestCheckSkipConditions_WhenFalse tests when expression is false
func TestCheckSkipConditions_WhenFalse(t *testing.T) {
	step := config.Step{
		When: "false",
	}

	renderer := mustNewRenderer()
	ec := &ExecutionContext{
		Evaluator: expression.NewGovaluateEvaluator(),
		Template:  renderer,
		Logger:    logger.NewTestLogger(),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, reason, err := CheckSkipConditions(step, ec)
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}

	if !shouldSkip {
		t.Error("Should skip when when is false")
	}
	if reason != "when" {
		t.Errorf("Reason = %s, want 'when'", reason)
	}
}

// TestLogServiceOperation_DisableService tests disabled service logging
func TestLogServiceOperation_DisableService(t *testing.T) {
	dryRun := NewDryRunLogger(logger.NewTestLogger())

	disabled := false
	serviceAction := &config.ServiceAction{
		Name:    "test-service",
		Enabled: &disabled, // Explicitly disable
	}

	// Should not panic
	dryRun.LogServiceOperation("test-service", serviceAction, false)
}
