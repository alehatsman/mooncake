// Package shell implements the shell action handler.
//
// The shell action executes shell commands with support for:
//   - Cross-platform interpreter dispatch (defaults: bash on Unix, powershell on Windows)
//   - Sudo/become privilege escalation (Unix); run_as_admin assertion (Windows)
//   - Environment variables and working directory
//   - Timeout and retry logic
//   - Stdin, stdout, stderr handling
//   - Result overrides (changed_when, failed_when)
//
// Platform-specific exec.Cmd construction lives in exec_unix.go and
// exec_windows.go via the Handler.buildCommand method.
package shell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the Handler interface for shell actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the shell action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "shell",
		Description:    "Execute shell commands",
		Category:       actions.CategoryCommand,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventStepStdout),
			string(events.EventStepStderr),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on command
		ImplementsCheck:    false,
	}
}

// Validate checks if the shell configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	shellAction := step.Shell
	if shellAction == nil {
		return fmt.Errorf("shell configuration is nil")
	}

	if shellAction.Cmd == "" {
		hint := actions.GetActionHint("shell", "cmd")
		return fmt.Errorf("shell command is empty%s", hint)
	}

	// Validate timeout if specified
	if step.Timeout != "" {
		if _, err := time.ParseDuration(step.Timeout); err != nil {
			return fmt.Errorf("invalid timeout duration %q: %w", step.Timeout, err)
		}
	}

	// Validate retry_delay if specified
	if step.RetryDelayDuration() != "" {
		if _, err := time.ParseDuration(step.RetryDelayDuration()); err != nil {
			return fmt.Errorf("invalid retry_delay duration %q: %w", step.RetryDelayDuration(), err)
		}
	}

	return nil
}

// Execute runs the shell action. Action-level creates:/unless: guards are
// evaluated upstream in the executor's idempotency check, alongside the
// step-level unless_exists/unless_command — by the time we get here the
// step is committed to running.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	shellAction := step.Shell
	shell := strings.Trim(shellAction.Cmd, " \n")

	// Render the command template
	renderedCommand, err := ctx.GetTemplate().Render(shell, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to render command: %w", err)
	}

	ctx.GetLogger().Debugf("  Executing: %s", renderedCommand)

	// Execute with retries
	return h.executeWithRetry(ctx, step, renderedCommand)
}

// DryRun logs what would be executed.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	shellAction := step.Shell
	shell := strings.Trim(shellAction.Cmd, " \n")

	// Attempt to render command (but don't fail if it errors)
	renderedCommand, err := ctx.GetTemplate().Render(shell, ctx.GetVariables())
	if err != nil {
		renderedCommand = shell + " (template render would fail)"
	}

	// Log what would be executed
	ctx.GetLogger().Infof("  [DRY-RUN] Would execute: %s", renderedCommand)
	if step.ShouldBecome() {
		ctx.GetLogger().Infof("  [DRY-RUN] With sudo privileges")
	}

	return nil
}

// executeWithRetry wraps command execution with retry logic. MT-48:
// retry decisions are based on the *raw* exit code, not the post-
// failed_when verdict — otherwise `retry: 3` + `failed_when: false`
// would mask the first failure and skip retries entirely. Each
// attempt runs the command without applying failed_when; only the
// final attempt's result has failed_when / changed_when overrides
// applied.
func (h *Handler) executeWithRetry(ctx actions.Context, step *config.Step, renderedCommand string) (actions.Result, error) {
	maxAttempts := step.RetryAttempts() + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastResult actions.Result
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			ctx.GetLogger().Debugf("  Retry attempt %d/%d", attempt-1, step.RetryAttempts())
		}

		result, err := h.executeShellCommandRaw(ctx, step, renderedCommand)
		if err == nil {
			// Raw success — apply overrides once and return.
			if rr, ok := result.(*executor.Result); ok {
				return h.finishResult(ctx, step, rr)
			}
			return result, nil
		}

		lastResult = result
		lastErr = err

		// Don't sleep after the last attempt
		if attempt < maxAttempts && step.RetryDelayDuration() != "" {
			base, parseErr := time.ParseDuration(step.RetryDelayDuration())
			if parseErr != nil {
				ctx.GetLogger().Debugf("  Invalid retry_delay %q: %v", step.RetryDelayDuration(), parseErr)
			} else {
				// MT-62: backoff: linear|exponential were silently
				// ignored; every retry slept for the bare delay. attempt
				// is 1-indexed here (we're about to start attempt+1), so
				// the n-th sleep multiplies the base delay accordingly.
				delay := ScaleRetryDelay(base, step.RetryBackoffStrategy(), attempt)
				ctx.GetLogger().Debugf("  Waiting %s before retry (backoff=%s)...", delay, step.RetryBackoffStrategy())
				time.Sleep(delay)
			}
		}
	}

	// Every attempt failed. Apply overrides once — failed_when may
	// still mask the final failure (the documented "retry N times
	// and don't fail the run no matter what" pattern). changed_when
	// override applies to the final result the same way.
	//
	// Only apply overrides when lastResult reflects a real exec
	// attempt (Failed=true means processCommandResult set it on a
	// non-nil execErr). For pre-exec failures (invalid timeout,
	// template render error) lastResult.Failed is false; in that
	// path we just propagate lastErr without consulting failed_when
	// — failed_when is for command outcomes, not setup errors.
	if r, ok := lastResult.(*executor.Result); ok && r.Failed {
		if err := h.evaluateResultOverrides(ctx, step, r); err != nil {
			return r, err
		}
		if !r.Failed {
			// failed_when masked it — successful end state.
			return r, nil
		}
		if step.RetryAttempts() > 0 {
			return r, fmt.Errorf("command failed after %d attempts: %w", maxAttempts, lastErr)
		}
		return r, lastErr
	}

	if step.RetryAttempts() > 0 {
		return lastResult, fmt.Errorf("command failed after %d attempts: %w", maxAttempts, lastErr)
	}
	return lastResult, lastErr
}

