package shell

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

// newMockExecutionContext creates a mock ExecutionContext for shell tests
func newMockExecutionContext() *executor.ExecutionContext {
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
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

	if meta.Name != "shell" {
		t.Errorf("Name = %v, want 'shell'", meta.Name)
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
	if len(meta.EmitsEvents) != 2 {
		t.Errorf("EmitsEvents = %v, want 2 events", len(meta.EmitsEvents))
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
			name: "valid shell command",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
			},
			wantErr: false,
		},
		{
			name: "nil shell action",
			step: &config.Step{
				Shell: nil,
			},
			wantErr: true,
		},
		{
			name: "empty command",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "",
				},
			},
			wantErr: true,
		},
		{
			name: "valid timeout",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
				Timeout: "5s",
			},
			wantErr: false,
		},
		{
			name: "invalid timeout",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
				Timeout: "invalid",
			},
			wantErr: true,
		},
		{
			name: "valid retry_delay",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
				RetryDelay: "1s",
			},
			wantErr: false,
		},
		{
			name: "invalid retry_delay",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
				RetryDelay: "invalid",
			},
			wantErr: true,
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
		cmd       string
		wantRC    int
		wantErr   bool
		checkOut  func(string) bool
		variables map[string]interface{}
	}{
		{
			name:   "echo command",
			cmd:    "echo hello",
			wantRC: 0,
			checkOut: func(out string) bool {
				return strings.Contains(out, "hello")
			},
			wantErr: false,
		},
		{
			name:   "command with template variable",
			cmd:    "echo {{ message }}",
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
			cmd:     "nonexistentcommand12345",
			wantRC:  127,
			wantErr: true,
		},
		{
			name:    "command that fails",
			cmd:     "ls /nonexistent/path/12345",
			wantRC:  1,
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
				Shell: &config.ShellAction{
					Cmd: tt.cmd,
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

func TestHandler_Execute_WithInterpreter(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		interpreter string
		cmd         string
		skipWindows bool
		skipUnix    bool
	}{
		{
			name:        "bash interpreter",
			interpreter: "bash",
			cmd:         "echo hello",
			skipWindows: true,
		},
		{
			name:        "sh interpreter",
			interpreter: "sh",
			cmd:         "echo hello",
			skipWindows: true,
		},
		{
			name:     "pwsh interpreter",
			interpreter: "pwsh",
			cmd:      "Write-Output hello",
			skipUnix: true, // Only test on Windows
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipWindows && runtime.GOOS == "windows" {
				t.Skip("Skipping on Windows")
			}
			if tt.skipUnix && runtime.GOOS != "windows" {
				t.Skip("Skipping on Unix")
			}

			ctx := newMockExecutionContext()
			step := &config.Step{
				Shell: &config.ShellAction{
					Cmd:         tt.cmd,
					Interpreter: tt.interpreter,
				},
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				// Interpreter might not be installed, skip test
				t.Skipf("Execute() error = %v (interpreter may not be available)", err)
			}

			execResult := result.(*executor.Result)
			if execResult.Rc != 0 {
				t.Errorf("Result.Rc = %v, want 0", execResult.Rc)
			}
		})
	}
}

func TestHandler_Execute_WithEnvironment(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo %TEST_VAR%"
	} else {
		cmd = "echo $TEST_VAR"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
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

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo %TEST_VAR%"
	} else {
		cmd = "echo $TEST_VAR"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
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

	tmpDir := os.TempDir()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd"
	} else {
		cmd = "pwd"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
		},
		Cwd: tmpDir,
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

func TestHandler_Execute_WithWorkingDirectoryTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	tmpDir := os.TempDir()
	ctx.Variables = map[string]interface{}{
		"work_dir": tmpDir,
	}

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd"
	} else {
		cmd = "pwd"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
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

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "timeout 2"
	} else {
		cmd = "sleep 2"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: "ls /nonexistent/path/12345",
		},
		Retries: 2,
	}

	_, err := h.Execute(ctx, step)

	if err == nil {
		t.Error("Execute() should fail after retries")
	}

	if !strings.Contains(err.Error(), "after") {
		t.Errorf("Error should mention failed attempts, got: %v", err)
	}
}

