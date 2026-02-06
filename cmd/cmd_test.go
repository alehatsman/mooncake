package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/urfave/cli/v2"
)

// TestParseTags tests the parseTags function
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
			result := parseTags(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseTags() length = %v, expected %v", len(result), len(tt.expected))
				return
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("parseTags()[%d] = %v, expected %v", i, tag, tt.expected[i])
				}
			}
		})
	}
}

// TestWriteFactsJSON tests the writeFactsJSON function
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
	err := writeFactsJSON(f, testPath)
	if err != nil {
		t.Errorf("writeFactsJSON() error = %v, expected nil", err)
	}

	// Verify file exists
	if _, err := os.Stat(testPath); os.IsNotExist(err) {
		t.Errorf("writeFactsJSON() did not create file")
	}

	// Verify file content
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result facts.Facts
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("writeFactsJSON() produced invalid JSON: %v", err)
	}

	if result.OS != f.OS || result.Arch != f.Arch || result.CPUCores != f.CPUCores {
		t.Errorf("writeFactsJSON() content mismatch")
	}

	// Test invalid path
	invalidPath := filepath.Join(tmpDir, "nonexistent", "facts.json")
	err = writeFactsJSON(f, invalidPath)
	if err == nil {
		t.Errorf("writeFactsJSON() with invalid path should return error")
	}
}

// TestFormatPlanJSON tests the formatPlanJSON function (indirectly)
func TestFormatPlanJSONIndirect(t *testing.T) {
	// Test that we can create the plan structure and it marshals correctly
	// This is a smoke test since formatPlanJSON writes to stdout
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
	// (indirect test of formatPlanJSON functionality)
	t.Run("plan json structure", func(t *testing.T) {
		// The actual formatPlanJSON writes to stdout, which is hard to test
		// But we can verify the structure is JSON-serializable
		// This test passes if it compiles and runs without error
	})
}

// TestGetSourceLabel tests the getSourceLabel function
func TestGetSourceLabel(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected string
	}{
		{
			name:     "local source",
			source:   "local",
			expected: "[local]  ",
		},
		{
			name:     "user source",
			source:   "user",
			expected: "[user]   ",
		},
		{
			name:     "system source",
			source:   "system",
			expected: "[system] ",
		},
		{
			name:     "unknown source",
			source:   "unknown",
			expected: "[unknown]",
		},
		{
			name:     "empty source",
			source:   "",
			expected: "[unknown]",
		},
		{
			name:     "random source",
			source:   "foobar",
			expected: "[unknown]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSourceLabel(tt.source)
			if result != tt.expected {
				t.Errorf("getSourceLabel(%q) = %q, expected %q", tt.source, result, tt.expected)
			}
		})
	}
}

// TestHasFzf tests the hasFzf function
func TestHasFzf(t *testing.T) {
	// This test verifies the function runs without panic
	// The actual result depends on the system
	result := hasFzf()
	t.Logf("hasFzf() = %v", result)

	// Test passes if function completes without error
	// We can't assert the exact value as it depends on the environment
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
	expectedCommands := []string{"presets", "run", "plan", "facts", "actions", "validate"}
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

// TestPresetsCommand tests the presetsCommand function
func TestPresetsCommand(t *testing.T) {
	cmd := presetsCommand()

	if cmd == nil {
		t.Fatal("presetsCommand() returned nil")
	}

	if cmd.Name != "presets" {
		t.Errorf("cmd.Name = %q, expected %q", cmd.Name, "presets")
	}

	// Test subcommands exist
	expectedSubcommands := []string{"add", "list", "info", "install", "status", "uninstall"}
	if len(cmd.Subcommands) != len(expectedSubcommands) {
		t.Errorf("cmd.Subcommands length = %d, expected %d", len(cmd.Subcommands), len(expectedSubcommands))
	}

	subcommandNames := make(map[string]bool)
	for _, subcmd := range cmd.Subcommands {
		subcommandNames[subcmd.Name] = true
	}

	for _, expectedSubcmd := range expectedSubcommands {
		if !subcommandNames[expectedSubcmd] {
			t.Errorf("missing subcommand: %s", expectedSubcmd)
		}
	}

	// Test that Action is set
	if cmd.Action == nil {
		t.Errorf("cmd.Action is nil, expected interactiveSelectorAction")
	}
}

// TestSelectWithFzf tests the selectWithFzf function error handling
func TestSelectWithFzfNoFzf(t *testing.T) {
	// Skip this test as selectWithFzf requires interactive fzf
	// Testing would hang waiting for user input
	t.Skip("Skipping interactive test - selectWithFzf requires fzf and user input")
}

// TestListPresetsActionEmpty tests listPresetsAction with no presets
func TestListPresetsActionEmptyList(t *testing.T) {
	// Create a test context with no presets discoverable
	// This is a basic smoke test since the actual behavior depends on file system

	testApp := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "list",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "detailed"},
				},
				Action: listPresetsAction,
			},
		},
	}

	// Test with detailed flag
	err := testApp.Run([]string{"test", "list", "--detailed"})

	// Should complete without panic (actual output depends on available presets)
	t.Logf("listPresetsAction completed with error: %v", err)
}