// finishResult applies changed_when / failed_when overrides to the
// final result returned from the retry loop. Returns the same result
// with potentially-modified Changed/Failed/Rc fields and a non-nil
// error iff result.Failed is still true after overrides.
func (h *Handler) finishResult(ctx actions.Context, step *config.Step, result *executor.Result) (actions.Result, error) {
	if err := h.evaluateResultOverrides(ctx, step, result); err != nil {
		return result, err
	}
	if result.Failed {
		return result, fmt.Errorf("command failed with exit code %d", result.Rc)
	}
	return result, nil
}

// executeShellCommand executes the actual shell command and applies
// changed_when / failed_when overrides. Kept for callers outside the
// retry loop (no current internal callers — see executeShellCommandRaw
// for the retry-friendly variant).
func (h *Handler) executeShellCommand(ctx actions.Context, step *config.Step, renderedCommand string) (actions.Result, error) {
	r, err := h.executeShellCommandRaw(ctx, step, renderedCommand)
	if err != nil {
		// Still apply overrides on failure so failed_when can mask it.
		if rr, ok := r.(*executor.Result); ok {
			if oerr := h.evaluateResultOverrides(ctx, step, rr); oerr != nil {
				return rr, oerr
			}
			if !rr.Failed {
				return rr, nil
			}
		}
		return r, err
	}
	if rr, ok := r.(*executor.Result); ok {
		if oerr := h.evaluateResultOverrides(ctx, step, rr); oerr != nil {
			return rr, oerr
		}
		if rr.Failed {
			return rr, fmt.Errorf("command failed with exit code %d", rr.Rc)
		}
	}
	return r, nil
}

// executeShellCommandRaw runs the command and returns a result whose
// Failed/Rc reflect the underlying process exit code only — NO
// changed_when / failed_when applied. The retry loop uses this so a
// failed_when:false override on intermediate attempts doesn't fool
// the loop into thinking the command succeeded (MT-48).
func (h *Handler) executeShellCommandRaw(ctx actions.Context, step *config.Step, renderedCommand string) (actions.Result, error) {
	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Setup context with timeout
	cmdCtx, cancel, err := h.setupCommandContext(step)
	if err != nil {
		return result, err
	}
	if cancel != nil {
		defer cancel()
	}

	// Create command (platform-specific dispatch lives in exec_{unix,windows}.go)
	command, err := h.buildCommand(cmdCtx, ctx, step, renderedCommand)
	if err != nil {
		return result, err
	}

	// Configure environment
	if err := h.configureCommandEnvironment(command, ctx, step); err != nil {
		return result, err
	}

	// Execute and capture output
	stdout, stderr, execErr := h.executeAndCaptureOutput(command, ctx, step)

	// Process result
	return h.processCommandResult(ctx, step, result, stdout, stderr, execErr)
}

// setupCommandContext creates a context with timeout if specified
func (h *Handler) setupCommandContext(step *config.Step) (context.Context, context.CancelFunc, error) {
	cmdCtx := context.Background()
	var cancel context.CancelFunc

	if step.Timeout != "" {
		timeout, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return nil, nil, fmt.Errorf("invalid timeout duration: %w", err)
		}
		cmdCtx, cancel = context.WithTimeout(cmdCtx, timeout)
	}

	return cmdCtx, cancel, nil
}

// configureCommandEnvironment sets environment variables and working directory
func (h *Handler) configureCommandEnvironment(command *exec.Cmd, ctx actions.Context, step *config.Step) error {
	// Set environment variables
	if len(step.Env) > 0 {
		envVars := os.Environ()
		for key, value := range step.Env {
			renderedValue, err := ctx.GetTemplate().Render(value, ctx.GetVariables())
			if err != nil {
				return fmt.Errorf("failed to render env var %s: %w", key, err)
			}
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, renderedValue))
		}
		command.Env = envVars
	}

	// Set working directory
	if step.Cwd != "" {
		renderedCwd, err := ctx.GetTemplate().Render(step.Cwd, ctx.GetVariables())
		if err != nil {
			return fmt.Errorf("failed to render cwd: %w", err)
		}
		command.Dir = renderedCwd
	}

	return nil
}

