// Package command implements the command action handler.
//
// The command action executes commands directly without shell interpolation.
// This is safer than shell when you have a known command with arguments,
// as it prevents shell injection attacks.
package command

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// Handler implements the Handler interface for command actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the command action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "cmd",
		Description:        "Execute commands directly without shell interpolation",
		Category:           actions.CategoryCommand,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on command
		ImplementsCheck:    false,
	}
}

// Validate checks if the command configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Cmd == nil {
		return fmt.Errorf("command configuration is nil")
	}

	if len(step.Cmd.Argv) == 0 {
		return fmt.Errorf("command argv is empty")
	}

	return nil
}

// executeWithRetry executes the command with retry logic if
// configured. MT-48: the retry decision is based on the underlying
// exit code, NOT the post-failed_when verdict — otherwise
// `retry: 3` + `failed_when: false` would mask the first failure
// and skip retries entirely. We run executeCommandRaw on each
// attempt (no failed_when applied); only the final result has
// failed_when / changed_when applied via evaluateResultOverrides.
func (h *Handler) executeWithRetry(ctx actions.Context, step *config.Step, renderedArgv []string) (actions.Result, error) {
	maxAttempts := step.RetryAttempts() + 1

	var lastResult actions.Result
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			ctx.GetLogger().Debugf("  Retry attempt %d/%d", attempt, maxAttempts)
		}

		result, err := h.executeCommandRaw(ctx, step, renderedArgv)
		if err == nil {
			// Raw success — apply overrides and return.
			if r, ok := result.(*executor.Result); ok {
				if oerr := h.applyOverrides(ctx, step, r); oerr != nil {
					return r, oerr
				}
				if r.Failed {
					return r, fmt.Errorf("command marked as failed by failed_when condition")
				}
			}
			return result, nil
		}

		lastResult = result
		lastErr = err

		// Sleep before retry if configured
		if attempt < maxAttempts && step.RetryDelayDuration() != "" {
			delay, parseErr := time.ParseDuration(step.RetryDelayDuration())
			if parseErr == nil && delay > 0 {
				ctx.GetLogger().Debugf("  Waiting %v before retry", delay)
				time.Sleep(delay)
			}
		}
	}

	// All attempts failed — apply overrides to the final result.
	// failed_when:false masks the failure; otherwise we propagate.
	if r, ok := lastResult.(*executor.Result); ok && r.Failed {
		if oerr := h.applyOverrides(ctx, step, r); oerr != nil {
			return r, oerr
		}
		if !r.Failed {
			return r, nil
		}
	}
	return lastResult, lastErr
}

// executeCommandRaw is the post-MT-48 retry-friendly variant of
// executeCommand. It runs the command exactly once and returns the
// raw result — failed_when / changed_when are NOT applied here so
// the retry loop above can decide retry based on the raw exit code
// (otherwise failed_when:false would mask the first failure and
// short-circuit retry).
func (h *Handler) executeCommandRaw(ctx actions.Context, step *config.Step, renderedArgv []string) (actions.Result, error) {
	return h.executeCommand(ctx, step, renderedArgv)
}

// applyOverrides applies changed_when / failed_when to a final
// result. Lives outside executeCommand so the retry loop can call
// it once after all attempts finish.
func (h *Handler) applyOverrides(ctx actions.Context, step *config.Step, result *executor.Result) error {
	if step.FailedWhen != "" {
		failed, evalErr := h.evaluateBoolExpression(ctx, step.FailedWhen, map[string]interface{}{
			"rc":     result.Rc,
			"stdout": result.Stdout,
			"stderr": result.Stderr,
		})
		if evalErr != nil {
			return fmt.Errorf("failed to evaluate failed_when: %w", evalErr)
		}
		result.Failed = failed
	}
	if step.ChangedWhen != "" {
		changed, evalErr := h.evaluateBoolExpression(ctx, step.ChangedWhen, map[string]interface{}{
			"rc":     result.Rc,
			"stdout": result.Stdout,
			"stderr": result.Stderr,
		})
		if evalErr != nil {
			return fmt.Errorf("failed to evaluate changed_when: %w", evalErr)
		}
		result.Changed = changed
	}
	return nil
}

// executeCommand executes a command once without retry logic.
func (h *Handler) executeCommand(ctx actions.Context, step *config.Step, renderedArgv []string) (actions.Result, error) {
	// We need access to SudoPass and other fields not in Context interface
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Create command with timeout context
	execCtx := context.Background()
	var cancel context.CancelFunc
	if step.Timeout != "" {
		timeout, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration: %w", err)
		}
		execCtx, cancel = context.WithTimeout(execCtx, timeout)
		defer cancel()
	}

	// Create the command
	cmd, err := h.createDirectCommand(execCtx, step, renderedArgv, ec)
	if err != nil {
		return nil, err
	}

	// Configure environment variables
	if len(step.Env) > 0 {
		envVars := make([]string, 0, len(step.Env))
		for key, value := range step.Env {
			rendered, renderErr := ctx.GetTemplate().Render(value, ctx.GetVariables())
			if renderErr != nil {
				return nil, fmt.Errorf("failed to render env var %s: %w", key, renderErr)
			}
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, rendered))
		}
		cmd.Env = append(cmd.Environ(), envVars...)
	}

	// Set working directory if specified
	if step.Cwd != "" {
		rendered, renderErr := ctx.GetTemplate().Render(step.Cwd, ctx.GetVariables())
		if renderErr != nil {
			return nil, fmt.Errorf("failed to render cwd: %w", renderErr)
		}
		cmd.Dir = rendered
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Execute the command
	err = cmd.Run()

	// Process result
	result := executor.NewResult()
	result.Stdout = strings.TrimSpace(stdout.String())
	result.Stderr = strings.TrimSpace(stderr.String())

	// Handle command execution error. MT-48: record the raw outcome
	// only — failed_when / changed_when are applied later by the
	// retry loop's caller (applyOverrides), so the retry decision
	// sees the underlying exit code unmasked.
	result.Changed = true
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Rc = exitErr.ExitCode()
		} else {
			result.Rc = 1
		}
		result.Failed = true
		return result, fmt.Errorf("command failed with exit code %d", result.Rc)
	}
	result.Rc = 0
	return result, nil
}

