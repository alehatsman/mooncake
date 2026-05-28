//go:build windows

package shell

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/sys/windows"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// defaultInterpreter is the shell interpreter used on Windows when the step
// does not specify one. Windows PowerShell 5.1 (powershell.exe) ships with
// every supported Windows version; pwsh.exe (PowerShell 7+) is opt-in.
const defaultInterpreter = "powershell"

// buildCommand constructs the exec.Cmd for Windows. Interpreter dispatch:
//   - "cmd" / "cmd.exe":  cmd /c <script>
//   - anything else:       <interp> -NoProfile -NonInteractive -Command <script>
//
// For PowerShell-family interpreters, an `error_action` field on the shell
// action prepends `$ErrorActionPreference = '<value>'` to the script so the
// caller doesn't have to repeat it every step.
//
// `become: true` is rejected at this layer because security.IsBecomeSupported
// returns false on Windows. Use `run_as_admin: true` to assert elevation
// instead — UAC is process-wide on Windows, so per-step elevation isn't a
// thing.
func (h *Handler) buildCommand(
	cmdCtx context.Context,
	ctx actions.Context,
	step *config.Step,
	renderedCommand string,
) (*exec.Cmd, error) {
	if _, ok := ctx.(*executor.ExecutionContext); !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	if step.Shell.RunAsAdmin && !isProcessElevated() {
		return nil, fmt.Errorf("run_as_admin: true but mooncake is not running elevated — relaunch from an Administrator shell")
	}

	interpreter := step.Shell.Interpreter
	if interpreter == "" {
		interpreter = defaultInterpreter
	}

	var args []string
	switch strings.ToLower(strings.TrimSuffix(interpreter, ".exe")) {
	case "cmd":
		args = []string{"/c", renderedCommand}
	default:
		// powershell, pwsh, and anything else that accepts -Command
		script := renderedCommand
		if step.Shell.ErrorAction != "" && isPowerShell(interpreter) {
			script = fmt.Sprintf("$ErrorActionPreference = '%s'\n%s", step.Shell.ErrorAction, renderedCommand)
		} else if isPowerShell(interpreter) {
			script = "$ErrorActionPreference = 'Stop'\n" + renderedCommand
		}
		args = []string{"-NoProfile", "-NonInteractive", "-Command", script}
	}

	//#nosec G204 — provisioning tool designed to execute shell commands
	command := exec.CommandContext(cmdCtx, interpreter, args...)

	if step.Shell.Stdin != "" {
		renderedStdin, err := ctx.Template().Render(step.Shell.Stdin, ctx.Variables())
		if err != nil {
			return nil, fmt.Errorf("failed to render stdin: %w", err)
		}
		command.Stdin = bytes.NewBufferString(renderedStdin)
	}

	return command, nil
}

// isPowerShell reports whether the interpreter name refers to a PowerShell
// host (Windows PowerShell 5.1 or PowerShell 7+). Used to decide whether
// $ErrorActionPreference applies.
func isPowerShell(interpreter string) bool {
	name := strings.ToLower(strings.TrimSuffix(interpreter, ".exe"))
	return name == "powershell" || name == "pwsh"
}

// isProcessElevated reports whether the current process holds the elevated
// token granted by UAC. Returns false on any error querying the token —
// callers treat that as "not elevated."
func isProcessElevated() bool {
	token := windows.GetCurrentProcessToken()
	return token.IsElevated()
}