// executeAndCaptureOutput runs the command and captures stdout/stderr
func (h *Handler) executeAndCaptureOutput(command *exec.Cmd, ctx actions.Context, step *config.Step) (string, string, error) {
	// Determine if we should capture output
	shouldCapture := true
	if step.Shell.Capture != nil {
		shouldCapture = *step.Shell.Capture
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return "", "", fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if startErr := command.Start(); startErr != nil {
		return "", "", fmt.Errorf("failed to start command: %w", startErr)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup

	// Stream stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.streamOutput(stdout, &stdoutBuf, ctx, shouldCapture, "stdout")
	}()

	// Stream stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		h.streamOutput(stderr, &stderrBuf, ctx, shouldCapture, "stderr")
	}()

	wg.Wait()

	err = command.Wait()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// streamOutput streams command output line by line
func (h *Handler) streamOutput(pipe io.Reader, buf *bytes.Buffer, ctx actions.Context, capture bool, stream string) {
	scanner := bufio.NewScanner(pipe)
	lineNum := 0

	publisher := ctx.GetEventPublisher()
	stepID := ctx.GetCurrentStepID()

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Capture if requested
		if capture {
			buf.WriteString(line)
			buf.WriteString("\n")
		}

		// Emit event
		if publisher != nil {
			var eventType events.EventType
			if stream == "stdout" {
				eventType = events.EventStepStdout
			} else {
				eventType = events.EventStepStderr
			}

			publisher.Publish(events.Event{
				Type:      eventType,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     stepID,
					Stream:     stream,
					Line:       line,
					LineNumber: lineNum,
				},
			})
		}
	}
}

// processCommandResult records the *raw* outcome of the command —
// stdout, stderr, exit code, raw Failed. MT-48: changed_when and
// failed_when are NOT applied here; they live in finishResult so
// the retry loop can decide on raw failure without being fooled by
// an intermediate `failed_when: false` mask.
func (h *Handler) processCommandResult(_ actions.Context, _ *config.Step, result *executor.Result, stdout, stderr string, execErr error) (*executor.Result, error) {
	result.Stdout = stdout
	result.Stderr = stderr

	// Set exit code
	result.Rc = 0
	result.Changed = true
	result.Failed = false

	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			result.Rc = exitErr.ExitCode()
		} else {
			result.Rc = 1
		}
		result.Failed = true
		return result, fmt.Errorf("command failed with exit code %d", result.Rc)
	}

	return result, nil
}

// evaluateResultOverrides applies changed_when and failed_when expressions
func (h *Handler) evaluateResultOverrides(ctx actions.Context, step *config.Step, result *executor.Result) error {
	// Create evaluation context
	evalContext := make(map[string]interface{})
	for k, v := range ctx.GetVariables() {
		evalContext[k] = v
	}
	evalContext["result"] = result.ToMap()

	// Evaluate changed_when
	if step.ChangedWhen != "" {
		boolResult, err := h.evaluateBoolExpression(ctx, step.ChangedWhen, "changed_when", evalContext)
		if err != nil {
			return err
		}
		result.Changed = boolResult
	}

	// Evaluate failed_when
	if step.FailedWhen != "" {
		boolResult, err := h.evaluateBoolExpression(ctx, step.FailedWhen, "failed_when", evalContext)
		if err != nil {
			return err
		}
		result.Failed = boolResult
		if result.Failed && result.Rc == 0 {
			result.Rc = 1
		}
	}

	return nil
}

// evaluateBoolExpression renders and evaluates a boolean expression
func (h *Handler) evaluateBoolExpression(ctx actions.Context, expression, fieldName string, evalContext map[string]interface{}) (bool, error) {
	renderedExpr, err := ctx.GetTemplate().Render(expression, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to render %s: %w", fieldName, err)
	}

	result, err := ctx.GetEvaluator().Evaluate(renderedExpr, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate %s: %w", fieldName, err)
	}

	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("%s expression evaluated to %T, expected bool", fieldName, result)
	}

	return boolResult, nil
}

// Run is the Spec 16 entry point. Shell commands can't be predicted
// for idempotency without running them (the executor's creates/unless
// guards already short-circuit before dispatch, so anything reaching
// Run will execute in normal mode).
//
// Plan mode surfaces the *rendered command text* so users can see
// what would run. WouldChange is set to true because we assume shell
// steps mutate state (matching the legacy Execute which always sets
// Changed=true).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true

		cmd := strings.TrimSpace(step.Shell.Cmd)
		rendered, err := ctx.GetTemplate().Render(cmd, ctx.GetVariables())
		if err != nil {
			rendered = cmd + " (template render would fail)"
		}
		preview := strings.ReplaceAll(rendered, "\n", " ")
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		if step.ShouldBecome() {
			r.Reason = fmt.Sprintf("would run (sudo): %s", preview)
		} else {
			r.Reason = fmt.Sprintf("would run: %s", preview)
		}
		return r, nil
	}
	return h.Execute(ctx, step)
}
