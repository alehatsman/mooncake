//go:build !windows

package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// defaultInterpreter is the shell interpreter used on this platform when the
// step does not specify one.
const defaultInterpreter = "bash"

// buildCommand constructs the exec.Cmd that will run the rendered script on
// POSIX systems. Sudo is applied via security.IsBecomeSupported gating.
func (h *Handler) buildCommand(
	cmdCtx context.Context,
	ctx actions.Context,
	step *config.Step,
	renderedCommand string,
) (*exec.Cmd, error) {
	interpreter := step.Shell.Interpreter
	if interpreter == "" {
		interpreter = defaultInterpreter
	}

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return nil, fmt.Errorf("become not supported on %s", runtime.GOOS)
		}

		args := []string{"-S"}
		if step.AsUser != "" {
			args = append(args, "-u", step.AsUser)
		}
		args = append(args, "--", interpreter, "-c", renderedCommand)

		//#nosec G204 — provisioning tool designed to execute shell commands
		command := exec.CommandContext(cmdCtx, "sudo", args...)

		if step.Shell.Stdin != "" {
			renderedStdin, err := ctx.GetTemplate().Render(step.Shell.Stdin, ctx.GetVariables())
			if err != nil {
				return nil, fmt.Errorf("failed to render stdin: %w", err)
			}
			command.Stdin = bytes.NewBuffer([]byte(ec.Svc.SudoPass + "\n" + renderedStdin))
		} else {
			command.Stdin = bytes.NewBuffer([]byte(ec.Svc.SudoPass + "\n"))
		}
		return command, nil
	}

	//#nosec G204 — provisioning tool designed to execute shell commands
	command := exec.CommandContext(cmdCtx, interpreter, "-c", renderedCommand)

	if step.Shell.Stdin != "" {
		renderedStdin, err := ctx.GetTemplate().Render(step.Shell.Stdin, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render stdin: %w", err)
		}
		command.Stdin = bytes.NewBufferString(renderedStdin)
	}

	return command, nil
}
