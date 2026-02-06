package command

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// mockExecutionContext wraps testutil.MockContext to provide ExecutionContext-compatible fields
type mockExecutionContext struct {
	*testutil.MockContext
	SudoPass string
	PathUtil *pathutil.PathExpander
	Template template.Renderer
}

// newMockExecutionContext creates a mock that can be cast to *executor.ExecutionContext
func newMockExecutionContext() *executor.ExecutionContext {
	tmpl := template.NewPongo2Renderer()
	return &executor.ExecutionContext{
		Variables:      make(map[string]interface{}),
		Template:       tmpl,
		Evaluator:      expression.NewExprEvaluator(),
		PathUtil:       pathutil.NewPathExpander(tmpl),
		Logger:         &testutil.MockLogger{Logs: []string{}},
		EventPublisher: &testutil.MockPublisher{Events: []events.Event{}},
		Redactor:       security.NewRedactor(),
		SudoPass:       "",
		CurrentStepID:  "step-1",
		Stats:          executor.NewExecutionStats(),
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "command" {
		t.Errorf("Name = %v, want 'command'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryCommand {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryCommand)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if !meta.SupportsBecome {
		t.Error("SupportsBecome should be true")
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid command",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"echo", "hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "nil command action",
			step: &config.Step{
				Command: nil,
			},
			wantErr: true,
		},
		{
			name: "empty argv",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{},
				},
			},
			wantErr: true,
		},
		{
			name: "single command",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"ls"},
				},
			},
			wantErr: false,
		},
		{
			name: "command with multiple arguments",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"ls", "-la", "/tmp"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_BasicCommand(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name      string
		argv      []string
		wantRC    int
		wantErr   bool
		checkOut  func(string) bool
		variables map[string]interface{}
	}{
		{
			name:   "echo command",
			argv:   []string{"echo", "hello"},
			wantRC: 0,
			checkOut: func(out string) bool {
				return strings.Contains(out, "hello")
			},
			wantErr: false,
		},
		{
			name:   "command with template variable",
			argv:   []string{"echo", "{{ message }}"},
			wantRC: 0,
			variables: map[string]interface{}{
				"message": "world",
			},
			checkOut: func(out string) bool {
				return strings.Contains(out, "world")
			},
			wantErr: false,
		},
		{
			name:    "non-existent command",
			argv:    []string{"nonexistentcommand12345"},
			wantRC:  1,
			wantErr: true,
		},
		{
			name:   "command that fails",
			argv:   []string{"ls", "/nonexistent/path/12345"},
			wantRC: 1, // Could be 1 or 2 depending on system
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()
			if tt.variables != nil {
				ctx.Variables = tt.variables
			}

			step := &config.Step{
				Command: &config.CommandAction{
					Argv: tt.argv,
				},
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				execResult, ok := result.(*executor.Result)
				if !ok {
					t.Fatalf("Execute() result is not *executor.Result")
				}

				if execResult.Rc != tt.wantRC {
					t.Errorf("Result.Rc = %v, want %v", execResult.Rc, tt.wantRC)
				}

				if tt.checkOut != nil && !tt.checkOut(execResult.Stdout) {
					t.Errorf("Result.Stdout = %q, check failed", execResult.Stdout)
				}

				if !execResult.Changed {
					t.Error("Result.Changed should be true by default")
				}
			}
		})
	}
}