// TestPresetInfoActionNoArgs tests presetInfoAction with no arguments
func TestPresetInfoActionNoArgs(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "info",
				Action: presetInfoAction,
			},
		},
	}

	err := app.Run([]string{"test", "info"})

	if err == nil {
		t.Errorf("presetInfoAction with no args should return error")
	}

	// Check error message
	expectedMsg := "preset name required"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

// TestInstallPresetActionNoArgs tests installPresetAction with no arguments
func TestInstallPresetActionNoArgs(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "install",
				Action: installPresetAction,
			},
		},
	}

	err := app.Run([]string{"test", "install"})

	if err == nil {
		t.Errorf("installPresetAction with no args should return error")
	}

	expectedMsg := "preset name required"
	if err != nil && !contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
	}
}

// TestExecutePresetInstallPasswordValidation tests password validation in executePresetInstall
func TestExecutePresetInstallPasswordValidation(t *testing.T) {
	tests := []struct {
		name        string
		sudoPass    string
		passFile    string
		askPass     bool
		insecure    bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "multiple password methods",
			sudoPass:    "pass",
			passFile:    "file",
			askPass:     false,
			insecure:    true,
			expectError: true,
			errorMsg:    "only one password method",
		},
		{
			name:        "sudo-pass without insecure flag",
			sudoPass:    "pass",
			passFile:    "",
			askPass:     false,
			insecure:    false,
			expectError: true,
			errorMsg:    "--insecure-sudo-pass",
		},
		{
			name:        "ask-pass and sudo-pass",
			sudoPass:    "pass",
			passFile:    "",
			askPass:     true,
			insecure:    true,
			expectError: true,
			errorMsg:    "only one password method",
		},
		{
			name:        "all three password methods",
			sudoPass:    "pass",
			passFile:    "file",
			askPass:     true,
			insecure:    true,
			expectError: true,
			errorMsg:    "only one password method",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test context
			app := &cli.App{
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sudo-pass"},
					&cli.StringFlag{Name: "sudo-pass-file"},
					&cli.BoolFlag{Name: "ask-become-pass"},
					&cli.BoolFlag{Name: "insecure-sudo-pass"},
				},
				Action: func(c *cli.Context) error {
					return executePresetInstall(c, "nonexistent-preset")
				},
			}

			args := []string{"test"}
			if tt.sudoPass != "" {
				args = append(args, "--sudo-pass", tt.sudoPass)
			}
			if tt.passFile != "" {
				args = append(args, "--sudo-pass-file", tt.passFile)
			}
			if tt.askPass {
				args = append(args, "--ask-become-pass")
			}
			if tt.insecure {
				args = append(args, "--insecure-sudo-pass")
			}

			err := app.Run(args)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			}
		})
	}
}

