package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/security"
)

// evaluateResultOverrides applies changed_when and failed_when expressions to override result status.
func evaluateResultOverrides(step config.Step, result *Result, ec *ExecutionContext) error {
	// Create context with result variables for evaluation
	evalContext := make(map[string]interface{})
	for k, v := range ec.Variables {
		evalContext[k] = v
	}
	evalContext["result"] = result.ToMap()

	// Evaluate changed_when if specified
	if step.ChangedWhen != "" {
		renderedExpr, err := ec.Template.Render(step.ChangedWhen, evalContext)
		if err != nil {
			return fmt.Errorf("failed to render changed_when: %w", err)
		}

		changedResult, err := ec.Evaluator.Evaluate(renderedExpr, evalContext)
		if err != nil {
			return fmt.Errorf("failed to evaluate changed_when: %w", err)
		}

		if boolResult, ok := changedResult.(bool); ok {
			result.Changed = boolResult
			ec.Logger.Debugf("  changed_when evaluated to: %v", boolResult)
		} else {
			return fmt.Errorf("changed_when expression did not evaluate to boolean: %v", changedResult)
		}
	}

	// Evaluate failed_when if specified
	if step.FailedWhen != "" {
		renderedExpr, err := ec.Template.Render(step.FailedWhen, evalContext)
		if err != nil {
			return fmt.Errorf("failed to render failed_when: %w", err)
		}

		failedResult, err := ec.Evaluator.Evaluate(renderedExpr, evalContext)
		if err != nil {
			return fmt.Errorf("failed to evaluate failed_when: %w", err)
		}

		if boolResult, ok := failedResult.(bool); ok {
			result.Failed = boolResult
			ec.Logger.Debugf("  failed_when evaluated to: %v", boolResult)
		} else {
			return fmt.Errorf("failed_when expression did not evaluate to boolean: %v", failedResult)
		}
	}

	return nil
}

// HandleShell executes a shell command step with retry logic if configured.
func HandleShell(step config.Step, ec *ExecutionContext) error {
	shell := *step.Shell

	shell = strings.Trim(shell, " ")
	shell = strings.Trim(shell, "\n")

	renderedCommand, err := ec.Template.Render(shell, ec.Variables)
	if err != nil {
		return err
	}

	// Check for dry-run mode
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogShellExecution(renderedCommand, step.Become)
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	ec.Logger.Debugf("  Executing: %s", renderedCommand)

	// Execute with retries if configured
	maxAttempts := step.Retries + 1 // Total attempts = retries + initial attempt
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Log retry attempts (not the first one)
		if attempt > 1 {
			ec.Logger.Debugf("  Retry attempt %d/%d", attempt-1, step.Retries)
		}

		err := executeShellCommand(step, ec, renderedCommand)
		if err == nil {
			// Success - no need to retry
			return nil
		}

		lastErr = err

		// Don't sleep after the last attempt
		if attempt < maxAttempts && step.RetryDelay != "" {
			delay, parseErr := time.ParseDuration(step.RetryDelay)
			if parseErr != nil {
				ec.Logger.Debugf("  Invalid retry_delay %q: %v", step.RetryDelay, parseErr)
			} else {
				ec.Logger.Debugf("  Waiting %s before retry...", step.RetryDelay)
				time.Sleep(delay)
			}
		}
	}

	// All attempts failed
	if step.Retries > 0 {
		return fmt.Errorf("command failed after %d attempts: %w", maxAttempts, lastErr)
	}
	return lastErr
}

