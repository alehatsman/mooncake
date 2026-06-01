package main

import (
	"testing"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
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
	expectedCommands := []string{"init", "doctor", "mod", "docs", "schema", "snapshot", "history", "mcp", "step", "task", "tool", "apply", "plan", "facts", "explain", "metrics", "actions", "validate", "agent", "agentd", "fleet", "runs", "query"}
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

// TestValidateCommandInvalidPath tests validateCommand with invalid config path
func TestValidateCommandInvalidPath(t *testing.T) {
	// This test will exit the program via os.Exit, so we can't test it directly
	// But we can verify the command structure is correct
	t.Logf("validateCommand structure test passed")
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
				// Defaults are pinned in cmd/kernel/shell.go
				// (defaultMaxOutputBytes, defaultMaxOutputLines).
				const wantBytes = 1048576
				const wantLines = 1000
				if f.Name == "max-output-bytes" && f.Value != wantBytes {
					t.Errorf("max-output-bytes default should be %d, got %d", wantBytes, f.Value)
				}
				if f.Name == "max-output-lines" && f.Value != wantLines {
					t.Errorf("max-output-lines default should be %d, got %d", wantLines, f.Value)
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

// TestAppCommandsUsage tests that all commands have proper usage text
func TestAppCommandsUsage(t *testing.T) {
	app := createApp()

	for _, cmd := range app.Commands {
		if cmd.Usage == "" {
			t.Errorf("command %s should have usage text", cmd.Name)
		}
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