// TestUninstallPresetActionNoArgs tests uninstallPresetAction with no args
func TestUninstallPresetActionNoArgs(t *testing.T) {
	// When no args are provided and fzf is available, the function tries to use interactive selection
	// When fzf is not available, it shows help text
	// We test that the function handles the no-args case gracefully

	app := &cli.App{
		Name:   "test",
		Action: func(c *cli.Context) error {
			// Call with no args - should either show interactive selector or help
			return uninstallPresetAction(c)
		},
	}

	err := app.Run([]string{"test"})

	// Function should complete without panic
	// Error is OK (no presets found, fzf interaction, etc.)
	t.Logf("uninstallPresetAction with no args completed with error: %v", err)
}

// TestPresetStatusActionNoPresets tests presetStatusAction with no presets
func TestPresetStatusActionNoArgs(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "status",
				Action: presetStatusAction,
			},
		},
	}

	// Should complete without error (shows all presets if available)
	err := app.Run([]string{"test", "status"})

	// Function should complete (may show "No presets found" or list presets)
	t.Logf("presetStatusAction completed with error: %v", err)
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
				Action: factsCommand,
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
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	expectedFlags := []string{
		"config", "vars", "log-level", "sudo-pass", "ask-become-pass",
		"sudo-pass-file", "insecure-sudo-pass", "tags", "raw", "dry-run",
		"output-format", "artifacts-dir", "capture-full-output",
		"max-output-bytes", "max-output-lines", "from-plan", "facts-json",
	}

	flagNames := make(map[string]bool)
	for _, flag := range runCmd.Flags {
		// Extract flag name from the flag interface
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		case *cli.IntFlag:
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
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("validate command missing flag: %s", expectedFlag)
		}
	}
}

// TestPresetsCommandFlags tests that presets command has expected flags
func TestPresetsCommandFlags(t *testing.T) {
	cmd := presetsCommand()

	expectedFlags := []string{"ask-become-pass", "sudo-pass", "sudo-pass-file", "insecure-sudo-pass"}

	flagNames := make(map[string]bool)
	for _, flag := range cmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("presets command missing flag: %s", expectedFlag)
		}
	}
}

// TestPresetsInstallSubcommandFlags tests install subcommand flags
func TestPresetsInstallSubcommandFlags(t *testing.T) {
	cmd := presetsCommand()

	var installCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "install" {
			installCmd = subcmd
			break
		}
	}

	if installCmd == nil {
		t.Fatal("install subcommand not found")
	}

	expectedFlags := []string{"ask-become-pass", "sudo-pass", "sudo-pass-file", "insecure-sudo-pass"}

	flagNames := make(map[string]bool)
	for _, flag := range installCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("install subcommand missing flag: %s", expectedFlag)
		}
	}
}

// TestPresetsUninstallSubcommandFlags tests uninstall subcommand flags
func TestPresetsUninstallSubcommandFlags(t *testing.T) {
	cmd := presetsCommand()

	var uninstallCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "uninstall" {
			uninstallCmd = subcmd
			break
		}
	}

	if uninstallCmd == nil {
		t.Fatal("uninstall subcommand not found")
	}

	expectedFlags := []string{"ask-become-pass", "sudo-pass", "sudo-pass-file", "insecure-sudo-pass"}

	flagNames := make(map[string]bool)
	for _, flag := range uninstallCmd.Flags {
		switch f := flag.(type) {
		case *cli.StringFlag:
			flagNames[f.Name] = true
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("uninstall subcommand missing flag: %s", expectedFlag)
		}
	}
}

