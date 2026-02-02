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
		Name:           "command",
		Description:    "Execute commands directly without shell interpolation",
		Category:       actions.CategoryCommand,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents:    []string{},
		Version:        "1.0.0",
	}
}

// Validate checks if the command configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Command == nil {
		return fmt.Errorf("command configuration is nil")
	}

	if len(step.Command.Argv) == 0 {
		return fmt.Errorf("command argv is empty")
	}

	return nil
}

// Execute runs the command action with retry logic if configured.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cmdAction := step.Command

	// Render all argv elements with templates
	renderedArgv := make([]string, len(cmdAction.Argv))
	for i, arg := range cmdAction.Argv {
		rendered, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render argv[%d]: %w", i, err)
		}
		renderedArgv[i] = rendered
	}

	ctx.GetLogger().Debugf("  Executing: %s", strings.Join(renderedArgv, " "))

	// Execute with retry logic
	return h.executeWithRetry(ctx, step, renderedArgv)
}

// executeWithRetry executes the command with retry logic if configured.
func (h *Handler) executeWithRetry(ctx actions.Context, step *config.Step, renderedArgv []string) (actions.Result, error) {
	maxAttempts := step.Retries + 1

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			ctx.GetLogger().Debugf("  Retry attempt %d/%d", attempt, maxAttempts)
		}

		result, err := h.executeCommand(ctx, step, renderedArgv)
		if err == nil {
			return result, nil
		}

		// If this was the last attempt or no more retries, return the error
		if attempt >= maxAttempts {
			return result, err
		}

		// Sleep before retry if configured
		if step.RetryDelay != "" {
			delay, parseErr := time.ParseDuration(step.RetryDelay)
			if parseErr == nil && delay > 0 {
				ctx.GetLogger().Debugf("  Waiting %v before retry", delay)
				time.Sleep(delay)
			}
		}
	}

	// Should never reach here
	return nil, fmt.Errorf("retry logic failed unexpectedly")
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

	// Handle command execution error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.Rc = exitErr.ExitCode()
		} else {
			result.Rc = 1
		}

		// Check failed_when condition before marking as failed
		if step.FailedWhen != "" {
			failed, evalErr := h.evaluateBoolExpression(ctx, step.FailedWhen, map[string]interface{}{
				"rc":     result.Rc,
				"stdout": result.Stdout,
				"stderr": result.Stderr,
			})
			if evalErr != nil {
				return result, fmt.Errorf("failed to evaluate failed_when: %w", evalErr)
			}
			result.Failed = failed
		} else {
			result.Failed = true
		}

		if result.Failed {
			return result, fmt.Errorf("command failed with exit code %d", result.Rc)
		}
	} else {
		result.Rc = 0

		// Even on success, check failed_when
		if step.FailedWhen != "" {
			failed, evalErr := h.evaluateBoolExpression(ctx, step.FailedWhen, map[string]interface{}{
				"rc":     result.Rc,
				"stdout": result.Stdout,
				"stderr": result.Stderr,
			})
			if evalErr != nil {
				return result, fmt.Errorf("failed to evaluate failed_when: %w", evalErr)
			}
			result.Failed = failed

			if result.Failed {
				return result, fmt.Errorf("command marked as failed by failed_when condition")
			}
		}
	}

	// Determine if command made changes
	if step.ChangedWhen != "" {
		changed, evalErr := h.evaluateBoolExpression(ctx, step.ChangedWhen, map[string]interface{}{
			"rc":     result.Rc,
			"stdout": result.Stdout,
			"stderr": result.Stderr,
		})
		if evalErr != nil {
			return result, fmt.Errorf("failed to evaluate changed_when: %w", evalErr)
		}
		result.Changed = changed
	} else {
		// Default: command execution is considered a change
		result.Changed = true
	}

	return result, nil
}

// createDirectCommand creates the exec.Cmd for direct command execution (no shell).
func (h *Handler) createDirectCommand(ctx context.Context, step *config.Step, argv []string, ec *executor.ExecutionContext) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, fmt.Errorf("empty argv")
	}

	if step.Become {
		if !security.IsBecomeSupported() {
			return nil, fmt.Errorf("become not supported on %s", runtime.GOOS)
		}
		if ec.SudoPass == "" {
			return nil, fmt.Errorf("no sudo password provided. Use --sudo-pass flag or --raw mode for interactive sudo")
		}

		// Build sudo arguments
		args := []string{"-S"}
		if step.BecomeUser != "" {
			args = append(args, "-u", step.BecomeUser)
		}
		args = append(args, "--")
		args = append(args, argv...)

		// #nosec G204 - This is a provisioning tool designed to execute commands
		command := exec.CommandContext(ctx, "sudo", args...)

		// Handle stdin: sudo password comes first, then user stdin if provided
		if step.Command.Stdin != "" {
			renderedStdin, err := ec.Template.Render(step.Command.Stdin, ec.Variables)
			if err != nil {
				return nil, fmt.Errorf("failed to render stdin: %w", err)
			}
			command.Stdin = bytes.NewBuffer([]byte(ec.SudoPass + "\n" + renderedStdin))
		} else {
			command.Stdin = bytes.NewBuffer([]byte(ec.SudoPass + "\n"))
		}
		return command, nil
	}

	// Direct command execution without shell
	// #nosec G204 - This is a provisioning tool designed to execute commands
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)

	// Handle stdin for non-sudo commands
	if step.Command.Stdin != "" {
		renderedStdin, err := ec.Template.Render(step.Command.Stdin, ec.Variables)
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

// DryRun logs what would be executed without actually running the command.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	cmdAction := step.Command

	// Render argv for dry-run display
	renderedArgv := make([]string, len(cmdAction.Argv))
	for i, arg := range cmdAction.Argv {
		rendered, err := ctx.GetTemplate().Render(arg, ctx.GetVariables())
		if err != nil {
			// Use original if rendering fails in dry-run
			rendered = arg
		}
		renderedArgv[i] = rendered
	}

	prefix := "  "
	if step.Become {
		ctx.GetLogger().Infof("%s[DRY-RUN] Would execute (with sudo): %s", prefix, strings.Join(renderedArgv, " "))
	} else {
		ctx.GetLogger().Infof("%s[DRY-RUN] Would execute: %s", prefix, strings.Join(renderedArgv, " "))
	}

	if step.Cwd != "" {
		ctx.GetLogger().Infof("%s           Working directory: %s", prefix, step.Cwd)
	}

	if len(step.Env) > 0 {
		ctx.GetLogger().Infof("%s           Environment variables: %d vars", prefix, len(step.Env))
	}

	if step.Timeout != "" {
		ctx.GetLogger().Infof("%s           Timeout: %s", prefix, step.Timeout)
	}

	if step.Retries > 0 {
		ctx.GetLogger().Infof("%s           Retries: %d", prefix, step.Retries)
	}

	return nil
}
