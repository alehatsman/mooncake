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

// executeOnce runs the rendered command exactly once and applies
// failed_when / changed_when overrides post-hoc. This is the path
// that backs both:
//
//   - Production apply: the executor dispatches through RunRaw
//     (single attempt, no overrides) and owns the retry loop +
//     override application in its own pipeline (executor/retry.go
//
//   - executor/finalize.go). Run() is never reached.
//
//   - Direct h.Run() tests: many of the shell tests construct a
//     mock context and call h.Run(...) without going through the
//     executor. They expect single-attempt + overrides applied;
//     retry behavior for those test paths is now covered at the
//     executor unit-test level (TestRunWithRetry_*, TestApplyResult
//     Overrides_*).
//
// Pre-spec-69, this codepath was an in-handler retry loop. Deleting
// the loop required first porting the MT-48 / MT-62 invariants to
// the executor's runWithRetry tests; both invariants are now
// independently guarded there.
func (h *Handler) executeOnce(ctx actions.Context, step *config.Step, renderedCommand string) (actions.Result, error) {
	result, err := h.executeShellCommandRaw(ctx, step, renderedCommand)
	if err != nil {
		// Pre-exec / template / setup failures don't go through
		// failed_when (failed_when is for command outcomes). Apply
		// overrides only when the result reflects a real exec
		// attempt the same way the pre-spec-69 loop did.
		if r, ok := result.(*executor.Result); ok && r.Failed {
			if oErr := h.evaluateResultOverrides(ctx, step, r); oErr != nil {
				return r, oErr
			}
			if !r.Failed {
				return r, nil // failed_when masked it
			}
			return r, err
		}
		return result, err
	}
	if rr, ok := result.(*executor.Result); ok {
		return h.finishResult(ctx, step, rr)
	}
	return result, nil
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
		return result, formatStepFailure(step, result)
	}
	return result, nil
}

// formatStepFailure returns the user-facing error for a failed step,
// distinguishing real subprocess failures from failures caused by a
// failed_when expression on an otherwise-clean run (issue #21). The
// pre-fix message lied — fabricated "exit code 1" on a clean exit-0
// command — sending operators chasing a non-existent shell failure.
func formatStepFailure(step *config.Step, result *executor.Result) error {
	if result.Rc == 0 && step.FailedWhen != "" {
		return fmt.Errorf("step marked failed by failed_when expression %q (underlying command exited 0)", step.FailedWhen)
	}
	return fmt.Errorf("command failed with exit code %d", result.Rc)
}

// executeShellCommand executes the actual shell command and applies
// changed_when / failed_when overrides. Kept for callers outside the
// retry loop (no current internal callers — see executeShellCommandRaw
// for the retry-friendly variant).
//
//nolint:unused // public-shape helper preserved for SDK/MCP callers that bypass retry.
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
			return rr, formatStepFailure(step, rr)
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

// streamOutput streams command output line by line.
//
// F018: bufio.Scanner defaults to a 64 KB token cap; lines longer than
// that triggered Scan()→false, scanner.Err()==bufio.ErrTooLong, and
// silent truncation of the rest of the stream — the command still
// reported success and a downstream step consuming `as: out` got an
// empty or partial stdout with no signal. Two changes:
//
//   - Raise the per-line cap to 1 MB via scanner.Buffer. 1 MB is
//     generous for human-readable output and small enough that a
//     runaway command can't OOM the daemon. Binary blobs > 1 MB
//     should be redirected to a file by the playbook, not captured.
//   - Check scanner.Err() after the loop and surface ErrTooLong (or
//     any other non-EOF pipe error) via the logger so the user sees
//     the truncation instead of treating short stdout as authoritative.
//     The step's exit code is unchanged — log-and-continue is the
//     least-surprising option, matching the finding's recommendation.
const shellStreamMaxLineBytes = 1024 * 1024