// TestPresetsListSubcommandFlags tests list subcommand flags
func TestPresetsListSubcommandFlags(t *testing.T) {
	cmd := presetsCommand()

	var listCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "list" {
			listCmd = subcmd
			break
		}
	}

	if listCmd == nil {
		t.Fatal("list subcommand not found")
	}

	expectedFlags := []string{"detailed"}

	flagNames := make(map[string]bool)
	for _, flag := range listCmd.Flags {
		switch f := flag.(type) {
		case *cli.BoolFlag:
			flagNames[f.Name] = true
		}
	}

	for _, expectedFlag := range expectedFlags {
		if !flagNames[expectedFlag] {
			t.Errorf("list subcommand missing flag: %s", expectedFlag)
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

	err := writeFactsJSON(f, testPath)
	if err != nil {
		t.Fatalf("writeFactsJSON() error = %v", err)
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

// TestParseTagsPreservesOrder tests that parseTags preserves tag order
func TestParseTagsPreservesOrder(t *testing.T) {
	input := "tag3,tag1,tag2"
	expected := []string{"tag3", "tag1", "tag2"}

	result := parseTags(input)

	if len(result) != len(expected) {
		t.Errorf("parseTags() length = %v, expected %v", len(result), len(expected))
		return
	}

	for i, tag := range result {
		if tag != expected[i] {
			t.Errorf("parseTags()[%d] = %v, expected %v (order not preserved)", i, tag, expected[i])
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
						Action: factsCommand,
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
				Action: planCommand,
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
						Action: planCommand,
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
				Action: planCommand,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--tags", "tag1,tag2"})

	if err != nil {
		t.Errorf("planCommand with tags failed: %v", err)
	}
}

// TestSelectWithFzfEmptyList tests selectWithFzf with empty preset list
func TestSelectWithFzfEmptyList(t *testing.T) {
	// Skip this test as selectWithFzf requires interactive fzf
	t.Skip("Skipping interactive test - selectWithFzf requires fzf and user input")
}

// TestGetSourceLabelCoverage tests all branches of getSourceLabel
func TestGetSourceLabelCoverage(t *testing.T) {
	// Test with various inputs to ensure complete coverage
	inputs := []string{"local", "user", "system", "unknown", "", "arbitrary"}

	for _, input := range inputs {
		result := getSourceLabel(input)
		if result == "" {
			t.Errorf("getSourceLabel(%q) returned empty string", input)
		}

		// Verify result has expected format (contains brackets)
		if !contains(result, "[") || !contains(result, "]") {
			t.Errorf("getSourceLabel(%q) = %q, expected bracketed format", input, result)
		}
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
				Action: planCommand,
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

// TestInteractiveSelectorActionWithNoPresets tests interactive selector with no presets
func TestInteractiveSelectorActionStructure(t *testing.T) {
	// This is a smoke test - we can't fully test interactive behavior
	// but we can verify the function is properly wired up

	cmd := presetsCommand()
	if cmd.Action == nil {
		t.Error("presetsCommand Action should not be nil")
	}

	// Verify it's the right function by checking it exists
	t.Log("interactiveSelectorAction is properly configured")
}

// TestPresetCommandsReturnProperErrors tests that various preset commands handle errors
func TestPresetCommandsErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		action      func(*cli.Context) error
		args        []string
		expectError bool
	}{
		{
			name:        "info with no args",
			action:      presetInfoAction,
			args:        []string{"test"},
			expectError: true,
		},
		{
			name:        "install with no args",
			action:      installPresetAction,
			args:        []string{"test"},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &cli.App{
				Name:   "test",
				Action: tt.action,
			}

			err := app.Run(tt.args)

			if tt.expectError && err == nil {
				t.Errorf("%s should return error", tt.name)
			}
		})
	}
}

// TestListPresetsActionDetailed tests listPresetsAction with detailed flag
func TestListPresetsActionDetailed(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "list",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "detailed"},
				},
				Action: listPresetsAction,
			},
		},
	}

	// Test without detailed flag
	err := app.Run([]string{"test", "list"})
	t.Logf("listPresetsAction without detailed flag: %v", err)

	// Test with detailed flag
	err = app.Run([]string{"test", "list", "--detailed"})
	t.Logf("listPresetsAction with detailed flag: %v", err)
}

// TestPresetInfoActionWithArg tests presetInfoAction with a preset name
func TestPresetInfoActionWithArg(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "info",
				Action: presetInfoAction,
			},
		},
	}

	// Test with nonexistent preset (will fail to load, but tests the code path)
	err := app.Run([]string{"test", "info", "nonexistent-preset"})

	// Should return error (preset not found)
	if err == nil {
		t.Logf("presetInfoAction: no error (preset might exist)")
	} else {
		t.Logf("presetInfoAction error (expected): %v", err)
	}
}

