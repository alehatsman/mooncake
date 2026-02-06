package executor_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/actions/print"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	_ "github.com/alehatsman/mooncake/internal/actions/vars"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
)

// TestExecutePlan_BasicExecution tests ExecutePlan with minimal plan
func TestExecutePlan_BasicExecution(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	// Create the plan directly - no need for a config file
	planData := &plan.Plan{
		RootFile:    configPath,
		Steps:       []config.Step{}, // Empty plan
		InitialVars: make(map[string]interface{}),
		Tags:        []string{},
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.ExecutePlan(planData, "", false, testLogger, publisher)
	if err != nil {
		t.Errorf("ExecutePlan failed: %v", err)
	}
}

// TestExecutePlan_WithSudoPass tests ExecutePlan with sudo password (redaction)
func TestExecutePlan_WithSudoPass(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	planData := &plan.Plan{
		RootFile:    configPath,
		Steps:       []config.Step{},
		InitialVars: make(map[string]interface{}),
		Tags:        []string{},
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.ExecutePlan(planData, "test-sudo-password", false, testLogger, publisher)
	if err != nil {
		t.Errorf("ExecutePlan failed: %v", err)
	}
}

// TestExecutePlan_DryRun tests ExecutePlan in dry-run mode
func TestExecutePlan_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	planData := &plan.Plan{
		RootFile:    configPath,
		Steps:       []config.Step{},
		InitialVars: make(map[string]interface{}),
		Tags:        []string{},
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.ExecutePlan(planData, "", true, testLogger, publisher)
	if err != nil {
		t.Errorf("ExecutePlan in dry-run failed: %v", err)
	}
}

// TestStart_EmptyConfigPath tests Start with empty config path
func TestStart_EmptyConfigPath(t *testing.T) {
	cfg := executor.StartConfig{
		ConfigFilePath: "", // Empty path
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.Start(cfg, testLogger, publisher)
	if err == nil {
		t.Error("Start should fail with empty config path")
	}
}

// TestStart_InvalidConfigPath tests Start with non-existent config file
func TestStart_InvalidConfigPath(t *testing.T) {
	cfg := executor.StartConfig{
		ConfigFilePath: "/nonexistent/path/config.yml",
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.Start(cfg, testLogger, publisher)
	if err == nil {
		t.Error("Start should fail with non-existent config file")
	}
}

// TestStart_WithVarsFile tests Start with variables file
func TestStart_WithVarsFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple config file
	configPath := filepath.Join(tmpDir, "test.yml")
	configContent := `
- name: Test Step
  print: "{{ test_var }}"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create a vars file
	varsPath := filepath.Join(tmpDir, "vars.yml")
	varsContent := `test_var: test_value`
	if err := os.WriteFile(varsPath, []byte(varsContent), 0644); err != nil {
		t.Fatalf("Failed to create vars file: %v", err)
	}

	cfg := executor.StartConfig{
		ConfigFilePath: configPath,
		VarsFilePath:   varsPath,
		DryRun:         false,
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.Start(cfg, testLogger, publisher)
	if err != nil {
		t.Errorf("Start failed: %v", err)
	}
}

// TestStart_DryRun tests Start in dry-run mode
func TestStart_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "test.yml")
	configContent := `
- name: Test Step
  print: "test message"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	cfg := executor.StartConfig{
		ConfigFilePath: configPath,
		DryRun:         true,
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	err := executor.Start(cfg, testLogger, publisher)
	if err != nil {
		t.Errorf("Start in dry-run failed: %v", err)
	}
}
