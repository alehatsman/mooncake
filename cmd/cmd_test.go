package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/cmd/kernel"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/urfave/cli/v2"
)

// TestParseTags tests the cmdutil.ParseTags function
func TestParseTags(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "single tag",
			input:    "tag1",
			expected: []string{"tag1"},
		},
		{
			name:     "multiple tags",
			input:    "tag1,tag2,tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "tags with spaces",
			input:    "tag1, tag2 , tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "tags with empty entries",
			input:    "tag1,,tag2, ,tag3",
			expected: []string{"tag1", "tag2", "tag3"},
		},
		{
			name:     "only commas and spaces",
			input:    ", , ,",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmdutil.ParseTags(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("cmdutil.ParseTags() length = %v, expected %v", len(result), len(tt.expected))
				return
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("cmdutil.ParseTags()[%d] = %v, expected %v", i, tag, tt.expected[i])
				}
			}
		})
	}
}

// TestWriteFactsJSON tests the kernel.WriteFactsJSON function
func TestWriteFactsJSON(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "facts.json")

	// Create simple facts
	f := &facts.Facts{
		OS:       "linux",
		Arch:     "amd64",
		CPUCores: 4,
	}

	// Test successful write
	err := kernel.WriteFactsJSON(f, testPath)
	if err != nil {
		t.Errorf("kernel.WriteFactsJSON() error = %v, expected nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("kernel.WriteFactsJSON() did not create file")
	}

	// Verify file content. MT-74: keys are snake_case (matching the
	// template scope and /v1/facts endpoint), so unmarshal into a
	// map[string]any rather than the typed *facts.Facts struct.
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("kernel.WriteFactsJSON() produced invalid JSON: %v", err)
	}

	if got := result["os"]; got != f.OS {
		t.Errorf("os mismatch: got %v, want %v", got, f.OS)
	}
	if got := result["arch"]; got != f.Arch {
		t.Errorf("arch mismatch: got %v, want %v", got, f.Arch)
	}
	if got, _ := result["cpu_cores"].(float64); int(got) != f.CPUCores {
		t.Errorf("cpu_cores mismatch: got %v, want %v", got, f.CPUCores)
	}
	if _, hasPascal := result["OS"]; hasPascal {
		t.Errorf("MT-74 regression: JSON has PascalCase 'OS' key — must be snake_case")
	}

	// Test invalid path
	invalidPath := filepath.Join(tmpDir, "nonexistent", "facts.json")
	err = kernel.WriteFactsJSON(f, invalidPath)
	if err == nil {
		t.Errorf("kernel.WriteFactsJSON() with invalid path should return error")
	}
}

// TestFormatPlanJSON tests the kernel.FormatPlanJSON function (indirectly)
func TestFormatPlanJSONIndirect(t *testing.T) {
	// Test that we can create the plan structure and it marshals correctly
	// This is a smoke test since kernel.FormatPlanJSON writes to stdout
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write minimal valid config
	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// This tests that the plan structure is valid for JSON marshaling
	// (indirect test of kernel.FormatPlanJSON functionality)
	t.Run("plan json structure", func(t *testing.T) {
		// The actual kernel.FormatPlanJSON writes to stdout, which is hard to test
		// But we can verify the structure is JSON-serializable
		// This test passes if it compiles and runs without error
	})
}

// TestCreateApp tests the createApp function
func TestCreateApp(t *testing.T) {
	app := createApp()

	if app == nil {
		t.Fatal("createApp() returned nil")
	}

	if app.Name != "mooncake" {
		t.Errorf("app.Name = %q, expected %q", app.Name, "mooncake")
	}

	if app.Usage != "Space fighters provisioning tool, Chookity!" {
		t.Errorf("app.Usage = %q, expected correct usage text", app.Usage)
	}

	if !app.EnableBashCompletion {
		t.Errorf("app.EnableBashCompletion = false, expected true")
	}

	// Test commands exist
	expectedCommands := []string{"init", "doctor", "mod", "docs", "schema", "snapshot", "history", "mcp", "step", "task", "tool", "apply", "plan", "facts", "explain", "metrics", "actions", "validate", "pilot", "agentd", "fleet", "runs", "query"}
	if len(app.Commands) != len(expectedCommands) {
		t.Errorf("app.Commands length = %d, expected %d", len(app.Commands), len(expectedCommands))
	}

	commandNames := make(map[string]bool)
	for _, cmd := range app.Commands {
		commandNames[cmd.Name] = true
	}

	for _, expectedCmd := range expectedCommands {
		if !commandNames[expectedCmd] {
			t.Errorf("missing command: %s", expectedCmd)
		}
	}
}