func TestHandler_Execute_WithRetryDelay(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping retry delay test in short mode")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: "ls /nonexistent/path/12345",
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

func TestHandler_Execute_WithStdin(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "more"
	} else {
		cmd = "cat"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd:   cmd,
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

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "more"
	} else {
		cmd = "cat"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd:   cmd,
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

func TestHandler_Execute_WithChangedWhen(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		cmd         string
		changedWhen string
		wantChanged bool
	}{
		{
			name:        "always changed (default)",
			cmd:         "echo hello",
			changedWhen: "",
			wantChanged: true,
		},
		{
			name:        "changed when rc is 0",
			cmd:         "echo hello",
			changedWhen: "result.rc == 0",
			wantChanged: true,
		},
		{
			name:        "not changed when stdout doesn't contain text",
			cmd:         "echo hello",
			changedWhen: "has(result.stdout, 'goodbye')",
			wantChanged: false,
		},
		{
			name:        "changed when stdout contains text",
			cmd:         "echo hello",
			changedWhen: "has(result.stdout, 'hello')",
			wantChanged: true,
		},
		{
			name:        "never changed",
			cmd:         "echo hello",
			changedWhen: "false",
			wantChanged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()

			step := &config.Step{
				Shell: &config.ShellAction{
					Cmd: tt.cmd,
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
		cmd        string
		failedWhen string
		wantFailed bool
		wantErr    bool
	}{
		{
			name:       "success by default",
			cmd:        "echo hello",
			failedWhen: "",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "success even with rc=0",
			cmd:        "echo hello",
			failedWhen: "result.rc != 0",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "fail when stdout contains text",
			cmd:        "echo ERROR",
			failedWhen: "has(result.stdout, 'ERROR')",
			wantFailed: true,
			wantErr:    true,
		},
		{
			name:       "success when stdout doesn't contain text",
			cmd:        "echo SUCCESS",
			failedWhen: "has(result.stdout, 'ERROR')",
			wantFailed: false,
			wantErr:    false,
		},
		{
			name:       "always fail",
			cmd:        "echo hello",
			failedWhen: "true",
			wantFailed: true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newMockExecutionContext()

			step := &config.Step{
				Shell: &config.ShellAction{
					Cmd: tt.cmd,
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

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: "ls /nonexistent/path/12345",
		},
		FailedWhen: "result.rc > 10", // Only fail if return code > 10
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

func TestHandler_Execute_WithCapture(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	captureTrue := true
	captureFalse := false

	tests := []struct {
		name        string
		capture     *bool
		wantCapture bool
	}{
		{
			name:        "capture true",
			capture:     &captureTrue,
			wantCapture: true,
		},
		{
			name:        "capture false",
			capture:     &captureFalse,
			wantCapture: false,
		},
		{
			name:        "capture default (true)",
			capture:     nil,
			wantCapture: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Shell: &config.ShellAction{
					Cmd:     "echo test output",
					Capture: tt.capture,
				},
			}

			result, err := h.Execute(ctx, step)
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			execResult := result.(*executor.Result)
			hasOutput := strings.Contains(execResult.Stdout, "test output")

			if tt.wantCapture && !hasOutput {
				t.Error("Result.Stdout should contain output when capture is true")
			}
			if !tt.wantCapture && hasOutput {
				t.Error("Result.Stdout should be empty when capture is false")
			}
		})
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
				Shell: &config.ShellAction{
					Cmd: "echo hello",
				},
			},
			wantErr: false,
		},
		{
			name: "command with template",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo {{ message }}",
				},
			},
			wantErr: false,
		},
		{
			name: "command with become",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "systemctl start nginx",
				},
				Become: true,
			},
			wantErr: false,
		},
		{
			name: "command with working directory",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "ls",
				},
				Cwd: "/tmp",
			},
			wantErr: false,
		},
		{
			name: "command with environment",
			step: &config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo $TEST",
				},
				Env: map[string]string{
					"TEST": "value",
				},
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
		Shell: &config.ShellAction{
			Cmd: "echo {{ missing_var }}",
		},
	}

	// Should not error on template failures in dry-run
	err := h.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() should not error on template failures, got: %v", err)
	}

	// Should log a message indicating template would fail
	mockLog := ctx.Logger.(*testutil.MockLogger)
	if len(mockLog.Logs) == 0 {
		t.Error("DryRun() should log something")
	}
}

