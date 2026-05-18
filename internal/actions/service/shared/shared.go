// Package shared holds the type definitions and helper functions used
// by all per-OS backends of os.service (linux/systemd, darwin/launchd,
// windows/SCM). Lives in its own subpackage so per-OS backends can
// import it without forming an import cycle with the parent
// `service` package (which dispatches at runtime via runtime.GOOS).
//
// See `internal/actions/service/handler.go` for the dispatcher and
// the public Handler type.
package shared

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// Service state vocabulary (the four values accepted by
// step.OsService.State). Exported so per-OS backends can branch on
// them without redeclaring the string literals.
const (
	StateStarted   = "started"
	StateStopped   = "stopped"
	StateReloaded  = "reloaded"
	StateRestarted = "restarted"
)

// ValidStates is the canonical list, mostly for surfacing the
// allowed set in error messages.
var ValidStates = []string{StateStarted, StateStopped, StateReloaded, StateRestarted}

// OsServiceReverseInfo is the per-step apply-time snapshot
// os.service stashes on Result.ReverseData via runApply at the
// dispatcher layer. Captures the unit's pre-apply (active, enabled)
// tuple plus the unit's intent flags from the step so Reverse can
// build a faithful inverse without re-rendering templates.
//
// Captured for linux (systemd), darwin (launchd), and windows
// (Get-Service). Platform field records which OS captured the
// snapshot so Reverse can apply platform-specific honoring rules
// (notably: darwin + windows skip State reverse because their
// transient/persistent distinctions don't map cleanly to systemd's
// started/stopped pair).
type OsServiceReverseInfo struct {
	// Platform is runtime.GOOS at capture time ("linux", "darwin",
	// "windows"). Empty string on older payloads is treated as
	// "linux" by Reverse for backward compatibility — pre-darwin
	// snapshots were always systemd.
	Platform string

	// Name is the unit name (e.g. "myapp.service"). Identity for
	// the reverse step.
	Name string

	// PriorActive reports whether the platform's "running"-probe
	// (systemctl is-active, launchctl print, Get-Service Status)
	// reported a live unit pre-apply.
	PriorActive bool

	// PriorEnabled reports whether the platform's enable-probe
	// (systemctl is-enabled, launchctl print loaded?, Get-Service
	// StartType) reported "enabled / static / indirect" / loaded /
	// Automatic pre-apply.
	PriorEnabled bool

	// HadEnabledIntent reports whether the apply-time step pinned
	// an Enabled value (i.e. step.OsService.Enabled != nil). Used
	// by Reverse to decide whether to set the inverse Enabled
	// field — if the apply didn't manage enabled state, neither
	// should the reverse.
	HadEnabledIntent bool

	// HadStartedIntent mirrors HadEnabledIntent for the started
	// flag.
	HadStartedIntent bool

	// HadStateIntent reports whether the apply pinned a State
	// (started/stopped/restarted/reloaded). When false, the
	// reverse leaves State empty too.
	HadStateIntent bool
}

// BecomeAwareCommand builds *exec.Cmd for `program args...`,
// front-ending it with `sudo -S` when step.ShouldBecome() is true.
// Returns SetupError early when become is requested but the host
// doesn't support it (Windows) or no sudo password was supplied.
// Centralizes the validate-then-construct policy that previously
// lived in 6 hand-rolled copies across handler.go (F004 / F005).
//
// Spec-69 phase-5 audit (NOT migrated to ctx.Privileged): this helper
// is the service action's per-step-conditional-become primitive. The
// step's `become:` field decides per invocation whether to escalate
// (a service-status check on a per-user systemd --user instance does
// NOT need sudo even when an adjacent service-start does), so we
// need BecomeRunner.Command(step.ShouldBecome(), ...) — the
// PrivilegedRunner contract is unconditional root and would force
// every service call through sudo. The same pattern recurs in
// writeFileWithSudo below.
func BecomeAwareCommand(step config.Step, ec *executor.ExecutionContext, program string, args ...string) (*exec.Cmd, error) {
	runner := security.BecomeRunner{SudoPass: ec.Svc.SudoPass, PasswordlessSudo: ec.Svc.PasswordlessSudo}
	cmd, err := runner.Command(step.ShouldBecome(), program, args...)
	if err != nil {
		return nil, WrapBecomeErrorAsSetup(err)
	}
	return cmd, nil
}

// WrapBecomeErrorAsSetup translates the two security.BecomeRunner
// sentinel errors into the executor.SetupError shape this package
// has always returned, so callers (and tests asserting on
// SetupError.Component) don't see an API change.
func WrapBecomeErrorAsSetup(err error) error {
	switch {
	case errors.Is(err, security.ErrBecomeUnsupported):
		return &executor.SetupError{
			Component: "become",
			Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
		}
	case errors.Is(err, security.ErrBecomeNoSudoPass):
		return &executor.SetupError{
			Component: "sudo",
			Issue:     "no password provided. Use --sudo-pass flag",
		}
	}
	return err
}