// executeShellCommand executes a shell command once without retry logic.
func executeShellCommand(step config.Step, ec *ExecutionContext, renderedCommand string) error {
	// Create result object with start time
	result := NewResult()
	result.StartTime = time.Now()

	// Create context with timeout if specified
	ctx := context.Background()
	var cancel context.CancelFunc
	if step.Timeout != "" {
		timeout, err := time.ParseDuration(step.Timeout)
		if err != nil {
			return fmt.Errorf("invalid timeout duration %q: %w", step.Timeout, err)
		}
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var command *exec.Cmd

	if step.Become {
		if !security.IsBecomeSupported() {
			return fmt.Errorf("become is not supported on %s", runtime.GOOS)
		}
		if ec.SudoPass == "" {
			return fmt.Errorf("step requires sudo but no password provided. Use --sudo-pass flag or --raw mode for interactive sudo")
		}
		// #nosec G204 - This is a provisioning tool designed to execute shell commands.
		// Command execution is the core functionality. The command comes from user-provided
		// YAML configuration files, and users are expected to validate and trust their configs.

		// Build sudo arguments
		args := []string{"-S"}
		if step.BecomeUser != "" {
			args = append(args, "-u", step.BecomeUser)
		}
		args = append(args, "--", "bash", "-c", renderedCommand)

		command = exec.CommandContext(ctx, "sudo", args...)
		command.Stdin = bytes.NewBuffer([]byte(ec.SudoPass + "\n"))
	} else {
		// #nosec G204 - This is a provisioning tool designed to execute shell commands.
		// Command execution is the core functionality. The command comes from user-provided
		// YAML configuration files, and users are expected to validate and trust their configs.
		command = exec.CommandContext(ctx, "bash", "-c", renderedCommand)
	}

	// Set environment variables if specified
	if len(step.Env) > 0 {
		// Start with current environment
		envVars := os.Environ()

		// Add/override with step environment variables
		for key, value := range step.Env {
			// Render the value through template engine
			renderedValue, err := ec.Template.Render(value, ec.Variables)
			if err != nil {
				return fmt.Errorf("failed to render env var %s: %w", key, err)
			}
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, renderedValue))
		}

		command.Env = envVars
	}

	// Set working directory if specified
	if step.Cwd != "" {
		// Render the path through template engine
		renderedCwd, err := ec.Template.Render(step.Cwd, ec.Variables)
		if err != nil {
			return fmt.Errorf("failed to render cwd: %w", err)
		}
		command.Dir = renderedCwd
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err = command.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go captureOutput(stdout, &stdoutBuf, ec, true, &wg)
	go captureOutput(stderr, &stderrBuf, ec, false, &wg)

	// Wait for all output to be processed
	wg.Wait()

	// Populate result fields
	result.Stdout = strings.TrimSpace(stdoutBuf.String())
	result.Stderr = strings.TrimSpace(stderrBuf.String())
	result.Changed = true // Commands always count as changed

	// Determine result status based on command execution
	waitErr := command.Wait()
	wasTimeout := false
	if waitErr != nil {
		// Check if timeout occurred
		if ctx.Err() == context.DeadlineExceeded {
			result.Rc = 124 // Standard timeout exit code
			result.Failed = true
			wasTimeout = true
		} else {
			// Extract exit code
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				result.Rc = exitErr.ExitCode()
			} else {
				result.Rc = 1
			}
			result.Failed = true
		}
	} else {
		result.Rc = 0
		result.Failed = false
	}

	// Record end time and duration
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	// Evaluate changed_when and failed_when overrides
	if evalErr := evaluateResultOverrides(step, result, ec); evalErr != nil {
		return evalErr
	}

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		if result.Failed {
			ec.Logger.Debugf("  Registered result as: %s (failed with rc=%d)", step.Register, result.Rc)
		} else {
			ec.Logger.Debugf("  Registered result as: %s", step.Register)
		}
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	// Return error if command failed (after applying overrides)
	if result.Failed {
		// On error, show captured output for debugging (logger will automatically redact)
		if stdoutBuf.Len() > 0 {
			ec.Logger.Errorf("Command output:\n%s", result.Stdout)
		}
		if stderrBuf.Len() > 0 {
			ec.Logger.Errorf("Error output:\n%s", result.Stderr)
		}

		if wasTimeout {
			return fmt.Errorf("command timed out after %s", step.Timeout)
		}
		return fmt.Errorf("command execution failed with exit code %d", result.Rc)
	}

	return nil
}

func captureOutput(pipe io.Reader, buf *bytes.Buffer, ec *ExecutionContext, isStdout bool, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(pipe)
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		buf.WriteString(line + "\n")
		lineNum++

		// Determine stream type
		stream := "stderr"
		if isStdout {
			stream = "stdout"
		}

		// Emit output event
		if isStdout {
			ec.EmitEvent(events.EventStepStdout, events.StepOutputData{
				StepID:     ec.CurrentStepID,
				Stream:     stream,
				Line:       line,
				LineNumber: lineNum,
			})
		} else {
			ec.EmitEvent(events.EventStepStderr, events.StepOutputData{
				StepID:     ec.CurrentStepID,
				Stream:     stream,
				Line:       line,
				LineNumber: lineNum,
			})
		}

		// Only show in debug mode (both stdout and stderr)
		ec.Logger.Debugf(" %v", line)
	}
}