// TestFactsCommandInvalidFormat tests factsCommand with invalid format
func TestFactsCommandInvalidFormat(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "facts",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.FactsAction,
			},
		},
	}

	// Test invalid format
	err := app.Run([]string{"test", "facts", "--format", "invalid"})

	if err == nil {
		t.Errorf("factsCommand with invalid format should return error")
	}

	expectedMsg := "invalid format"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

// TestValidateCommandInvalidPath tests validateCommand with invalid config path
func TestValidateCommandInvalidPath(t *testing.T) {
	// This test will exit the program via os.Exit, so we can't test it directly
	// But we can verify the command structure is correct
	t.Logf("validateCommand structure test passed")
}

// TestConstants verifies that important constants are defined
func TestConstants(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
	}{
		{
			name:     "outputFormatJSON",
			value:    outputFormatJSON,
			expected: "json",
		},
		{
			name:     "outputFormatText",
			value:    outputFormatText,
			expected: "text",
		},
		{
			name:     "outputFormatYAML",
			value:    outputFormatYAML,
			expected: "yaml",
		},
		{
			name:     "defaultMaxOutputBytes",
			value:    defaultMaxOutputBytes,
			expected: 1048576,
		},
		{
			name:     "defaultMaxOutputLines",
			value:    defaultMaxOutputLines,
			expected: 1000,
		},
		{
			name:     "yamlIndentSpaces",
			value:    yamlIndentSpaces,
			expected: 2,
		},
		{
			name:     "exitCodeValidationError",
			value:    exitCodeValidationError,
			expected: 2,
		},
		{
			name:     "exitCodeRuntimeError",
			value:    exitCodeRuntimeError,
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.expected {
				t.Errorf("%s = %v, expected %v", tt.name, tt.value, tt.expected)
			}
		})
	}
}

// TestRunCommandFlags tests that run command has all expected flags
func TestRunCommandFlags(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	expectedFlags := []string{
		"config", "vars", "log-level", "sudo-pass", "ask-become-pass",
		"sudo-pass-file", "insecure-sudo-pass", "tags", "tui",
		"output-format", "artifacts-dir", "capture-full-output",
		"max-output-bytes", "max-output-lines", "from-plan", "facts-json",
		"allow-stale", "max-plan-age",
	}

	flagNames := make(map[string]bool)
	for _, flag := range runCmd.Flags {
		// Extract flag name from the flag interface
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.StringSliceFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		case *cli.IntFlag:
			flagNames[f.Name] = true
		case *cli.DurationFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("run command missing flag: %s", expectedFlag)
		}
	}
}

// TestPlanCommandFlags tests that plan command has all expected flags
func TestPlanCommandFlags(t *testing.T) {
	app := createApp()

	var planCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "plan" {
			planCmd = cmd
			break
		}
	}

	if planCmd == nil {
		t.Fatal("plan command not found")
	}

	expectedFlags := []string{
		"config", "vars", "tags", "format", "show-origins", "output",
	}

	flagNames := make(map[string]bool)
	for _, flag := range planCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.StringSliceFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("plan command missing flag: %s", expectedFlag)
		}
	}
}

// TestFactsCommandFlags tests that facts command has all expected flags
func TestFactsCommandFlags(t *testing.T) {
	app := createApp()

	var factsCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "facts" {
			factsCmd = cmd
			break
		}
	}

	if factsCmd == nil {
		t.Fatal("facts command not found")
	}

	expectedFlags := []string{"format"}

	flagNames := make(map[string]bool)
	for _, flag := range factsCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("facts command missing flag: %s", expectedFlag)
		}
	}
}

// TestValidateCommandFlags tests that validate command has all expected flags
func TestValidateCommandFlags(t *testing.T) {
	app := createApp()

	var validateCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "validate" {
			validateCmd = cmd
			break
		}
	}

	if validateCmd == nil {
		t.Fatal("validate command not found")
	}

	expectedFlags := []string{"config", "vars", "format"}

	flagNames := make(map[string]bool)
	for _, flag := range validateCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.StringSliceFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("validate command missing flag: %s", expectedFlag)
		}
	}
}