// RunBecomeAware is BecomeAwareCommand + CombinedOutput + the
// standard CommandError wrap. The `what` label fronts the error
// message ("daemon-reload failed", "systemctl start failed", etc.).
func RunBecomeAware(step config.Step, ec *executor.ExecutionContext, what, program string, args ...string) ([]byte, error) {
	cmd, err := BecomeAwareCommand(step, ec, program, args...)
	if err != nil {
		return nil, err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return output, &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("%s failed: %w (output: %s)", what, err, string(output)),
		}
	}
	return output, nil
}

// RenderTemplateOrContent renders content from either a template
// file or inline content. Reduces code duplication across systemd
// unit-file / drop-in management and launchd plist management.
func RenderTemplateOrContent(srcTemplate, inlineContent, fieldPrefix string, ec *executor.ExecutionContext) (string, error) {
	if srcTemplate != "" {
		// Expand and render template file
		srcPath, expandErr := ec.Svc.PathUtil.ExpandPath(srcTemplate, ec.CurrentDir, ec.GetVariables())
		if expandErr != nil {
			return "", &executor.RenderError{Field: fieldPrefix + ".src_template", Cause: expandErr}
		}

		// Make path absolute relative to config directory
		if !filepath.IsAbs(srcPath) {
			srcPath = filepath.Join(ec.CurrentDir, srcPath)
		}

		// Read template file
		// #nosec G304 - This is a provisioning tool that reads user-specified template files
		templateData, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			return "", &executor.FileOperationError{Operation: "read", Path: srcPath, Cause: readErr}
		}

		// Render template
		content, renderErr := ec.Svc.Template.Render(string(templateData), ec.GetVariables())
		if renderErr != nil {
			return "", &executor.RenderError{Field: fieldPrefix + ".src_template", Cause: renderErr}
		}
		return content, nil
	}

	if inlineContent != "" {
		// Render inline content
		content, renderErr := ec.Svc.Template.Render(inlineContent, ec.GetVariables())
		if renderErr != nil {
			return "", &executor.RenderError{Field: fieldPrefix + ".content", Cause: renderErr}
		}
		return content, nil
	}

	return "", &executor.StepValidationError{
		Field:   fieldPrefix,
		Message: "either src_template or content is required",
	}
}

// WriteFileWithPrivileges writes a file, retrying via sudo when the
// direct write fails with EPERM and the step requested escalation.
func WriteFileWithPrivileges(path string, content []byte, mode string, step config.Step, ec *executor.ExecutionContext) error {
	fileMode := ParseFileMode(mode, 0644)

	if err := os.WriteFile(path, content, fileMode); err != nil {
		if os.IsPermission(err) && step.ShouldBecome() {
			return writeFileWithSudo(path, content, fileMode, ec)
		}
		return &executor.FileOperationError{Operation: "write", Path: path, Cause: err}
	}
	return nil
}

// writeFileWithSudo writes a file using sudo via a sudo-validated
// temp-file + cp + chmod sequence. The first runner.Command call
// validates both become-supported and sudo-pass-present (F005
// final-mile centralized those preflight checks in BecomeRunner).
//
// Spec-69 phase-5 audit (NOT migrated to ctx.Privileged): this site
// needs BecomeRunner.Command directly because each sub-step (cp,
// chmod) constructs an *exec.Cmd whose CombinedOutput + ProcessState
// are inspected separately so we can report distinct exit codes per
// failure mode. PrivilegedRunner.Run returns ([]byte, error) and
// collapses that distinction. See the os_systemd writeAtomic helper
// for the same shape.
func writeFileWithSudo(path string, content []byte, mode os.FileMode, ec *executor.ExecutionContext) error {
	runner := security.BecomeRunner{SudoPass: ec.Svc.SudoPass, PasswordlessSudo: ec.Svc.PasswordlessSudo}

	tmpFile, err := os.CreateTemp("", "mooncake-unit-*")
	if err != nil {
		return &executor.FileOperationError{Operation: "create temp", Path: path, Cause: err}
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close()
		return &executor.FileOperationError{Operation: "write temp", Path: tmpPath, Cause: err}
	}
	if err := tmpFile.Close(); err != nil {
		return &executor.FileOperationError{Operation: "close temp", Path: tmpPath, Cause: err}
	}

	cmd, err := runner.Command(true, "cp", tmpPath, path)
	if err != nil {
		return WrapBecomeErrorAsSetup(err)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("sudo cp failed: %w (output: %s)", err, string(output)),
		}
	}

	cmd, err = runner.Command(true, "chmod", fmt.Sprintf("%o", mode), path)
	if err != nil {
		return WrapBecomeErrorAsSetup(err)
	}
	if output, err := cmd.CombinedOutput(); err != nil {
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("sudo chmod failed: %w (output: %s)", err, string(output)),
		}
	}

	return nil
}

// ParseFileMode parses a file mode string (octal) and returns
// os.FileMode. Falls back to the supplied default when the input is
// empty or unparseable.
func ParseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}
	return os.FileMode(mode)
}

// MarkStepFailed marks a step as failed and registers the result
// when `as:` was set. Mirrors the helper that lived in handler.go
// before the per-OS split — each backend calls it when its
// platform-specific dispatch can't proceed.
func MarkStepFailed(result *executor.Result, step config.Step, ec *executor.ExecutionContext) {
	result.Failed = true
	result.Rc = 1
	if step.As != "" {
		ec.RegisterResult(result, step.As)
	}
}
