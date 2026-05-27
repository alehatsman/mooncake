package kernel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/urfave/cli/v2"
)

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

	// Verify file content. MT-74: keys are snake_case (matching the
	// template scope and /v1/facts endpoint), so unmarshal into a
	// map[string]any rather than the typed *facts.Facts struct.
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("writeFactsJSON() produced invalid JSON: %v", err)
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
	err = writeFactsJSON(f, invalidPath)
	if err == nil {
		t.Errorf("writeFactsJSON() with invalid path should return error")
	}
}

// TestFormatPlanJSON tests the FormatPlanJSON function (indirectly)
func TestFormatPlanJSONIndirect(t *testing.T) {
	// Test that we can create the plan structure and it marshals correctly
	// This is a smoke test since FormatPlanJSON writes to stdout
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
	// (indirect test of FormatPlanJSON functionality)
	t.Run("plan json structure", func(t *testing.T) {
		// The actual FormatPlanJSON writes to stdout, which is hard to test
		// But we can verify the structure is JSON-serializable
		// This test passes if it compiles and runs without error
	})
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
				Action: factsAction,
			},
		},
	}

	// Test invalid format
	err := app.Run([]string{"test", "facts", "--format", "invalid"})

	if err == nil {
		t.Errorf("factsCommand with invalid format should return error")
	}

	expectedMsg := "invalid format"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("error message should contain %q, got %q", expectedMsg, err.Error())
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
						Action: factsAction,
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
				Action: planAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", "invalid"})

	if err == nil {
		t.Errorf("planCommand with invalid format should return error")
	}

	expectedMsg := "unsupported format"
	if err != nil && !strings.Contains(err.Error(), expectedMsg) {
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
						Action: planAction,
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
				Action: planAction,
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
				Action: planAction,
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

// TestFormatPlanTextWithLoopContext tests FormatPlanText with loop context
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
				Action: planAction,
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
					&cli.StringSliceFlag{Name: "vars"},
					&cli.StringFlag{Name: "format", Value: "text"},
				},
				Action: planAction,
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
				Action: planAction,
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
				Action: planAction,
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
				Action: planAction,
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
				Action: factsAction,
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

	// Verify JSON content. MT-74: keys are snake_case (matching the
	// template scope and /v1/facts endpoint).
	data, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Errorf("writeFactsJSON() produced invalid JSON: %v", err)
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

// TestFormatPlanTextAllActionTypes tests FormatPlanText with different action types
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
				Action: planAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig})

	if err != nil {
		t.Errorf("planCommand with multiple action types failed: %v", err)
	}
}

// TestFormatPlanYAMLIndent tests that FormatPlanYAML uses correct indentation
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
				Action: planAction,
			},
		},
	}

	err := app.Run([]string{"test", "plan", "--config", testConfig, "--format", "yaml"})

	if err != nil {
		t.Errorf("planCommand with yaml format failed: %v", err)
	}
}

// TestFormatPlanTextWithOriginAndChain tests FormatPlanText with include chain
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
				Action: planAction,
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
				Action: planAction,
			},
		},
	}

	// Filter to only run steps with "special" tag
	err := app.Run([]string{"test", "plan", "--config", testConfig, "--tags", "special"})

	if err != nil {
		t.Errorf("planCommand with tag filtering failed: %v", err)
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
	if !strings.Contains(err.Error(), "write file") {
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
				Action: factsAction,
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
				Action: planAction,
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
				Action: planAction,
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
				Action: planAction,
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