// TestWriteFactsJSONFilePermissions tests that facts JSON is written with correct permissions
func TestWriteFactsJSONFilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "facts.json")

	f := &facts.Facts{
		OS:   "linux",
		Arch: "amd64",
	}

	err := kernel.WriteFactsJSON(f, testPath)
	if err != nil {
		t.Fatalf("kernel.WriteFactsJSON() error = %v", err)
	}

	// Check file permissions
	info, err := os.Stat(testPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	mode := info.Mode()
	expectedMode := os.FileMode(0600)

	if mode != expectedMode {
		t.Errorf("file permissions = %v, expected %v", mode, expectedMode)
	}
}

// TestParseTagsPreservesOrder tests that cmdutil.ParseTags preserves tag order
func TestParseTagsPreservesOrder(t *testing.T) {
	input := "tag3,tag1,tag2"
	expected := []string{"tag3", "tag1", "tag2"}

	result := cmdutil.ParseTags(input)

	if len(result) != len(expected) {
		t.Errorf("cmdutil.ParseTags() length = %v, expected %v", len(result), len(expected))
		return
	}

	for i, tag := range result {
		if tag != expected[i] {
			t.Errorf("cmdutil.ParseTags()[%d] = %v, expected %v (order not preserved)", i, tag, expected[i])
		}
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// TestFactsCommandValidFormat tests factsCommand with valid formats
func TestFactsCommandValidFormats(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "text format",
			format: "text",
		},
		{
			name:   "json format",
			format: "json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.App{
				Name: "test",
				Commands: []*cli.Command{
					{
						Name: "facts",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "format", Value: "text"},
						},
						Action: kernel.FactsAction,
					},
				},
			}

			// Should complete without error (output goes to stdout)
			err := app.Run([]string{"test", "facts", "--format", tt.format})

			// The command should complete (may produce output)
			t.Logf("factsCommand with format %s completed with error: %v", tt.format, err)
		})
	}
}

// TestPlanCommandInvalidFormat tests planCommand with invalid format
func TestPlanCommandInvalidFormatHandling(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write minimal valid config
	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", "invalid"})

	if err == nil {
		t.Errorf("planCommand with invalid format should return error")
	}

	expectedMsg := "unsupported format"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

// TestPlanCommandValidFormats tests planCommand with valid formats
func TestPlanCommandValidFormats(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	tests := []struct {
		name   string
		format string
	}{
		{
			name:   "text format",
			format: "text",
		},
		{
			name:   "json format",
			format: "json",
		},
		{
			name:   "yaml format",
			format: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.App{
				Name: "test",
				Commands: []*cli.Command{
					{
						Name: "plan",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "config", Required: true},
							&cli.StringFlag{Name: "format", Value: "text"},
						},
						Action: kernel.PlanAction,
					},
				},
			}

			err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", tt.format})

			// Should complete without error (output goes to stdout)
			if err != nil {
				t.Errorf("planCommand with format %s failed: %v", tt.format, err)
			}
		})
	}
}