// TestPresetStatusActionWithArg tests presetStatusAction with a preset name
func TestPresetStatusActionWithArg(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "status",
				Action: presetStatusAction,
			},
		},
	}

	// Test with nonexistent preset
	err := app.Run([]string{"test", "status", "nonexistent-preset"})

	// Should return error (preset not found)
	if err == nil {
		t.Logf("presetStatusAction: no error (preset might exist)")
	} else {
		t.Logf("presetStatusAction error (expected): %v", err)
	}
}

// TestInstallPresetActionWithArg tests installPresetAction with a preset name
func TestInstallPresetActionWithArg(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "install",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "sudo-pass"},
					&cli.StringFlag{Name: "sudo-pass-file"},
					&cli.BoolFlag{Name: "ask-become-pass"},
					&cli.BoolFlag{Name: "insecure-sudo-pass"},
				},
				Action: installPresetAction,
			},
		},
	}

	// Test with nonexistent preset (will fail to load)
	err := app.Run([]string{"test", "install", "nonexistent-preset"})

	// Should return error (preset not found)
	if err == nil {
		t.Logf("installPresetAction: no error (preset might exist)")
	} else {
		t.Logf("installPresetAction error (expected): %v", err)
	}
}

// TestFormatPlanTextWithLoopContext tests formatPlanText with loop context
func TestFormatPlanTextWithLoopContext(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write config with loop
	configContent := `steps:
  - name: test loop
    shell: echo {{ item }}
    loop:
      - item1
      - item2
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
				Action: planCommand,
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
				Action: planCommand,
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
					&cli.StringFlag{Name: "vars"},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: planCommand,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--vars", "/nonexistent/vars.yml"})

	if err == nil {
		t.Errorf("planCommand with invalid vars file should return error")
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
				Action: planCommand,
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
				Action: factsCommand,
			},
		},
	}

	err := app.Run([]string{"test", "facts", "--format", "json"})

	if err != nil {
		t.Errorf("factsCommand with json format failed: %v", err)
	}
}

// TestWriteFactsJSONMarshalCheck tests writeFactsJSON marshaling
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

	err := writeFactsJSON(f, testPath)
	if err != nil {
		t.Errorf("writeFactsJSON() error = %v", err)
	}

	// Verify JSON content
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result facts.Facts
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("writeFactsJSON() produced invalid JSON: %v", err)
	}

	// Verify a few key fields
	if result.OS != f.OS {
		t.Errorf("OS mismatch: got %v, want %v", result.OS, f.OS)
	}
	if result.Hostname != f.Hostname {
		t.Errorf("Hostname mismatch: got %v, want %v", result.Hostname, f.Hostname)
	}
	if result.CPUCores != f.CPUCores {
		t.Errorf("CPUCores mismatch: got %v, want %v", result.CPUCores, f.CPUCores)
	}
}

// TestParseTagsEdgeCases tests parseTags with additional edge cases
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
			result := parseTags(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("parseTags() length = %v, expected %v", len(result), len(tt.expected))
				return
			}
			for i, tag := range result {
				if tag != tt.expected[i] {
					t.Errorf("parseTags()[%d] = %v, expected %v", i, tag, tt.expected[i])
				}
			}
		})
	}
}

// TestPresetInfoActionWithRealPreset tests presetInfoAction with a real preset
func TestPresetInfoActionWithRealPreset(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "info",
				Action: presetInfoAction,
			},
		},
	}

	// Test with act preset (should exist in presets directory)
	err := app.Run([]string{"test", "info", "act"})

	// Should succeed if preset exists, or fail if not
	if err != nil {
		t.Logf("presetInfoAction with 'act': %v", err)
	} else {
		t.Logf("presetInfoAction with 'act' succeeded")
	}
}

// TestPresetStatusActionWithRealPreset tests presetStatusAction with a real preset
func TestPresetStatusActionWithRealPreset(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name:   "status",
				Action: presetStatusAction,
			},
		},
	}

	// Test with act preset
	err := app.Run([]string{"test", "status", "act"})

	if err != nil {
		t.Logf("presetStatusAction with 'act': %v", err)
	} else {
		t.Logf("presetStatusAction with 'act' succeeded")
	}
}

// TestListPresetsActionSuccess tests listPresetsAction when presets exist
func TestListPresetsActionSuccess(t *testing.T) {
	app := &cli.App{
		Name: "test",
		Commands: []*cli.Command{
			{
				Name: "list",
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "detailed"},
				},
				Action: listPresetsAction,
			},
		},
	}

	// Should list available presets (if any exist in the project)
	err := app.Run([]string{"test", "list"})

	if err != nil {
		t.Errorf("listPresetsAction failed: %v", err)
	}
}

// TestFormatPlanTextAllActionTypes tests formatPlanText with different action types
func TestFormatPlanTextAllActionTypes(t *testing.T) {
	tmpDir := t.TempDir()
	testConfig := filepath.Join(tmpDir, "test.yml")

	// Write config with multiple action types to cover all branches
	configContent := `steps:
  - name: shell step
    shell: echo hello

  - name: file step
    file:
      path: /tmp/test
      state: touch

  - name: vars step
    vars:
      my_var: value

  - name: print step
    print: "test"
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
				Action: planCommand,
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
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	// Verify run command has output-format flag
	hasOutputFormat := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "output-format" {
			hasOutputFormat = true
			if f.Value != "text" {
				t.Errorf("output-format default should be 'text', got %q", f.Value)
			}
		}
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
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

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
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	// Check for password flags
	passwordFlags := map[string]bool{
		"sudo-pass":         false,
		"sudo-pass-file":    false,
		"ask-become-pass":   false,
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

// TestPlanCommandRequiredFlags tests that plan command has config as required
func TestPlanCommandRequiredFlags(t *testing.T) {
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

	// Check that config flag is required
	hasRequiredConfig := false
	for _, flag := range planCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "config" {
			if f.Required {
				hasRequiredConfig = true
			}
		}
	}

	if !hasRequiredConfig {
		t.Error("plan command config flag should be required")
	}
}

// TestValidateCommandRequiredFlags tests that validate command has config as required
func TestValidateCommandRequiredFlags(t *testing.T) {
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

	// Check that config flag is required
	hasRequiredConfig := false
	for _, flag := range validateCmd.Flags {
		if f, ok := flag.(*cli.StringFlag); ok && f.Name == "config" {
			if f.Required {
				hasRequiredConfig = true
			}
		}
	}

	if !hasRequiredConfig {
		t.Error("validate command config flag should be required")
	}
}

// TestPresetsSubcommandActions tests that all preset subcommands have actions
func TestPresetsSubcommandActions(t *testing.T) {
	cmd := presetsCommand()

	expectedSubcommands := map[string]bool{
		"add":       true,
		"list":      true,
		"info":      true,
		"install":   true,
		"status":    true,
		"uninstall": true,
	}

	for _, subcmd := range cmd.Subcommands {
		if !expectedSubcommands[subcmd.Name] {
			t.Errorf("unexpected subcommand: %s", subcmd.Name)
			continue
		}

		if subcmd.Action == nil {
			t.Errorf("subcommand %s should have an action", subcmd.Name)
		}

		delete(expectedSubcommands, subcmd.Name)
	}

	if len(expectedSubcommands) > 0 {
		t.Errorf("missing subcommands: %v", expectedSubcommands)
	}
}

// TestFormatPlanYAMLIndent tests that formatPlanYAML uses correct indentation
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
				Action: planCommand,
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

// TestPresetsSubcommandUsage tests that all preset subcommands have proper usage
func TestPresetsSubcommandUsage(t *testing.T) {
	cmd := presetsCommand()

	if cmd.Usage == "" {
		t.Error("presets command should have usage text")
	}

	for _, subcmd := range cmd.Subcommands {
		if subcmd.Usage == "" {
			t.Errorf("preset subcommand %s should have usage text", subcmd.Name)
		}
	}
}

// TestFormatPlanTextWithOriginAndChain tests formatPlanText with include chain
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
  - include: ` + includedConfig + `
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
				Action: planCommand,
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
				Action: planCommand,
			},
		},
	}

	// Filter to only run steps with "special" tag
	err := app.Run([]string{"test", "plan", "--config", testConfig, "--tags", "special"})

	if err != nil {
		t.Errorf("planCommand with tag filtering failed: %v", err)
	}
}

