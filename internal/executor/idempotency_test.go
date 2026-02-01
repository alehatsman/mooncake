package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestCheckIdempotencyConditions_Creates_FileExists(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Step with creates pointing to existing file
	step := config.Step{
		Shell:   strPtr("echo test"),
		Creates: strPtr(tmpFile.Name()),
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		PathUtil:  pathutil.NewPathExpander(renderer),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, reason, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when file exists")
	}
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
	if !strings.Contains(reason, tmpFile.Name()) {
		t.Errorf("Expected reason to contain file path, got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Creates_FileNotExists(t *testing.T) {
	creates := "/nonexistent/file/that/does/not/exist"
	step := config.Step{
		Shell:   strPtr("echo test"),
		Creates: strPtr(creates),
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		PathUtil:  pathutil.NewPathExpander(renderer),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, _, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when file doesn't exist")
	}
}

func TestCheckIdempotencyConditions_Creates_WithTemplateVariable(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Shell:   strPtr("echo test"),
		Creates: strPtr("{{ output_file }}"),
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template: renderer,
		PathUtil: pathutil.NewPathExpander(renderer),
		Variables: map[string]interface{}{
			"output_file": tmpFile.Name(),
		},
	}

	shouldSkip, reason, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when templated file exists")
	}
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Unless_CommandSucceeds(t *testing.T) {
	unless := "true" // Always succeeds
	step := config.Step{
		Shell:  strPtr("echo test"),
		Unless: &unless,
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		Variables: make(map[string]interface{}),
	}

	shouldSkip, reason, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when unless command succeeds")
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("Expected reason to contain 'unless:', got: %s", reason)
	}
	if !strings.Contains(reason, "true") {
		t.Errorf("Expected reason to contain command, got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_Unless_CommandFails(t *testing.T) {
	unless := "false" // Always fails
	step := config.Step{
		Shell:  strPtr("echo test"),
		Unless: &unless,
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		Variables: make(map[string]interface{}),
	}

	shouldSkip, _, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when unless command fails")
	}
}

func TestCheckIdempotencyConditions_Unless_WithTemplateVariable(t *testing.T) {
	step := config.Step{
		Shell:  strPtr("echo test"),
		Unless: strPtr("test -f {{ marker_file }}"),
	}

	// Create temp file for testing
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template: renderer,
		Variables: map[string]interface{}{
			"marker_file": tmpFile.Name(),
		},
	}

	shouldSkip, reason, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when unless command with template succeeds")
	}
	if !strings.Contains(reason, "unless:") {
		t.Errorf("Expected reason to contain 'unless:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_BothConditions(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Both creates and unless are satisfied
	step := config.Step{
		Shell:   strPtr("echo test"),
		Creates: strPtr(tmpFile.Name()),
		Unless:  strPtr("true"),
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		PathUtil:  pathutil.NewPathExpander(renderer),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, reason, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !shouldSkip {
		t.Error("Expected step to be skipped when creates condition is met")
	}
	// Creates is checked first, so reason should be about creates
	if !strings.Contains(reason, "creates:") {
		t.Errorf("Expected reason to contain 'creates:', got: %s", reason)
	}
}

func TestCheckIdempotencyConditions_NoConditions(t *testing.T) {
	step := config.Step{
		Shell: strPtr("echo test"),
	}

	renderer := template.NewPongo2Renderer()
	ec := &ExecutionContext{
		Template:  renderer,
		PathUtil:  pathutil.NewPathExpander(renderer),
		Variables: make(map[string]interface{}),
	}

	shouldSkip, _, err := checkIdempotencyConditions(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if shouldSkip {
		t.Error("Expected step NOT to be skipped when no idempotency conditions")
	}
}

func TestExecuteStep_IdempotencyIntegration(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "test")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	step := config.Step{
		Name:    "Test step with creates",
		Shell:   strPtr("echo should not run"),
		Creates: strPtr(tmpFile.Name()),
	}

	renderer := template.NewPongo2Renderer()
	statsSkipped := 0
	statsExecuted := 0
	statsFailed := 0
	globalExecuted := 0

	ec := &ExecutionContext{
		Template:            renderer,
		PathUtil:            pathutil.NewPathExpander(renderer),
		Evaluator:           expression.NewGovaluateEvaluator(),
		Variables:           make(map[string]interface{}),
		Logger:              logger.NewConsoleLogger(logger.InfoLevel),
		StatsSkipped:        &statsSkipped,
		StatsExecuted:       &statsExecuted,
		StatsFailed:         &statsFailed,
		GlobalStepsExecuted: &globalExecuted,
	}

	err = ExecuteStep(step, ec)

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Step should be skipped
	if statsSkipped != 1 {
		t.Errorf("Expected 1 skipped step, got %d", statsSkipped)
	}
	if statsExecuted != 0 {
		t.Errorf("Expected 0 executed steps, got %d", statsExecuted)
	}
}