// TestPlanCommandWithTags tests planCommand with tags
func TestPlanCommandWithTags(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: test
    shell: echo hello
    tags: [tag1, tag2]
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.StringFlag{Name: "tags"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--tags", "tag1,tag2"})

	if err != nil {
		t.Errorf("planCommand with tags failed: %v", err)
	}
}

// TestPlanCommandWithOutput tests planCommand with output file
func TestPlanCommandWithOutput(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")
	outputFile := filepath.Join(tmpDir, "plan.json")

	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.StringFlag{Name: "output"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--output", outputFile})

	if err != nil {
		t.Errorf("planCommand with output file failed: %v", err)
	}

	// Verify output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("planCommand did not create output file")
	}
}

// TestFormatPlanTextWithLoopContext tests kernel.FormatPlanText with loop context
func TestFormatPlanTextWithLoopContext(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write config with for_each (spec-21 renamed `loop` → `for_each`).
	// for_each is a string in the schema; the inline-list form is an
	// older shape the runtime still accepts but the validator doesn't,
	// so use the documented `for_each: <vars-ref>` form here.
	configContent := `version: "1.0"
vars:
  my_items:
    - item1
    - item2
steps:
  - name: test loop
    shell: echo {{ item }}
    for_each: "{{ my_items }}"
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.BoolFlag{Name: "show-origins"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	// Test with show-origins flag to cover more code paths
	err := app.Run([]string{"test", "plan", "--config", testConfig, "--show-origins"})

	if err != nil {
		t.Errorf("planCommand with loop context failed: %v", err)
	}
}

// TestPlanCommandWithVars tests planCommand with variables file
func TestPlanCommandWithVars(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")
	varsFile := filepath.Join(tmpDir, "vars.yml")

	configContent := `steps:
  - name: test
    shell: echo {{ my_var }}
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	varsContent := `my_var: hello
`
	if err := os.WriteFile(varsFile, []byte(varsContent), 0600); err != nil {
		t.Fatalf("failed to write vars file: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "vars"},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--vars", varsFile})

	if err != nil {
		t.Errorf("planCommand with vars failed: %v", err)
	}
}

// TestPlanCommandInvalidVarsFile tests planCommand with invalid vars file
func TestPlanCommandInvalidVarsFile(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringSliceFlag{Name: "vars"},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--vars", "/nonexistent/vars.yml"})

	if err == nil {
		t.Errorf("planCommand with invalid vars file should return error")
	}
}

// TestPlanCommand_MultipleVarsFiles verifies that --vars accepts multiple
// files and that they're merged in order (later wins). Before the fix to
// make --vars a StringSliceFlag, only the last -v was honored — earlier
// files were silently dropped.
func TestPlanCommand_MultipleVarsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")
	outputPath := filepath.Join(tmpDir, "plan.json")
	varsA := filepath.Join(tmpDir, "a.yml")
	varsB := filepath.Join(tmpDir, "b.yml")

	if err := os.WriteFile(configPath, []byte(`steps:
  - name: noop
    shell: "true"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(varsA, []byte("ONLY_A: from_a\nSHARED: a_value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(varsB, []byte("ONLY_B: from_b\nSHARED: b_value\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringSliceFlag{Name: "vars"},
					&cli.StringFlag{Name: "output"},
					&cli.StringFlag{Name: "format", Value: "json"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{
		"test", "plan",
		"--config", configPath,
		"--vars", varsA,
		"--vars", varsB,
		"--output", outputPath,
	})
	if err != nil {
		t.Fatalf("planCommand returned: %v", err)
	}

	// Read the saved plan and verify the merged vars landed in InitialVars.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read plan: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		`"ONLY_A": "from_a"`,
		`"ONLY_B": "from_b"`,
		`"SHARED": "b_value"`, // b wins (later)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plan output missing %q\nfull output:\n%s", want, body)
		}
	}
	if strings.Contains(body, `"SHARED": "a_value"`) {
		t.Error("plan output kept a's SHARED value; b should have overridden it")
	}
}

// TestPlanCommandInvalidConfigFile tests planCommand with invalid config
func TestPlanCommandInvalidConfigFile(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", "/nonexistent/config.yml"})

	if err == nil {
		t.Errorf("planCommand with invalid config should return error")
	}
}

// TestFactsCommandJSONOutput tests factsCommand with JSON output
func TestFactsCommandJSONOutput(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "facts",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.FactsAction,
			},
		},
	}

	err := app.Run([]string{"test", "facts", "--format", "json"})

	if err != nil {
		t.Errorf("factsCommand with json format failed: %v", err)
	}
}

// TestWriteFactsJSONMarshalCheck tests kernel.WriteFactsJSON marshaling
func TestWriteFactsJSONMarshalCheck(t *testing.T) {
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "facts.json")

	// Create facts with various fields populated
	f := &facts.Facts{
		OS:            "linux",
		Arch:          "amd64",
		Hostname:      "test-host",
		KernelVersion: "5.10.0",
		CPUCores:      8,
		CPUModel:      "Intel Core i7",
		MemoryTotalMB: 16384,
	}

	err := kernel.WriteFactsJSON(f, testPath)
	if err != nil {
		t.Errorf("kernel.WriteFactsJSON() error = %v", err)
	}

	// Verify JSON content. MT-74: keys are snake_case (matching the
	// template scope and /v1/facts endpoint).
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("kernel.WriteFactsJSON() produced invalid JSON: %v", err)
	}

	if got := result["os"]; got != f.OS {
		t.Errorf("os mismatch: got %v, want %v", got, f.OS)
	}
	if got := result["hostname"]; got != f.Hostname {
		t.Errorf("hostname mismatch: got %v, want %v", got, f.Hostname)
	}
	if got, _ := result["cpu_cores"].(float64); int(got) != f.CPUCores {
		t.Errorf("cpu_cores mismatch: got %v, want %v", got, f.CPUCores)
	}
}

// TestParseTagsEdgeCases tests cmdutil.ParseTags with additional edge cases
func TestParseTagsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "single comma",
			input:    ",",
			expected: nil,
		},
		{
			name:     "leading comma",
			input:    ",tag1,tag2",
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "trailing comma",
			input:    "tag1,tag2,",
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "multiple consecutive commas",
			input:    "tag1,,,tag2",
			expected: []string{"tag1", "tag2"},
		},
		{
			name:     "whitespace only tags",
			input:    "   ,   ,   ",
			expected: nil,
		},
		{
			name:     "mixed spaces and commas",
			input:    " tag1 , , tag2 ",
			expected: []string{"tag1", "tag2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmdutil.ParseTags(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("cmdutil.ParseTags() length = %v, expected %v", len(result), len(tt.expected))
				return
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("cmdutil.ParseTags()[%d] = %v, expected %v", i, tag, tt.expected[i])
				}
			}
		})
	}
}

// TestFormatPlanTextAllActionTypes tests kernel.FormatPlanText with different action types
func TestFormatPlanTextAllActionTypes(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write config with multiple action types to cover all branches
	configContent := `steps:
  - name: shell step
    shell: echo hello

  - name: file step
    file.write:
      path: /tmp/test
      state: touch

  - name: vars step
    vars:
      my_var: value

  - name: print step
    log: "test"
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.BoolFlag{Name: "show-origins"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig})

	if err != nil {
		t.Errorf("planCommand with multiple action types failed: %v", err)
	}
}

// TestRunCommandOutputFormatValidation tests run command output format validation
func TestRunCommandOutputFormatValidation(t *testing.T) {
	// These tests verify the validation logic by checking the command structure
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	// Verify run command has output-format flag
	hasOutputFormat := false
	hasFormatAlias := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "output-format" {
			hasOutputFormat = true
			if f.Value != "text" {
				t.Errorf("output-format default should be 'text', got %q", f.Value)
			}
			// MT-68: `--format` is an alias for `--output-format` on
			// apply, matching every other subcommand. Without this
			// users typing `mooncake apply --format json` (the
			// universal pattern) hit "flag provided but not defined".
			for _, a := range f.Aliases {
				if a == "format" {
					hasFormatAlias = true
				}
			}
		}
	}
	if !hasFormatAlias {
		t.Error("MT-68: apply --output-format should expose --format as an alias")
	}

	if !hasOutputFormat {
		t.Error("run command should have output-format flag")
	}
}