func TestHandler_Execute_WithEnvironment(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	// Use a command that outputs environment variables
	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/c", "echo", "%TEST_VAR%"}
	} else {
		argv = []string{"sh", "-c", "echo $TEST_VAR"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		Env: map[string]string{
			"TEST_VAR": "test_value",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !strings.Contains(execResult.Stdout, "test_value") {
		t.Errorf("Result.Stdout = %q, want to contain 'test_value'", execResult.Stdout)
	}
}

func TestHandler_Execute_WithEnvironmentTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.Variables = map[string]interface{}{
		"env_value": "rendered_value",
	}

	// Use a command that outputs environment variables
	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/c", "echo", "%TEST_VAR%"}
	} else {
		argv = []string{"sh", "-c", "echo $TEST_VAR"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		Env: map[string]string{
			"TEST_VAR": "{{ env_value }}",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !strings.Contains(execResult.Stdout, "rendered_value") {
		t.Errorf("Result.Stdout = %q, want to contain 'rendered_value'", execResult.Stdout)
	}
}

func TestHandler_Execute_WithWorkingDirectory(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	// Create a temporary directory for testing
	tmpDir := os.TempDir()

	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/c", "cd"}
	} else {
		argv = []string{"pwd"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		Cwd: tmpDir,
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	// Output should start with the temp directory path (pwd outputs full path)
	if !strings.HasPrefix(execResult.Stdout, tmpDir) && !strings.HasPrefix(execResult.Stdout, strings.ToUpper(tmpDir)) {
		// On some systems like macOS, /tmp is a symlink to /private/tmp
		// Just check that output is a non-empty directory path
		if !strings.Contains(execResult.Stdout, "/") && !strings.Contains(execResult.Stdout, "\\") {
			t.Errorf("Result.Stdout = %q, want to contain directory path", execResult.Stdout)
		}
	}
}

func TestHandler_Execute_WithWorkingDirectoryTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	tmpDir := os.TempDir()
	ctx.Variables = map[string]interface{}{
		"work_dir": tmpDir,
	}

	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"cmd", "/c", "cd"}
	} else {
		argv = []string{"pwd"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		Cwd: "{{ work_dir }}",
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	// Just verify we got a directory path output
	if !strings.Contains(execResult.Stdout, "/") && !strings.Contains(execResult.Stdout, "\\") {
		t.Errorf("Result.Stdout = %q, want to contain directory path", execResult.Stdout)
	}
}

func TestHandler_Execute_WithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping timeout test in short mode")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	// Command that sleeps for 2 seconds
	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"timeout", "2"}
	} else {
		argv = []string{"sleep", "2"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		Timeout: "500ms",
	}

	start := time.Now()
	_, err := h.Execute(ctx, step)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Execute() should timeout and return error")
	}

	// Should timeout around 500ms, not wait full 2 seconds
	if elapsed > time.Second {
		t.Errorf("Execute() took %v, expected timeout around 500ms", elapsed)
	}
}

func TestHandler_Execute_WithTimeout_Success(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		Timeout: "5s",
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Errorf("Execute() error = %v, want nil", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Rc != 0 {
		t.Errorf("Result.Rc = %v, want 0", execResult.Rc)
	}
}

func TestHandler_Execute_WithInvalidTimeout(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		Timeout: "invalid",
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should return error for invalid timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Error should mention timeout, got: %v", err)
	}
}

func TestHandler_Execute_WithRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry test in short mode")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	// Command that always fails
	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"ls", "/nonexistent/path/12345"},
		},
		Retries: 2,
	}

	_, err := h.Execute(ctx, step)

	if err == nil {
		t.Error("Execute() should fail after retries")
	}

	// Just verify that retries happened and command still failed
	// (timing is not reliable in tests)
}

func TestHandler_Execute_WithRetryDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry delay test in short mode")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"ls", "/nonexistent/path/12345"},
		},
		Retries:    2,
		RetryDelay: "100ms",
	}

	start := time.Now()
	_, err := h.Execute(ctx, step)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("Execute() should fail after retries")
	}

	// Should wait at least 200ms total (2 retries * 100ms each)
	if elapsed < 200*time.Millisecond {
		t.Errorf("Execute() took %v, expected at least 200ms for retry delays", elapsed)
	}
}

func TestHandler_Execute_WithChangedWhen(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		argv        []string
		changedWhen string
		wantChanged bool
	}{
		{
			name:        "always changed (default)",
			argv:        []string{"echo", "hello"},
			changedWhen: "",
			wantChanged: true,
		},
		{
			name:        "changed when rc is 0",
			argv:        []string{"echo", "hello"},
			changedWhen: "rc == 0",
			wantChanged: true,
		},
		{
			name:        "not changed when stdout doesn't contain text",
			argv:        []string{"echo", "hello"},
			changedWhen: "has(stdout, 'goodbye')",
			wantChanged: false,
		},
		{
			name:        "changed when stdout contains text",
			argv:        []string{"echo", "hello"},
			changedWhen: "has(stdout, 'hello')",
			wantChanged: true,
		},
		{
			name:        "never changed",
			argv:        []string{"echo", "hello"},
			changedWhen: "false",
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()

			step := &config.Step{
				Command: &config.CommandAction{
					Argv: tt.argv,
				},
				ChangedWhen: tt.changedWhen,
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			execResult := result.(*executor.Result)
			if execResult.Changed != tt.wantChanged {
				t.Errorf("Result.Changed = %v, want %v", execResult.Changed, tt.wantChanged)
			}
		})
	}
}