func TestHandler_Execute_InvalidCommandTemplate(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: "echo {{ invalid.syntax",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd:   "cat",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
		},
		FailedWhen: "'string'", // Evaluates to string, not bool
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when failed_when doesn't return bool")
	}
}

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
		Shell: &config.ShellAction{
			Cmd: "id",
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
		Shell: &config.ShellAction{
			Cmd: "whoami",
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
		Shell: &config.ShellAction{
			Cmd: "echo hello",
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

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo error 1>&2"
	} else {
		cmd = "echo error >&2"
	}

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
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

func TestHandler_Execute_MultilineCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping multiline test on Windows")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	cmd := `
echo line1
echo line2
echo line3
`

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: cmd,
		},
	}

	result, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	out := execResult.Stdout
	if !strings.Contains(out, "line1") || !strings.Contains(out, "line2") || !strings.Contains(out, "line3") {
		t.Errorf("Result.Stdout = %q, want to contain all lines", out)
	}
}

func TestHandler_Execute_ResultRc(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	tests := []struct {
		name   string
		cmd    string
		wantRC int
	}{
		{
			name:   "success command",
			cmd:    "echo hello",
			wantRC: 0,
		},
		{
			name:   "failing command",
			cmd:    "ls /nonexistent12345",
			wantRC: 1, // or 2 on some systems
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &config.Step{
				Shell: &config.ShellAction{
					Cmd: tt.cmd,
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

func TestHandler_Execute_EventEmission(t *testing.T) {
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: "echo test",
		},
	}

	_, err := h.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	mockPub := ctx.EventPublisher.(*testutil.MockPublisher)
	if len(mockPub.Events) == 0 {
		t.Error("Execute() should emit events")
	}

	// Check for stdout event
	foundStdout := false
	for _, event := range mockPub.Events {
		if event.Type == events.EventStepStdout {
			foundStdout = true
			break
		}
	}
	if !foundStdout {
		t.Error("Execute() should emit EventStepStdout")
	}
}

func TestHandler_GetInterpreter(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		interpreter string
		wantDefault string
	}{
		{
			name:        "explicit bash",
			interpreter: "bash",
			wantDefault: "bash",
		},
		{
			name:        "explicit sh",
			interpreter: "sh",
			wantDefault: "sh",
		},
		{
			name:        "explicit pwsh",
			interpreter: "pwsh",
			wantDefault: "pwsh",
		},
		{
			name:        "default on unix",
			interpreter: "",
			wantDefault: "bash", // or pwsh on windows
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shellAction := &config.ShellAction{
				Interpreter: tt.interpreter,
			}

			got := h.getInterpreter(shellAction)

			if tt.interpreter != "" {
				// Explicit interpreter should match
				if got != tt.interpreter {
					t.Errorf("getInterpreter() = %v, want %v", got, tt.interpreter)
				}
			} else {
				// Default should be bash on unix, pwsh on windows
				if runtime.GOOS == "windows" {
					if got != "pwsh" {
						t.Errorf("getInterpreter() = %v, want 'pwsh' on Windows", got)
					}
				} else {
					if got != "bash" {
						t.Errorf("getInterpreter() = %v, want 'bash' on Unix", got)
					}
				}
			}
		})
	}
}

func TestHandler_Execute_WithStdinAndBecome(t *testing.T) {
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
		Shell: &config.ShellAction{
			Cmd:   "cat",
			Stdin: "test input",
		},
		Become: true,
	}

	// Should error because no sudo password
	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when become is true but no sudo password")
	}
}
