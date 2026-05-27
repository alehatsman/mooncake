package kernel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v2"
)

// runValidateAction drives validateAction through a fake cli.App so we
// can assert on the returned error chain. Before F052 this wasn't
// possible — the action called os.Exit and killed the test process.
func runValidateAction(t *testing.T, configPath string, format string) error {
	t.Helper()
	var actionErr error
	app := &cli.App{
		Flags:  ValidateCommand().Flags,
		Action: func(c *cli.Context) error { actionErr = validateAction(c); return nil },
	}
	args := []string{"test", "--config", configPath}
	if format != "" {
		args = append(args, "--format", format)
	}
	if err := app.Run(args); err != nil {
		t.Fatalf("app.Run: %v", err)
	}
	return actionErr
}

func writeValidateConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

// TestValidate_HasErrors_ReturnsValidationExitCode pins the F052 fix:
// a config with validation errors returns a cli.ExitCoder carrying
// exitCodeValidationError (2) instead of calling os.Exit(2).
func TestValidate_HasErrors_ReturnsValidationExitCode(t *testing.T) {
	// Step with two action keys — reader_test.go confirms this
	// produces a diagnostic with severity=error.
	path := writeValidateConfig(t, `- name: invalid step
  shell: echo hello
  file.write:
    path: /tmp/test
`)

	err := runValidateAction(t, path, "text")
	if err == nil {
		t.Fatal("expected error for config with validation errors, got nil")
	}

	var coder cli.ExitCoder
	if !errors.As(err, &coder) {
		t.Fatalf("expected cli.ExitCoder, got %T: %v", err, err)
	}
	if coder.ExitCode() != exitCodeValidationError {
		t.Errorf("ExitCode() = %d, want %d", coder.ExitCode(), exitCodeValidationError)
	}
}

// TestValidate_InvalidYAML_ReturnsRuntimeExitCode pins the runtime-
// error path of the F052 fix: a malformed YAML returns a cli.ExitCoder
// carrying exitCodeRuntimeError (3) instead of calling os.Exit(3).
func TestValidate_InvalidYAML_ReturnsRuntimeExitCode(t *testing.T) {
	// Tab-indented value triggers a yaml parse error inside
	// ReadConfigWithValidation, hitting the first os.Exit→cli.Exit
	// conversion.
	path := writeValidateConfig(t, "- name: bad\n\tshell: oops\n")

	err := runValidateAction(t, path, "text")
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}

	var coder cli.ExitCoder
	if !errors.As(err, &coder) {
		t.Fatalf("expected cli.ExitCoder, got %T: %v", err, err)
	}
	if coder.ExitCode() != exitCodeRuntimeError {
		t.Errorf("ExitCode() = %d, want %d", coder.ExitCode(), exitCodeRuntimeError)
	}
}

// TestValidate_Clean_ReturnsNil confirms the success path is unchanged
// by the F052 conversion — a valid config still returns nil.
func TestValidate_Clean_ReturnsNil(t *testing.T) {
	path := writeValidateConfig(t, "- name: ok\n  shell: echo hi\n")

	err := runValidateAction(t, path, "text")
	if err != nil {
		t.Errorf("expected nil for valid config, got: %v", err)
	}
}