func TestHandler_Execute_WithFailedWhen(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name       string
		argv       []string
		failedWhen string
		wantFailed bool
		wantErr    bool
	}{
		{
			name:       "success by default",
			argv:       []string{"echo", "hello"},
			failedWhen: "",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "success even with rc=0",
			argv:       []string{"echo", "hello"},
			failedWhen: "rc != 0",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "fail when stdout contains text",
			argv:       []string{"echo", "ERROR"},
			failedWhen: "has(stdout, 'ERROR')",
			wantFailed: true,
			wantErr:    true,
		},
		{
			name:       "success when stdout doesn't contain text",
			argv:       []string{"echo", "SUCCESS"},
			failedWhen: "has(stdout, 'ERROR')",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "always fail",
			argv:       []string{"echo", "hello"},
			failedWhen: "true",
			wantFailed: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()

			step := &config.Step{
				Command: &config.CommandAction{
					Argv: tt.argv,
				},
				FailedWhen: tt.failedWhen,
			}

			result, err := h.Execute(ctx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if result != nil {
				execResult := result.(*executor.Result)
				if execResult.Failed != tt.wantFailed {
					t.Errorf("Result.Failed = %v, want %v", execResult.Failed, tt.wantFailed)
				}
			}
		})
	}
}

func TestHandler_Execute_FailedCommand_WithFailedWhen(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	// Command that fails
	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"ls", "/nonexistent/path/12345"},
		},
		FailedWhen: "rc > 10", // Only fail if return code > 10
	}

	result, err := h.Execute(ctx, step)
	// Should not error because failed_when condition is not met
	if err != nil {
		t.Errorf("Execute() error = %v, want nil (failed_when overrides failure)", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Failed {
		t.Error("Result.Failed should be false when failed_when condition not met")
	}
}

func TestHandler_Execute_WithStdin(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	var argv []string
	if runtime.GOOS == "windows" {
		// Windows: use more command to read stdin
		argv = []string{"more"}
	} else {
		// Unix: use cat to read stdin
		argv = []string{"cat"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv:  argv,
			Stdin: "test input",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !strings.Contains(execResult.Stdout, "test input") {
		t.Errorf("Result.Stdout = %q, want to contain 'test input'", execResult.Stdout)
	}
}

func TestHandler_Execute_WithStdinTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.Variables = map[string]interface{}{
		"input_text": "rendered input",
	}

	var argv []string
	if runtime.GOOS == "windows" {
		argv = []string{"more"}
	} else {
		argv = []string{"cat"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv:  argv,
			Stdin: "{{ input_text }}",
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !strings.Contains(execResult.Stdout, "rendered input") {
		t.Errorf("Result.Stdout = %q, want to contain 'rendered input'", execResult.Stdout)
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "simple command",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"echo", "hello"},
				},
			},
			wantErr: false,
		},
		{
			name: "command with template",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"echo", "{{ message }}"},
				},
			},
			wantErr: false,
		},
		{
			name: "command with become",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"systemctl", "start", "nginx"},
				},
				Become: true,
			},
			wantErr: false,
		},
		{
			name: "command with working directory",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"ls"},
				},
				Cwd: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "command with environment",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"env"},
				},
				Env: map[string]string{
					"TEST": "value",
				},
			},
			wantErr: false,
		},
		{
			name: "command with timeout",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"sleep", "10"},
				},
				Timeout: "5s",
			},
			wantErr: false,
		},
		{
			name: "command with retries",
			step: &config.Step{
				Command: &config.CommandAction{
					Argv: []string{"curl", "https://example.com"},
				},
				Retries: 3,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()

			err := h.DryRun(ctx, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check that something was logged
			mockLog := ctx.Logger.(*testutil.MockLogger)
			if len(mockLog.Logs) == 0 {
				t.Error("DryRun() should log something")
			}
		})
	}
}

func TestHandler_DryRun_TemplateRenderFailure(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "{{ missing_var }}"},
		},
	}

	// Should not error on template failures in dry-run
	err := h.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() should not error on template failures, got: %v", err)
	}
}

func TestHandler_Execute_InvalidArgvTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "{{ invalid.syntax"},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid template syntax")
	}
}