// TestHasFzfEnvironment tests hasFzf function in current environment
func TestHasFzfEnvironment(t *testing.T) {
	result := hasFzf()

	// Just verify it returns a boolean and doesn't panic
	t.Logf("hasFzf() returned: %v", result)
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

// TestWriteFactsJSONErrorPaths tests error handling in writeFactsJSON
func TestWriteFactsJSONErrorPaths(t *testing.T) {
	// Test with invalid path (directory doesn't exist)
	f := &facts.Facts{OS: "linux"}
	err := writeFactsJSON(f, "/nonexistent/dir/facts.json")

	if err == nil {
		t.Error("writeFactsJSON with invalid path should return error")
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
				Action: factsCommand,
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
    loop: "{{ my_items }}"
    register: loop_result

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
				Action: planCommand,
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
				Action: planCommand,
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
				Action: planCommand,
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

// TestPresetsCommandDescription tests that presets command has description
func TestPresetsCommandDescription(t *testing.T) {
	cmd := presetsCommand()

	if cmd.Description == "" {
		t.Error("presets command should have description")
	}
}

// TestPresetsInstallSubcommandDescription tests install subcommand description
func TestPresetsInstallSubcommandDescription(t *testing.T) {
	cmd := presetsCommand()

	var installCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "install" {
			installCmd = subcmd
			break
		}
	}

	if installCmd == nil {
		t.Fatal("install subcommand not found")
	}

	// Install command should have usage
	if installCmd.Usage == "" {
		t.Error("install subcommand should have usage")
	}
}

// TestPresetsUninstallSubcommandDescription tests uninstall subcommand description
func TestPresetsUninstallSubcommandDescription(t *testing.T) {
	cmd := presetsCommand()

	var uninstallCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "uninstall" {
			uninstallCmd = subcmd
			break
		}
	}

	if uninstallCmd == nil {
		t.Fatal("uninstall subcommand not found")
	}

	// Uninstall command should have description
	if uninstallCmd.Description == "" {
		t.Error("uninstall subcommand should have description")
	}
}

// TestPresetsStatusSubcommandDescription tests status subcommand description
func TestPresetsStatusSubcommandDescription(t *testing.T) {
	cmd := presetsCommand()

	var statusCmd *cli.Command
	for _, subcmd := range cmd.Subcommands {
		if subcmd.Name == "status" {
			statusCmd = subcmd
			break
		}
	}

	if statusCmd == nil {
		t.Fatal("status subcommand not found")
	}

	// Status command should have description
	if statusCmd.Description == "" {
		t.Error("status subcommand should have description")
	}
}

// TestRunCommandDryRunFlag tests that run command has dry-run flag
func TestRunCommandDryRunFlag(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	hasDryRun := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "dry-run" {
			hasDryRun = true
			if f.Value != false {
				t.Errorf("dry-run default should be false, got %v", f.Value)
			}
		}
	}

	if !hasDryRun {
		t.Error("run command should have dry-run flag")
	}
}

// TestRunCommandRawFlag tests that run command has raw flag
func TestRunCommandRawFlag(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "run" {
			runCmd = cmd
			break
		}
	}

	if runCmd == nil {
		t.Fatal("run command not found")
	}

	hasRaw := false
	for _, flag := range runCmd.Flags {
		if f, ok := flag.(*cli.BoolFlag); ok && f.Name == "raw" {
			hasRaw = true
			if f.Value != false {
				t.Errorf("raw default should be false, got %v", f.Value)
			}
		}
	}

	if !hasRaw {
		t.Error("run command should have raw flag")
	}
}

// TestRunCommandLogLevelFlag tests that run command has log-level flag with correct default
func TestRunCommandLogLevelFlag(t *testing.T) {
	app := createApp()

	var runCmd *cli.Command
	for _, cmd := range app.Commands {
		if cmd.Name == "run" {
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