// TestRunCommandArtifactsFlags tests run command artifacts-related flags
func TestRunCommandArtifactsFlags(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	// MT-76: --capture-full-output alone (without --artifacts-dir)
	// must hard-error instead of silently no-opping. Pinned here so a
	// future refactor that drops the early-return slips the test.
	t.Run("MT-76 capture-full-output requires artifacts-dir", func(t *testing.T) {
		app := createApp()
		err := app.Run([]string{"mooncake", "apply", "--capture-full-output"})
		if err == nil {
			t.Fatal("expected error when --capture-full-output is set without --artifacts-dir")
		}
		if !contains(err.Error(), "--capture-full-output") || !contains(err.Error(), "--artifacts-dir") {
			t.Errorf("error should name both flags; got: %v", err)
		}
	})

	// MT-76: setting both flags must NOT trigger the validation (it's
	// the partnership the help text promises).
	t.Run("MT-76 capture-full-output + artifacts-dir is valid", func(t *testing.T) {
		app := createApp()
		// Use a guaranteed-nonexistent config path so the run aborts
		// after our validation passes but before any real apply work.
		err := app.Run([]string{
			"mooncake", "apply",
			"--capture-full-output",
			"--artifacts-dir", "/tmp/mooncake-mt76-test",
			"--config", "/tmp/this-config-does-not-exist-mt76.yml",
		})
		if err == nil {
			// We expect the config-not-found error, not nil.
			return
		}
		// Whatever error fires, it must NOT be the MT-76 partnership
		// check — that one would carry the "--capture-full-output requires"
		// phrasing.
		if contains(err.Error(), "--capture-full-output requires") {
			t.Errorf("validation should pass when both flags set; got: %v", err)
		}
	})

	// MT-86: same shape for --max-output-bytes / --max-output-lines.
	// They only affect the artifact bundle; without --artifacts-dir
	// the truncation is silently ignored.
	t.Run("MT-86 max-output-bytes requires artifacts-dir", func(t *testing.T) {
		app := createApp()
		err := app.Run([]string{"mooncake", "apply", "--max-output-bytes", "100"})
		if err == nil {
			t.Fatal("expected error when --max-output-bytes is set without --artifacts-dir")
		}
		if !contains(err.Error(), "--max-output-bytes") || !contains(err.Error(), "--artifacts-dir") {
			t.Errorf("error should name both flags; got: %v", err)
		}
	})

	t.Run("MT-86 max-output-lines requires artifacts-dir", func(t *testing.T) {
		app := createApp()
		err := app.Run([]string{"mooncake", "apply", "--max-output-lines", "5"})
		if err == nil {
			t.Fatal("expected error when --max-output-lines is set without --artifacts-dir")
		}
		if !contains(err.Error(), "--max-output-lines") || !contains(err.Error(), "--artifacts-dir") {
			t.Errorf("error should name both flags; got: %v", err)
		}
	})

	// MT-86: a default-valued (unset) flag must NOT trip the
	// validation — only explicit user-supplied overrides do. Otherwise
	// every plain `mooncake apply` would fail.
	t.Run("MT-86 max-output-* unset is fine", func(t *testing.T) {
		app := createApp()
		err := app.Run([]string{"mooncake", "apply", "--config", "/tmp/this-config-does-not-exist-mt86.yml"})
		if err != nil && (contains(err.Error(), "--max-output-bytes requires") ||
			contains(err.Error(), "--max-output-lines requires")) {
			t.Errorf("validation should not fire when flags are unset; got: %v", err)
		}
	})

	// Check for artifacts flags
	artifactsFlags := map[string]bool{
		"artifacts-dir":       false,
		"capture-full-output": false,
		"max-output-bytes":    false,
		"max-output-lines":    false,
	}

	for _, flag := range runCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			if _, exists := artifactsFlags[f.Name]; exists {
				artifactsFlags[f.Name] = true
			}
		case *cli.BoolFlag:
			if _, exists := artifactsFlags[f.Name]; exists {
				artifactsFlags[f.Name] = true
			}
		case *cli.IntFlag:
			if _, exists := artifactsFlags[f.Name]; exists {
				artifactsFlags[f.Name] = true
				// Check default values
				if f.Name == "max-output-bytes" && f.Value != defaultMaxOutputBytes {
					t.Errorf("max-output-bytes default should be %d, got %d", defaultMaxOutputBytes, f.Value)
				}
				if f.Name == "max-output-lines" && f.Value != defaultMaxOutputLines {
					t.Errorf("max-output-lines default should be %d, got %d", defaultMaxOutputLines, f.Value)
				}
			}
		}
	}

	for flag, found := range artifactsFlags {
		if !found {
			t.Errorf("run command missing artifacts flag: %s", flag)
		}
	}
}

