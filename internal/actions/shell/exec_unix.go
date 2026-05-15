//go:build !windows

package shell

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"syscall"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// Test seams so we can inject failures without poking PATH/etc-passwd
// in unit tests. Production paths use the real lookups.
var (
	lookPathFunc = exec.LookPath
	userLookup   = user.Lookup
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

		// MT-81: pre-flight checks for as_user so failures don't surface
		// as the generic "command failed with exit code 1" from sudo.
		// Distinguish:
		//   - sudo not on PATH
		//   - target user doesn't exist on this host
		// Both reasons turn into actionable, specific errors before we
		// even spawn sudo.
		if _, err := lookPathFunc("sudo"); err != nil {
			if step.AsUser != "" {
				return nil, fmt.Errorf("as_user: %s requires sudo; sudo not on PATH (apt-get install sudo, or run mooncake as the target user directly)", step.AsUser)
			}
			return nil, fmt.Errorf("become: sudo not on PATH (apt-get install sudo, or run mooncake elevated)")
		}
		if step.AsUser != "" {
			if _, err := userLookup(step.AsUser); err != nil {
				if errors.As(err, new(user.UnknownUserError)) {
					return nil, fmt.Errorf("as_user: %s: user does not exist on this host (check spelling, or `os.user: { name: %s, state: present }` first)", step.AsUser, step.AsUser)
				}
				return nil, fmt.Errorf("as_user: %s: cannot resolve user: %w", step.AsUser, err)
			}
		}

		args := []string{"-S"}
		if step.AsUser != "" {
			args = append(args, "-u", step.AsUser)
		}
		args = append(args, "--", interpreter, "-c", renderedCommand)

		//#nosec G204 — provisioning tool designed to execute shell commands
		command := exec.CommandContext(cmdCtx, "sudo", args...)
		installProcessGroupKill(command)

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
	installProcessGroupKill(command)

	if step.Shell.Stdin != "" {
		renderedStdin, err := ctx.GetTemplate().Render(step.Shell.Stdin, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to render stdin: %w", err)
		}
		command.Stdin = bytes.NewBufferString(renderedStdin)
	}

	return command, nil
}

// installProcessGroupKill puts the spawned shell into its own process
// group and arranges for context cancellation (timeout) to SIGKILL the
// entire group, not just the shell. Without this, compound commands
// like `sleep 30; echo done` would orphan their children: bash itself
// gets killed, but the already-forked `sleep` keeps running until its
// own duration elapses with init as the new parent. Issue #16.
//
// The default exec.CommandContext cancel only signals cmd.Process.Pid,
// reaching the shell process; setting Cancel ourselves replaces that
// behavior with a group-wide kill via the negative-pid pattern.
func installProcessGroupKill(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.Setpgid = true
	c.Cancel = func() error {
		if c.Process == nil {
			return os.ErrProcessDone
		}
		// Kill the entire process group; the negative pid form addresses
		// the group whose pgid == c.Process.Pid (because we set
		// Setpgid=true above).
		return syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}
