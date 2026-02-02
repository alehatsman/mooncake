package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/security"
)

// HandleCommand executes a direct command without shell interpolation with retry logic if configured.
// This is safer than shell when you have a known command with arguments.
func HandleCommand(step config.Step, ec *ExecutionContext) error {
	cmdAction := step.Command
	if cmdAction == nil || len(cmdAction.Argv) == 0 {
		return &SetupError{Component: "command", Issue: "no command argv specified"}
	}

	// Render all argv elements with templates
	renderedArgv := make([]string, len(cmdAction.Argv))
	for i, arg := range cmdAction.Argv {
		rendered, err := ec.Template.Render(arg, ec.Variables)
		if err != nil {
			return &RenderError{Field: fmt.Sprintf("argv[%d]", i), Cause: err}
		}
		renderedArgv[i] = rendered
	}

	// Check for dry-run mode
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogShellExecution(strings.Join(renderedArgv, " "), step.Become)
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	ec.Logger.Debugf("  Executing: %s", strings.Join(renderedArgv, " "))

	// Execute with retries using common retry logic
	return executeWithRetry(step, ec, func() error {
		return executeCommand(step, ec, renderedArgv)
	})
}

// executeCommand executes a command once without retry logic.
func executeCommand(step config.Step, ec *ExecutionContext, argv []string) error {
	return executeCommandCommon(step, ec, func(ctx context.Context) (*exec.Cmd, error) {
		return createDirectCommand(ctx, step, argv, ec)
	})
}

// createDirectCommand creates the exec.Cmd for direct command execution (no shell)
func createDirectCommand(ctx context.Context, step config.Step, argv []string, ec *ExecutionContext) (*exec.Cmd, error) {
	if len(argv) == 0 {
		return nil, &SetupError{Component: "command", Issue: "empty argv"}
	}

	if step.Become {
		if !security.IsBecomeSupported() {
			return nil, &SetupError{
				Component: "become",
				Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
			}
		}
		if ec.SudoPass == "" {
			return nil, &SetupError{
				Component: "sudo",
				Issue:     "no password provided. Use --sudo-pass flag or --raw mode for interactive sudo",
			}
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
				return nil, &RenderError{Field: "stdin", Cause: err}
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
			return nil, &RenderError{Field: "stdin", Cause: err}
		}
		command.Stdin = bytes.NewBufferString(renderedStdin)
	}

	return command, nil
}