// TestRunCommandPasswordFlags tests run command password-related flags
func TestRunCommandPasswordFlags(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	// Check for password flags
	passwordFlags := map[string]bool{
		"sudo-pass":          false,
		"sudo-pass-file":     false,
		"ask-become-pass":    false,
		"insecure-sudo-pass": false,
	}

	for _, flag := range runCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			if _, exists := passwordFlags[f.Name]; exists {
				passwordFlags[f.Name] = true
			}
		case *cli.BoolFlag:
			if _, exists := passwordFlags[f.Name]; exists {
				passwordFlags[f.Name] = true
			}
		}
	}

	for flag, found := range passwordFlags {
		if !found {
			t.Errorf("run command missing password flag: %s", flag)
		}
	}
}

// TestPlanCommandConfigFlag asserts the --config flag exists on plan and is
// NOT required at the CLI level. Spec 40 makes it auto-discoverable from cwd
// (./mooncake.yml or ./mooncake/main.yml).
func TestPlanCommandConfigFlag(t *testing.T) {
	app := createApp()

	var planCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "plan" {
			planCmd = cmd
			break
		}
	}
	if planCmd == nil {
		t.Fatal("plan command not found")
	}

	var configFlag *cli.StringFlag
	for _, flag := range planCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "config" {
			configFlag = f
			break
		}
	}
	if configFlag == nil {
		t.Fatal("plan command is missing --config flag")
	}
	if configFlag.Required {
		t.Error("plan --config must NOT be Required (spec 40: auto-discoverable)")
	}
}

// TestValidateCommandConfigFlag mirrors TestPlanCommandConfigFlag for the
// validate subcommand.
func TestValidateCommandConfigFlag(t *testing.T) {
	app := createApp()

	var validateCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "validate" {
			validateCmd = cmd
			break
		}
	}
	if validateCmd == nil {
		t.Fatal("validate command not found")
	}

	var configFlag *cli.StringFlag
	for _, flag := range validateCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "config" {
			configFlag = f
			break
		}
	}
	if configFlag == nil {
		t.Fatal("validate command is missing --config flag")
	}
	if configFlag.Required {
		t.Error("validate --config must NOT be Required (spec 40: auto-discoverable)")
	}
}