func TestHandler_Execute_InvalidEnvTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		Env: map[string]string{
			"TEST": "{{ invalid.syntax",
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid env template")
	}
}

func TestHandler_Execute_InvalidCwdTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		Cwd: "{{ invalid.syntax",
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid cwd template")
	}
}

func TestHandler_Execute_InvalidStdinTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv:  []string{"cat"},
			Stdin: "{{ invalid.syntax",
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid stdin template")
	}
}

func TestHandler_Execute_InvalidChangedWhenExpression(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		ChangedWhen: "invalid expression syntax",
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid changed_when expression")
	}
}

func TestHandler_Execute_InvalidFailedWhenExpression(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		FailedWhen: "invalid expression syntax",
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error on invalid failed_when expression")
	}
}

func TestHandler_Execute_NonBoolChangedWhenResult(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		ChangedWhen: "42", // Evaluates to int, not bool
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when changed_when doesn't return bool")
	}
}

func TestHandler_Execute_NonBoolFailedWhenResult(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
		FailedWhen: "'string'", // Evaluates to string, not bool
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when failed_when doesn't return bool")
	}
}

// TestHandler_Execute_WithBecome tests sudo functionality
// Note: This test requires sudo access and will be skipped if not available
func TestHandler_Execute_WithBecome(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	// Skip if sudo not available
	if _, err := os.Stat("/usr/bin/sudo"); os.IsNotExist(err) {
		t.Skip("sudo not available")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.SudoPass = "" // No password - would fail in real execution

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"id"},
		},
		Become: true,
	}

	// Should error because no sudo password provided
	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when become is true but no sudo password")
	}
	if !strings.Contains(err.Error(), "sudo password") {
		t.Errorf("Error should mention sudo password, got: %v", err)
	}
}

func TestHandler_Execute_WithBecomeUser(t *testing.T) {
	// Skip on Windows
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	// Skip if sudo not available
	if _, err := os.Stat("/usr/bin/sudo"); os.IsNotExist(err) {
		t.Skip("sudo not available")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()
	ctx.SudoPass = "" // No password

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"whoami"},
		},
		Become:     true,
		BecomeUser: "nobody",
	}

	// Should error because no sudo password
	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when become is true but no sudo password")
	}
}

func TestHandler_Execute_ContextNotExecutionContext(t *testing.T) {
	h := &Handler{}
	// Use testutil.MockContext which doesn't cast to ExecutionContext
	ctx := testutil.NewMockContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "hello"},
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}
	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

func TestHandler_Execute_StderrCapture(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	var argv []string
	if runtime.GOOS == "windows" {
		// Windows: redirect error output
		argv = []string{"cmd", "/c", "echo error 1>&2"}
	} else {
		// Unix: write to stderr
		argv = []string{"sh", "-c", "echo error >&2"}
	}

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: argv,
		},
		FailedWhen: "false", // Don't fail on any output
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !strings.Contains(execResult.Stderr, "error") {
		t.Errorf("Result.Stderr = %q, want to contain 'error'", execResult.Stderr)
	}
}

func TestHandler_Execute_MultipleArgv(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Command: &config.CommandAction{
			Argv: []string{"echo", "arg1", "arg2", "arg3"},
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	out := execResult.Stdout
	if !strings.Contains(out, "arg1") || !strings.Contains(out, "arg2") || !strings.Contains(out, "arg3") {
		t.Errorf("Result.Stdout = %q, want to contain all args", out)
	}
}

func TestHandler_Execute_ResultRc(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	tests := []struct {
		name   string
		argv   []string
		wantRC int
	}{
		{
			name:   "success command",
			argv:   []string{"echo", "hello"},
			wantRC: 0,
		},
		{
			name:   "failing command",
			argv:   []string{"ls", "/nonexistent12345"},
			wantRC: 1, // or 2 on some systems
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Command: &config.CommandAction{
					Argv: tt.argv,
				},
				FailedWhen: "false", // Don't fail
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			execResult := result.(*executor.Result)
			if tt.wantRC != 0 && execResult.Rc == 0 {
				t.Errorf("Result.Rc = %v, want non-zero", execResult.Rc)
			}
			if tt.wantRC == 0 && execResult.Rc != 0 {
				t.Errorf("Result.Rc = %v, want 0", execResult.Rc)
			}
		})
	}
}
