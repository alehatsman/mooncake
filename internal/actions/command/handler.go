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
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/streamoutput"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
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
		Name:           "cmd",
		Description:    "Execute commands directly without shell interpolation",
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
		Examples: []string{
			`# Run a command and capture stdout for a later step
- name: Probe the API version
  cmd:
    argv: [curl, -fsSL, "https://api.example.com/version"]
  as: api_version

- name: Echo what we got
  log:
    msg: "api version = {{ api_version.stdout }}"`,
			`# Silence the captured buffer for noisy commands; output still streams
- name: Run a long build
  cmd:
    argv: [make, build]
    capture: false`,
		},
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

// executeOnce runs the command exactly once and applies overrides
// post-hoc. Production apply goes through RunRaw (executor handles
// retry centrally per spec-69); Run is reached only by plan mode
// and direct unit tests in this package. The in-handler retry loop
// (executeWithRetry) was deleted in the spec-69 cleanup phase
// after its MT-48 / MT-62 invariants were ported to
// internal/executor/retry_test.go.
func (h *Handler) executeOnce(ctx actions.Context, step *config.Step, renderedArgv []string) (actions.Result, error) {
	result, err := h.executeCommandRaw(ctx, step, renderedArgv)
	if err == nil {
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
	if r, ok := result.(*executor.Result); ok && r.Failed {
		if oerr := h.applyOverrides(ctx, step, r); oerr != nil {
			return r, oerr
		}
		if !r.Failed {
			return r, nil
		}
	}
	return result, err
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
		failed, evalErr := actions.EvaluateBoolExpression(ctx, "failed_when", step.FailedWhen, map[string]interface{}{
			"rc":     result.Rc,
			"stdout": result.Stdout,
			"stderr": result.Stderr,
		})
		if evalErr != nil {
			return evalErr
		}
		result.Failed = failed
	}
	if step.ChangedWhen != "" {
		changed, evalErr := actions.EvaluateBoolExpression(ctx, "changed_when", step.ChangedWhen, map[string]interface{}{
			"rc":     result.Rc,
			"stdout": result.Stdout,
			"stderr": result.Stderr,
		})
		if evalErr != nil {
			return evalErr
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

	// Stream stdout/stderr through the shared line-buffered streamer so
	// the console subscriber renders each line in time and `result.Stdout`
	// still carries the full capture for downstream `register/as`.
	// Cmd.Capture defaults to true; honor an explicit `capture: false`.
	shouldCapture := true
	if step.Cmd != nil && step.Cmd.Capture != nil {
		shouldCapture = *step.Cmd.Capture
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if startErr := cmd.Start(); startErr != nil {
		return nil, fmt.Errorf("failed to start command: %w", startErr)
	}

	var stdout, stderr bytes.Buffer
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamoutput.Stream(stdoutPipe, &stdout, ctx, shouldCapture, "stdout")
	}()
	go func() {
		defer wg.Done()
		streamoutput.Stream(stderrPipe, &stderr, ctx, shouldCapture, "stderr")
	}()
	wg.Wait()

	err = cmd.Wait()

	// Process result
	result := executor.NewResult()
	result.Operation = executor.OpUpdate
	result.Target = strings.Join(cmd.Args, " ")
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

// Run is the Spec 16 entry point. Like shell, command actions can't be
// predicted for idempotency. Plan mode surfaces the rendered argv so
// users see what would run. WouldChange is set because command steps
// are assumed to mutate state. Apply mode renders the argv and runs
// the command via executeOnce (retry is owned by the executor's
// runWithRetry post-spec-69 cleanup; Run() is reached only by plan
// mode + direct-call tests).
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
	return h.executeOnce(ctx, step, renderedArgv)
}