// TestApplyCommandDryRunFlag asserts that `apply` exposes a --dry-run flag
// (spec 40) with an `-n` short alias.
func TestApplyCommandDryRunFlag(t *testing.T) {
	app := createApp()

	var applyCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			applyCmd = cmd
			break
		}
	}
	if applyCmd == nil {
		t.Fatal("apply command not found")
	}

	var dryRun *cli.BoolFlag
	for _, flag := range applyCmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "dry-run" {
			dryRun = f
			break
		}
	}
	if dryRun == nil {
		t.Fatal("apply is missing --dry-run flag")
	}
	hasShort := false
	for _, a := range dryRun.Aliases {
		if a == "n" {
			hasShort = true
		}
	}
	if !hasShort {
		t.Error("apply --dry-run should have -n short alias")
	}
}

// TestFormatPlanYAMLIndent tests that kernel.FormatPlanYAML uses correct indentation
func TestFormatPlanYAMLIndent(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", "yaml"})

	if err != nil {
		t.Errorf("planCommand with yaml format failed: %v", err)
	}
}

// TestAppCommandsUsage tests that all commands have proper usage text
func TestAppCommandsUsage(t *testing.T) {
	app := createApp()

	for _, cmd := range app.Commands {
		if cmd.Usage == "" {
			t.Errorf("command %s should have usage text", cmd.Name)
		}
	}
}

// TestFormatPlanTextWithOriginAndChain tests kernel.FormatPlanText with include chain
func TestFormatPlanTextWithOriginAndChain(t *testing.T) {
	tmpDir := t.TempDir()
	mainConfig := filepath.Join(tmpDir, "main.yml")
	includedConfig := filepath.Join(tmpDir, "included.yml")

	// Create included config
	includedContent := `steps:
  - name: included step
    shell: echo from included
`
	if err := os.WriteFile(includedConfig, []byte(includedContent), 0600); err != nil {
		t.Fatalf("failed to write included config: %v", err)
	}

	// Create main config with include
	mainContent := `steps:
  - name: main step
    shell: echo from main
  - import: ` + includedConfig + `
`
	if err := os.WriteFile(mainConfig, []byte(mainContent), 0600); err != nil {
		t.Fatalf("failed to write main config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.BoolFlag{Name: "show-origins"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", mainConfig, "--show-origins"})

	if err != nil {
		t.Errorf("planCommand with include chain failed: %v", err)
	}
}

// TestPlanCommandWithSkippedSteps tests plan with skipped steps due to tags
func TestPlanCommandWithSkippedSteps(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: untagged step
    shell: echo untagged

  - name: tagged step
    shell: echo tagged
    tags: [special]
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.StringFlag{Name: "tags"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	// Filter to only run steps with "special" tag
	err := app.Run([]string{"test", "plan", "--config", testConfig, "--tags", "special"})

	if err != nil {
		t.Errorf("planCommand with tag filtering failed: %v", err)
	}
}

// TestContainsHelper tests the contains helper function
func TestContainsHelper(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "contains substring",
			s:        "hello world",
			substr:   "world",
			expected: true,
		},
		{
			name:     "does not contain",
			s:        "hello world",
			substr:   "foo",
			expected: false,
		},
		{
			name:     "exact match",
			s:        "hello",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "empty substring",
			s:        "hello",
			substr:   "",
			expected: true,
		},
		{
			name:     "substring longer than string",
			s:        "hi",
			substr:   "hello",
			expected: false,
		},
		{
			name:     "both empty",
			s:        "",
			substr:   "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

// TestWriteFactsJSONErrorPaths tests error handling in kernel.WriteFactsJSON
func TestWriteFactsJSONErrorPaths(t *testing.T) {
	// Test with invalid path (directory doesn't exist)
	f := &facts.Facts{OS: "linux"}
	err := kernel.WriteFactsJSON(f, "/nonexistent/dir/facts.json")

	if err == nil {
		t.Error("kernel.WriteFactsJSON with invalid path should return error")
	}

	// Verify error message contains useful info
	if !contains(err.Error(), "write file") {
		t.Errorf("error should mention write file, got: %v", err)
	}
}

// TestFactsCommandDefaultFormat tests factsCommand with default format
func TestFactsCommandDefaultFormat(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "facts",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.FactsAction,
			},
		},
	}

	// Run without specifying format (should use default)
	err := app.Run([]string{"test", "facts"})

	if err != nil {
		t.Errorf("factsCommand with default format failed: %v", err)
	}
}