func (h *Handler) streamOutput(pipe io.Reader, buf *bytes.Buffer, ctx actions.Context, capture bool, stream string) {
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 64*1024), shellStreamMaxLineBytes)
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
	if err := scanner.Err(); err != nil {
		if log := ctx.GetLogger(); log != nil {
			log.Errorf("  shell %s stream stopped early (output truncated): %v", stream, err)
		}
		// F038: also surface the truncation through the programmatic
		// channels. F018 wired the human logger only; consumers
		// reading result.Stdout / result.Stderr or subscribing to
		// step.* events would see the step complete with empty/short
		// output and no signal that data was dropped. Two writes:
		//
		//   1. Append a clearly-prefixed marker line to the captured
		//      buffer so result.Stdout / result.Stderr carries the
		//      truncation message. The "mooncake: " prefix makes it
		//      distinguishable from real subprocess output.
		//   2. Publish a synthetic step.stderr event so live SSE
		//      subscribers receive the message without waiting for
		//      step.completed.
		//
		// result.Failed stays false: truncation is not a step failure
		// (the subprocess may still exit 0). Consumers that want to
		// fail on truncation can grep for the marker in stderr.
		msg := fmt.Sprintf("mooncake: %s stream truncated (line exceeded %d-byte limit): %v", stream, shellStreamMaxLineBytes, err)
		if capture {
			buf.WriteString(msg)
			buf.WriteString("\n")
		}
		if publisher != nil {
			publisher.Publish(events.Event{
				Type:      events.EventStepStderr,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     stepID,
					Stream:     "stderr",
					Line:       msg,
					LineNumber: lineNum + 1,
				},
			})
		}
		// CRITICAL: keep draining the pipe even after Scanner gave up,
		// otherwise the child process blocks on its write end when the
		// kernel pipe buffer fills (PIPE_BUF is small) and command.Wait()
		// hangs forever — turning silent truncation into a process leak.
		// Discard the rest; capture is best-effort once we've decided
		// the stream is too long for us.
		_, _ = io.Copy(io.Discard, pipe)
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
		boolResult, err := actions.EvaluateBoolExpression(ctx, "changed_when", step.ChangedWhen, evalContext)
		if err != nil {
			return err
		}
		result.Changed = boolResult
	}

	// Evaluate failed_when. Issue #21: do NOT fabricate Rc=1 to
	// "signal" failure — that lied to the operator ("command failed
	// with exit code 1") when the underlying command exited 0. Leave
	// result.Rc reflecting the actual underlying exit code; the
	// downstream error-message path detects Rc==0 && Failed==true and
	// emits a failed-by-failed_when message instead of an exit-code lie.
	if step.FailedWhen != "" {
		boolResult, err := actions.EvaluateBoolExpression(ctx, "failed_when", step.FailedWhen, evalContext)
		if err != nil {
			return err
		}
		result.Failed = boolResult
	}

	return nil
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
// RunRaw is the spec-69 phase 2-3 entry point. The executor calls
// this once per attempt and owns the retry loop + override
// application (failed_when / changed_when). Run() below stays for
// direct-test callers and goes through the in-handler retry path.
//
// MT-48 invariant: RunRaw MUST NOT apply failed_when overrides. The
// retry decision is made on the raw exit code in the executor's
// runWithRetry; applying failed_when here would mask the first
// failure and short-circuit retries.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	shellAction := step.Shell
	cmd := strings.Trim(shellAction.Cmd, " \n")
	if ctx.Mode() == actions.ModePlan {
		// Plan mode goes through Run, not RunRaw — but the executor
		// already bypasses RunRaw in plan mode (see dispatchRunner).
		// This guard is belt-and-suspenders for direct callers.
		return h.Run(ctx, step)
	}
	rendered, err := ctx.GetTemplate().Render(cmd, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to render command: %w", err)
	}
	ctx.GetLogger().Debugf("  Executing: %s", rendered)
	return h.executeShellCommandRaw(ctx, step, rendered)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	shellAction := step.Shell
	cmd := strings.Trim(shellAction.Cmd, " \n")

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true

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

	// Apply mode: render command (strict — render failures bail) and
	// dispatch to executeOnce. Production apply goes through RunRaw
	// (executor handles retry + overrides centrally); Run() is only
	// reached by direct-call tests in this package.
	//
	// F011: legacy Execute / DryRun pair folded into Run.
	renderedCommand, err := ctx.GetTemplate().Render(cmd, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to render command: %w", err)
	}
	ctx.GetLogger().Debugf("  Executing: %s", renderedCommand)
	return h.executeOnce(ctx, step, renderedCommand)
}