// createDirectCommand creates the exec.Cmd for direct command execution (no shell).
func (h *Handler) createDirectCommand(ctx context.Context, step *config.Step, argv []string, ec *executor.ExecutionContext) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty argv")
	}

	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return nil, fmt.Errorf("become not supported on %s", runtime.GOOS)
		}

		// Build sudo arguments. Empty SudoPass is OK for passwordless
		// sudo hosts; sudo will fail with a real error otherwise.
		args := []string{"-S"}
		if step.AsUser != "" {
			args = append(args, "-u", step.AsUser)
		}
		args = append(args, "--")
		args = append(args, argv...)

		// #nosec G204 - This is a provisioning tool designed to execute commands
		command := exec.CommandContext(ctx, "sudo", args...)

		// Handle stdin: sudo password comes first, then user stdin if provided
		if step.Cmd.Stdin != "" {
			renderedStdin, err := ec.Svc.Template.Render(step.Cmd.Stdin, ec.GetVariables())
			if err != nil {
				return nil, fmt.Errorf("failed to render stdin: %w", err)
			}
			command.Stdin = bytes.NewBuffer([]byte(ec.Svc.SudoPass + "\n" + renderedStdin))
		} else {
			command.Stdin = bytes.NewBuffer([]byte(ec.Svc.SudoPass + "\n"))
		}
		return command, nil
	}

	// Direct command execution without shell
	// #nosec G204 - This is a provisioning tool designed to execute commands
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)

	// Handle stdin for non-sudo commands
	if step.Cmd.Stdin != "" {
		renderedStdin, err := ec.Svc.Template.Render(step.Cmd.Stdin, ec.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render stdin: %w", err)
		}
		command.Stdin = bytes.NewBufferString(renderedStdin)
	}

	return command, nil
}

// evaluateBoolExpression evaluates an expression that should return a boolean value.
func (h *Handler) evaluateBoolExpression(ctx actions.Context, expression string, evalContext map[string]interface{}) (bool, error) {
	// Render the expression with variables
	renderedExpr, err := ctx.GetTemplate().Render(expression, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to render expression: %w", err)
	}

	// Evaluate the expression
	result, err := ctx.GetEvaluator().Evaluate(renderedExpr, evalContext)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate expression: %w", err)
	}

	// Cast to bool
	boolResult, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("expression evaluated to %T, expected bool", result)
	}

	return boolResult, nil
}

// Run is the Spec 16 entry point. Like shell, command actions can't be
// predicted for idempotency. Plan mode surfaces the rendered argv so
// users see what would run. WouldChange is set because command steps
// are assumed to mutate state. Apply mode renders the argv and runs
// the command via executeWithRetry.
//
// F011: legacy Execute / DryRun pair folded into Run.
// RunRaw is the spec-69 phase 2-3 entry point. Mirror of shell.RunRaw:
// single attempt, no retry, no failed_when/changed_when overrides.
// The executor's dispatchRunner wraps RunRaw in the retry loop and
// applies overrides post-loop.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cmdAction := step.Cmd
	if ctx.Mode() == actions.ModePlan {
		return h.Run(ctx, step)
	}
	renderedArgv := make([]string, len(cmdAction.Argv))
	for i, arg := range cmdAction.Argv {
		rendered, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render argv[%d]: %w", i, err)
		}
		renderedArgv[i] = rendered
	}
	ctx.GetLogger().Debugf("  Executing: %s", strings.Join(renderedArgv, " "))
	return h.executeCommandRaw(ctx, step, renderedArgv)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cmdAction := step.Cmd

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true

		rendered := make([]string, len(cmdAction.Argv))
		for i, arg := range cmdAction.Argv {
			out, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
			if err != nil {
				out = arg
			}
			rendered[i] = out
		}
		joined := strings.Join(rendered, " ")
		if len(joined) > 80 {
			joined = joined[:77] + "..."
		}
		if step.ShouldBecome() {
			r.Reason = fmt.Sprintf("would run (sudo): %s", joined)
		} else {
			r.Reason = fmt.Sprintf("would run: %s", joined)
		}
		return r, nil
	}

	// Apply mode: render argv (strict — render failures bail) and
	// dispatch to the retry-aware runner.
	renderedArgv := make([]string, len(cmdAction.Argv))
	for i, arg := range cmdAction.Argv {
		rendered, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render argv[%d]: %w", i, err)
		}
		renderedArgv[i] = rendered
	}
	ctx.GetLogger().Debugf("  Executing: %s", strings.Join(renderedArgv, " "))
	return h.executeWithRetry(ctx, step, renderedArgv)
}