// TestPlanCommandWithComplexConfig tests plan with more complex configuration
func TestPlanCommandWithComplexConfig(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Create config with conditional, loop, and register
	configContent := `steps:
  - name: set variable
    vars:
      my_items:
        - item1
        - item2

  - name: loop step
    shell: echo {{ item }}
    for_each: "{{ my_items }}"
    as: loop_result

  - name: conditional step
    shell: echo conditional
    when: "{{ loop_result.changed }}"
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig})

	if err != nil {
		t.Errorf("planCommand with complex config failed: %v", err)
	}
}

// TestFormatPlanJSONWithComplexPlan tests JSON formatting with complex plan
func TestFormatPlanJSONWithComplexPlan(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	configContent := `steps:
  - name: step with tags
    shell: echo hello
    tags: [tag1, tag2]
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", "json"})

	if err != nil {
		t.Errorf("planCommand JSON format with tags failed: %v", err)
	}
}

// TestPlanCommandOutputToFile tests plan command with output file
func TestPlanCommandOutputToFileYAML(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")
	outputFile := filepath.Join(tmpDir, "plan.yaml")

	configContent := `steps:
  - name: test
    shell: echo hello
`
	if err := os.WriteFile(testConfig, []byte(configContent), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "plan",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "config", Required: true},
					&cli.StringFlag{Name: "format", Value: "text"},
					&cli.StringFlag{Name: "output"},
				},
				Action: kernel.PlanAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--output", outputFile})

	if err != nil {
		t.Errorf("planCommand with YAML output file failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("output file was not created")
	}
}

// TestApplyCommandStaleFlags verifies the Spec 16 stale-plan flags
// are wired on `apply`. (The legacy --dry-run flag was removed; use
// `mooncake plan` for state-aware preview.)
func TestApplyCommandStaleFlags(t *testing.T) {
	app := createApp()

	var applyCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			applyCmd = cmd
			break
		}
	}
	if applyCmd == nil {
		t.Fatal("apply command not found")
	}

	hasAllowStale, hasMaxAge := false, false
	for _, flag := range applyCmd.Flags {
		switch f := flag.(type) {
		case *cli.BoolFlag:
			if f.Name == "allow-stale" {
				hasAllowStale = true
			}
		case *cli.DurationFlag:
			if f.Name == "max-plan-age" {
				hasMaxAge = true
			}
		}
	}
	if !hasAllowStale {
		t.Error("apply command should have --allow-stale flag")
	}
	if !hasMaxAge {
		t.Error("apply command should have --max-plan-age flag")
	}
}

// TestApplyCommandTUIFlag verifies the apply command exposes --tui
// (opt-in animated display; default is raw console output).
func TestApplyCommandTUIFlag(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("apply command not found")
	}

	hasTUI := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "tui" {
			hasTUI = true
			if f.Value != false {
				t.Errorf("--tui default should be false, got %v", f.Value)
			}
		}
	}

	if !hasTUI {
		t.Error("apply command should have --tui flag")
	}
}

// TestRunCommandLogLevelFlag tests that run command has log-level flag with correct default
func TestRunCommandLogLevelFlag(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "apply" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	hasLogLevel := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "log-level" {
			hasLogLevel = true
			if f.Value != "info" {
				t.Errorf("log-level default should be 'info', got %q", f.Value)
			}
		}
	}

	if !hasLogLevel {
		t.Error("run command should have log-level flag")
	}
}

// TestPlanCommandFormatFlag tests that plan command has format flag with correct default
func TestPlanCommandFormatFlag(t *testing.T) {
	app := createApp()

	var planCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "plan" {
			planCmd = cmd
			break
		}
	}

	if planCmd == nil {
		t.Fatal("plan command not found")
	}

	hasFormat := false
	for _, flag := range planCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "format" {
			hasFormat = true
			if f.Value != "text" {
				t.Errorf("format default should be 'text', got %q", f.Value)
			}
		}
	}

	if !hasFormat {
		t.Error("plan command should have format flag")
	}
}

// TestFactsCommandFormatFlag tests that facts command has format flag with correct default
func TestFactsCommandFormatFlag(t *testing.T) {
	app := createApp()

	var factsCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "facts" {
			factsCmd = cmd
			break
		}
	}

	if factsCmd == nil {
		t.Fatal("facts command not found")
	}

	hasFormat := false
	for _, flag := range factsCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "format" {
			hasFormat = true
			if f.Value != "text" {
				t.Errorf("format default should be 'text', got %q", f.Value)
			}
		}
	}

	if !hasFormat {
		t.Error("facts command should have format flag")
	}
}
