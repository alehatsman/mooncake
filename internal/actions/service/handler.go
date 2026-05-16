// Package service implements the service action handler.
// Manages services across different platforms (systemd, launchd, Windows).
package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

// Valid service states
const (
	ServiceStateStarted   = "started"
	ServiceStateStopped   = "stopped"
	ServiceStateReloaded  = "reloaded"
	ServiceStateRestarted = "restarted"
)

// Handler implements the service action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("OsServiceReverseInfo", func() any { return &OsServiceReverseInfo{} })
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "os.service",
		Description:        "Manage services across platforms (systemd, launchd, Windows)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{"linux", "darwin", "windows"}, // Platform-specific implementations
		RequiresSudo:       true,                                   // Typically requires elevated privileges
		ImplementsCheck:    true,                                   // Checks service state before changes
	}
}

// Permissions implements actions.Permitter (spec-22). os.service
// always declares Sudo: every supported backend (systemd via systemctl,
// launchd via launchctl, Windows SCM via sc.exe / Set-Service) needs
// elevated privileges to start/stop/enable units and reload
// configuration. Unconditional because the action TYPE is the
// indicator — even a status-only `state: started` check on a system
// service requires the right to introspect via the managing daemon.
//
// Network is false: service-manager operations are all local.
// FilesystemWrite is empty: unit-file mutations go to
// /etc/systemd/system/, ~/Library/LaunchAgents/, etc. — paths the
// user doesn't address directly via step.OsService.Path (which
// doesn't exist). RequiredBinaries left empty because the handler
// detects the backend; demanding a specific binary on PATH would
// break cross-platform support.
func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{Sudo: true}
}

// Validate validates the service action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.OsService == nil {
		return fmt.Errorf("service action requires service configuration")
	}

	serviceAction := step.OsService

	// Validate service name
	if serviceAction.Name == "" {
		return fmt.Errorf("service name is required")
	}

	// Validate state if provided
	if serviceAction.State != "" {
		validStates := []string{ServiceStateStarted, ServiceStateStopped, ServiceStateReloaded, ServiceStateRestarted}
		isValid := false
		for _, s := range validStates {
			if serviceAction.State == s {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid state %q, must be one of: %v", serviceAction.State, validStates)
		}
	}

	return nil
}

// runApply wraps HandleService with reverse-capture: on Linux, it
// queries systemctl is-active / is-enabled BEFORE delegating, then
// bolts the captured snapshot onto ec.CurrentResult.ReverseData
// after HandleService returns (HandleService writes its Result via
// a deferred side-effect into ec.CurrentResult — we don't change
// that contract). Non-Linux paths skip capture; their Reverse refuses.
func runApply(step *config.Step, ec *executor.ExecutionContext) (actions.Result, error) {
	var priorInfo *OsServiceReverseInfo
	if runtime.GOOS == "linux" && step.OsService != nil {
		priorInfo = captureSystemdPriorState(step.OsService.Name, *step, ec)
	}

	err := HandleService(*step, ec)

	if ec.CurrentResult != nil && priorInfo != nil && ec.CurrentResult.Changed {
		ec.CurrentResult.ReverseData = priorInfo
	}
	return ec.CurrentResult, err
}

// HandleService manages services across different platforms (systemd, launchd, Windows).
func HandleService(step config.Step, ec *executor.ExecutionContext) error {
	serviceAction := step.OsService
	if serviceAction == nil {
		return &executor.SetupError{Component: "service", Issue: "no service configuration specified"}
	}

	// Validate service name
	if serviceAction.Name == "" {
		return &executor.StepValidationError{Field: "name", Message: "service name is required"}
	}

	// Render service name
	renderedName, err := ec.Svc.Template.Render(serviceAction.Name, ec.GetVariables())
	if err != nil {
		return &executor.RenderError{Field: "service.name", Cause: err}
	}

	// Validate state if provided
	if serviceAction.State != "" {
		validStates := []string{ServiceStateStarted, ServiceStateStopped, ServiceStateReloaded, ServiceStateRestarted}
		isValid := false
		for _, s := range validStates {
			if serviceAction.State == s {
				isValid = true
				break
			}
		}
		if !isValid {
			return &executor.StepValidationError{
				Field:   "state",
				Message: fmt.Sprintf("invalid state %q, must be one of: %v", serviceAction.State, validStates),
			}
		}
	}

	// Dispatch to platform-specific handler
	switch runtime.GOOS {
	case "linux":
		return handleSystemdService(renderedName, serviceAction, step, ec)
	case "darwin":
		return handleLaunchdService(renderedName, serviceAction, step, ec)
	case "windows":
		return handleWindowsService(renderedName, serviceAction, step, ec)
	default:
		return &executor.SetupError{
			Component: "service",
			Issue:     fmt.Sprintf("service management not supported on %s", runtime.GOOS),
		}
	}
}

// renderTemplateOrContent renders content from either a template file or inline content.
// This helper function reduces code duplication across unit file, dropin, and plist management.
func renderTemplateOrContent(srcTemplate, inlineContent, fieldPrefix string, ec *executor.ExecutionContext) (string, error) {
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

// handleSystemdService manages systemd services on Linux.
func handleSystemdService(serviceName string, serviceAction *config.ServiceAction, step config.Step, ec *executor.ExecutionContext) error {
	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	changed := false
	operations := []string{}

	// Handle unit file management
	if serviceAction.Unit != nil {
		unitChanged, err := manageSystemdUnitFile(serviceName, serviceAction.Unit, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if unitChanged {
			changed = true
			operations = append(operations, "unit file updated")
		}
	}

	// Handle drop-in file management
	if serviceAction.Dropin != nil {
		dropinChanged, err := manageSystemdDropin(serviceName, serviceAction.Dropin, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if dropinChanged {
			changed = true
			operations = append(operations, "drop-in updated")
		}
	}

	// Run daemon-reload if needed (after unit/dropin changes or if explicitly requested)
	if (changed && serviceAction.DaemonReload) || (serviceAction.DaemonReload && !changed) {
		if err := systemdDaemonReload(step, ec); err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		operations = append(operations, "daemon-reload")
	}

	// Manage service state
	if serviceAction.State != "" {
		stateChanged, err := manageSystemdServiceState(serviceName, serviceAction.State, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if stateChanged {
			changed = true
			operations = append(operations, fmt.Sprintf("service %s", serviceAction.State))
		}
	}

	// Manage service enablement
	if serviceAction.Enabled != nil {
		enableChanged, err := manageSystemdServiceEnabled(serviceName, *serviceAction.Enabled, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if enableChanged {
			changed = true
			if *serviceAction.Enabled {
				operations = append(operations, "service enabled")
			} else {
				operations = append(operations, "service disabled")
			}
		}
	}

	// Set result properties
	result.Changed = changed
	result.Rc = 0
	result.Failed = false

	// Emit event
	ec.EmitEvent(events.EventServiceManaged, events.ServiceManagementData{
		Service:    serviceName,
		State:      serviceAction.State,
		Enabled:    serviceAction.Enabled,
		Changed:    changed,
		Operations: operations,
		DryRun:     ec.Mode() == actions.ModePlan,
	})

	// Register result if specified
	if step.As != "" {
		ec.RegisterResult(result, step.As)
	}

	if changed {
		ec.Svc.Logger.Infof("  Service %s: %s", serviceName, strings.Join(operations, ", "))
	} else {
		ec.Svc.Logger.Debugf("  Service %s: no changes needed", serviceName)
	}

	return nil
}

// manageSystemdUnitFile creates or updates a systemd unit file.
func manageSystemdUnitFile(serviceName string, unit *config.ServiceUnit, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	// Determine unit file path
	unitPath := unit.Dest
	if unitPath == "" {
		// Default to system unit directory
		unitPath = fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	}

	// Render content from template or inline
	content, err := renderTemplateOrContent(unit.SrcTemplate, unit.Content, "service.unit", ec)
	if err != nil {
		return false, err
	}

	// Check if file exists and has same content (idempotency)
	// #nosec G304 - This is a provisioning tool that manages service unit files
	existingContent, readErr := os.ReadFile(unitPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Unit file %s already up to date", unitPath)
		return false, nil
	}

	// Write unit file (may require sudo)
	if err := writeFileWithPrivileges(unitPath, []byte(content), unit.Mode, step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Unit file written: %s", unitPath)
	return true, nil
}

// manageSystemdDropin creates or updates a systemd drop-in file.
func manageSystemdDropin(serviceName string, dropin *config.ServiceDropin, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	if dropin.Name == "" {
		return false, &executor.StepValidationError{Field: "service.dropin.name", Message: "drop-in name is required"}
	}

	// Drop-in directory path
	dropinDir := fmt.Sprintf("/etc/systemd/system/%s.service.d", serviceName)
	dropinPath := filepath.Join(dropinDir, dropin.Name)

	// Render content from template or inline
	content, err := renderTemplateOrContent(dropin.SrcTemplate, dropin.Content, "service.dropin", ec)
	if err != nil {
		return false, err
	}

	// Check if file exists and has same content (idempotency)
	// #nosec G304 - This is a provisioning tool that manages service drop-in files
	existingContent, readErr := os.ReadFile(dropinPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Drop-in file %s already up to date", dropinPath)
		return false, nil
	}

	// Ensure drop-in directory exists
	// #nosec G301 - Drop-in directories need to be readable by systemd (0755 is appropriate)
	if err := os.MkdirAll(dropinDir, 0755); err != nil {
		if os.IsPermission(err) && !step.ShouldBecome() {
			return false, &executor.FileOperationError{
				Operation: "mkdir",
				Path:      dropinDir,
				Cause:     fmt.Errorf("permission denied (try become: true)"),
			}
		}
		return false, &executor.FileOperationError{Operation: "mkdir", Path: dropinDir, Cause: err}
	}

	// Write drop-in file (may require sudo)
	if err := writeFileWithPrivileges(dropinPath, []byte(content), "0644", step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Drop-in file written: %s", dropinPath)
	return true, nil
}

// systemdDaemonReload executes systemctl daemon-reload.
// becomeAwareCommand builds *exec.Cmd for `program args...`, front-ending
// it with `sudo -S` when step.ShouldBecome() is true. Returns SetupError
// early when become is requested but the host doesn't support it (Windows)
// or no sudo password was supplied. F004 centralizes the 6 hand-rolled
// copies of this pattern in handler.go — collapses each call site to a
// couple of lines and closes the asymmetry where two sites
// (getSystemdServiceState, isSystemdServiceEnabled) silently skipped the
// IsBecomeSupported guard.
//
// strings (systemctl, launchctl); args are validated service names.
// Documented #nosec at each former call site is now satisfied here.
//
//nolint:gosec // G204: program names come from this package's literal
func becomeAwareCommand(step config.Step, ec *executor.ExecutionContext, program string, args ...string) (*exec.Cmd, error) {
	if !step.ShouldBecome() {
		return exec.Command(program, args...), nil
	}
	if !security.IsBecomeSupported() {
		return nil, &executor.SetupError{
			Component: "become",
			Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
		}
	}
	if ec.Svc.SudoPass == "" {
		return nil, &executor.SetupError{
			Component: "sudo",
			Issue:     "no password provided. Use --sudo-pass flag",
		}
	}
	sudoArgs := append([]string{"-S", program}, args...)
	cmd := exec.Command("sudo", sudoArgs...)
	cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
	return cmd, nil
}

// runBecomeAware is becomeAwareCommand + CombinedOutput + the standard
// CommandError wrap. The `what` label fronts the error message
// ("daemon-reload failed", "systemctl start failed", etc.).
func runBecomeAware(step config.Step, ec *executor.ExecutionContext, what, program string, args ...string) ([]byte, error) {
	cmd, err := becomeAwareCommand(step, ec, program, args...)
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

func systemdDaemonReload(step config.Step, ec *executor.ExecutionContext) error {
	ec.Svc.Logger.Debugf("  Running systemctl daemon-reload")
	_, err := runBecomeAware(step, ec, "daemon-reload", "systemctl", "daemon-reload")
	return err
}

// manageSystemdServiceState manages the service state (started/stopped/restarted/reloaded).
func manageSystemdServiceState(serviceName, desiredState string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	// Get current state
	currentState, err := getSystemdServiceState(serviceName, step, ec)
	if err != nil {
		return false, err
	}

	var action string
	switch desiredState {
	case ServiceStateStarted:
		if currentState == "active" {
			ec.Svc.Logger.Debugf("  Service %s already active", serviceName)
			return false, nil
		}
		action = "start"
	case ServiceStateStopped:
		if currentState == "inactive" || currentState == "failed" {
			ec.Svc.Logger.Debugf("  Service %s already stopped", serviceName)
			return false, nil
		}
		action = "stop"
	case ServiceStateRestarted:
		action = "restart"
	case ServiceStateReloaded:
		action = "reload"
	default:
		return false, &executor.StepValidationError{
			Field:   "state",
			Message: fmt.Sprintf("unsupported state: %s", desiredState),
		}
	}

	// Execute systemctl command
	ec.Svc.Logger.Debugf("  Running systemctl %s %s", action, serviceName)
	if _, err := runBecomeAware(step, ec, "systemctl "+action, "systemctl", action, serviceName); err != nil {
		return false, err
	}
	return true, nil
}

// getSystemdServiceState returns the current state of a systemd service.
func getSystemdServiceState(serviceName string, step config.Step, ec *executor.ExecutionContext) (string, error) {
	cmd, err := becomeAwareCommand(step, ec, "systemctl", "is-active", serviceName)
	if err != nil {
		return "", err
	}
	output, _ := cmd.Output() // Ignore error, is-active returns non-zero for inactive services
	state := strings.TrimSpace(string(output))
	ec.Svc.Logger.Debugf("  Service %s current state: %s", serviceName, state)
	return state, nil
}

// manageSystemdServiceEnabled manages the service enabled status.
func manageSystemdServiceEnabled(serviceName string, shouldBeEnabled bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	// Check current enabled status
	isEnabled, err := isSystemdServiceEnabled(serviceName, step, ec)
	if err != nil {
		return false, err
	}

	if isEnabled == shouldBeEnabled {
		ec.Svc.Logger.Debugf("  Service %s enabled status already correct: %v", serviceName, isEnabled)
		return false, nil
	}

	// Execute enable/disable command
	var action string
	if shouldBeEnabled {
		action = "enable"
	} else {
		action = "disable"
	}

	ec.Svc.Logger.Debugf("  Running systemctl %s %s", action, serviceName)
	if _, err := runBecomeAware(step, ec, "systemctl "+action, "systemctl", action, serviceName); err != nil {
		return false, err
	}
	return true, nil
}

// isSystemdServiceEnabled checks if a systemd service is enabled.
func isSystemdServiceEnabled(serviceName string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	cmd, err := becomeAwareCommand(step, ec, "systemctl", "is-enabled", serviceName)
	if err != nil {
		return false, err
	}
	output, _ := cmd.Output() // Ignore error, is-enabled returns non-zero for disabled services
	status := strings.TrimSpace(string(output))
	isEnabled := status == "enabled" || status == "static" || status == "indirect"
	ec.Svc.Logger.Debugf("  Service %s enabled status: %s (treated as enabled: %v)", serviceName, status, isEnabled)
	return isEnabled, nil
}

// writeFileWithPrivileges writes a file with optional sudo privileges.
func writeFileWithPrivileges(path string, content []byte, mode string, step config.Step, ec *executor.ExecutionContext) error {
	// Parse mode if provided
	fileMode := parseFileMode(mode, 0644)

	// Try direct write first
	if err := os.WriteFile(path, content, fileMode); err != nil {
		if os.IsPermission(err) && step.ShouldBecome() {
			// Use sudo to write file
			return writeFileWithSudo(path, content, fileMode, ec)
		}
		return &executor.FileOperationError{Operation: "write", Path: path, Cause: err}
	}

	return nil
}

// writeFileWithSudo writes a file using sudo (for privileged paths).
func writeFileWithSudo(path string, content []byte, mode os.FileMode, ec *executor.ExecutionContext) error {
	if !security.IsBecomeSupported() {
		return &executor.SetupError{
			Component: "become",
			Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
		}
	}

	if ec.Svc.SudoPass == "" {
		return &executor.SetupError{
			Component: "sudo",
			Issue:     "no password provided. Use --sudo-pass flag",
		}
	}

	// Write to temporary file first
	tmpFile, err := os.CreateTemp("", "mooncake-unit-*")
	if err != nil {
		return &executor.FileOperationError{Operation: "create temp", Path: path, Cause: err}
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }() // Best-effort cleanup

	if _, err := tmpFile.Write(content); err != nil {
		_ = tmpFile.Close() // Best-effort cleanup on error path
		return &executor.FileOperationError{Operation: "write temp", Path: tmpPath, Cause: err}
	}
	if err := tmpFile.Close(); err != nil {
		return &executor.FileOperationError{Operation: "close temp", Path: tmpPath, Cause: err}
	}

	// Use sudo to copy temp file to target location
	// #nosec G204 - This is a provisioning tool that needs to copy files with elevated privileges
	cmd := exec.Command("sudo", "-S", "cp", tmpPath, path)
	cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")

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

	// Set file permissions with sudo
	// #nosec G204 - This is a provisioning tool that needs to set file permissions with elevated privileges
	cmd = exec.Command("sudo", "-S", "chmod", fmt.Sprintf("%o", mode), path)
	cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")

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

// handleLaunchdService manages launchd services on macOS.
func handleLaunchdService(serviceName string, serviceAction *config.ServiceAction, step config.Step, ec *executor.ExecutionContext) error {
	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	changed := false
	operations := []string{}

	// Determine if this is a user agent or system daemon based on become flag
	isSystem := step.ShouldBecome()
	domain := getLaunchdDomain(isSystem)

	// Handle plist file management
	if serviceAction.Unit != nil {
		plistChanged, err := manageLaunchdPlist(serviceName, serviceAction.Unit, isSystem, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if plistChanged {
			changed = true
			operations = append(operations, "plist updated")
		}
	}

	// Get plist path for launchctl commands
	plistPath := getLaunchdPlistPath(serviceName, serviceAction.Unit, isSystem, ec)
	serviceID := fmt.Sprintf("%s/%s", domain, serviceName)

	// Check if service is loaded
	isLoaded, err := isLaunchdServiceLoaded(serviceID, step, ec)
	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}

	// Manage service state
	if serviceAction.State != "" {
		stateChanged, err := manageLaunchdServiceState(serviceName, serviceID, plistPath, domain, serviceAction.State, isLoaded, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if stateChanged {
			changed = true
			operations = append(operations, fmt.Sprintf("service %s", serviceAction.State))
		}
	}

	// Manage service enablement (load/unload)
	if serviceAction.Enabled != nil {
		enableChanged, err := manageLaunchdServiceEnabled(serviceID, plistPath, domain, *serviceAction.Enabled, isLoaded, step, ec)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if enableChanged {
			changed = true
			if *serviceAction.Enabled {
				operations = append(operations, "service loaded")
			} else {
				operations = append(operations, "service unloaded")
			}
		}
	}

	// Set result properties
	result.Changed = changed
	result.Rc = 0
	result.Failed = false

	// Emit event
	ec.EmitEvent(events.EventServiceManaged, events.ServiceManagementData{
		Service:    serviceName,
		State:      serviceAction.State,
		Enabled:    serviceAction.Enabled,
		Changed:    changed,
		Operations: operations,
		DryRun:     ec.Mode() == actions.ModePlan,
	})

	// Register result if specified
	if step.As != "" {
		ec.RegisterResult(result, step.As)
	}

	if changed {
		ec.Svc.Logger.Infof("  Service %s: %s", serviceName, strings.Join(operations, ", "))
	} else {
		ec.Svc.Logger.Debugf("  Service %s: no changes needed", serviceName)
	}

	return nil
}

// getLaunchdDomain returns the appropriate launchd domain (system or user)
func getLaunchdDomain(isSystem bool) string {
	if isSystem {
		return "system"
	}
	return fmt.Sprintf("gui/%d", os.Getuid())
}

// getLaunchdPlistPath returns the plist file path for a launchd service
func getLaunchdPlistPath(serviceName string, unit *config.ServiceUnit, isSystem bool, _ *executor.ExecutionContext) string {
	if unit != nil && unit.Dest != "" {
		return unit.Dest
	}

	// Default paths based on system vs user
	if isSystem {
		return fmt.Sprintf("/Library/LaunchDaemons/%s.plist", serviceName)
	}

	homeDir := os.Getenv("HOME")
	return fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", homeDir, serviceName)
}

// manageLaunchdPlist creates or updates a launchd plist file
func manageLaunchdPlist(serviceName string, unit *config.ServiceUnit, isSystem bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	plistPath := getLaunchdPlistPath(serviceName, unit, isSystem, ec)

	// Render content from template or inline content
	content, err := renderTemplateOrContent(unit.SrcTemplate, unit.Content, "service.unit", ec)
	if err != nil {
		return false, err
	}

	// Check if file exists and has same content (idempotency)
	// #nosec G304 - This is a provisioning tool that reads plist files from validated paths
	existingContent, readErr := os.ReadFile(plistPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Plist file %s already up to date", plistPath)
		return false, nil
	}

	// Ensure parent directory exists
	plistDir := filepath.Dir(plistPath)
	// #nosec G301 - Plist directories need to be readable by launchd (0755 is appropriate)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return false, &executor.FileOperationError{Operation: "mkdir", Path: plistDir, Cause: err}
	}

	// Write plist file (may require sudo for system daemons)
	if err := writeFileWithPrivileges(plistPath, []byte(content), unit.Mode, step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Plist file written: %s", plistPath)
	return true, nil
}

// isLaunchdServiceLoaded checks if a launchd service is loaded
func isLaunchdServiceLoaded(serviceID string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	var cmd *exec.Cmd
	if step.ShouldBecome() {
		if ec.Svc.SudoPass == "" {
			return false, &executor.SetupError{
				Component: "sudo",
				Issue:     "no password provided. Use --sudo-pass flag",
			}
		}
		cmd = exec.Command("sudo", "-S", "launchctl", "print", serviceID)
		cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
	} else {
		cmd = exec.Command("launchctl", "print", serviceID)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Service not loaded (launchctl print returns error if not loaded)
		if strings.Contains(string(output), "Could not find service") {
			ec.Svc.Logger.Debugf("  Service %s is not loaded", serviceID)
			return false, nil
		}
		// Other errors
		ec.Svc.Logger.Debugf("  Error checking service status: %v", err)
		return false, nil
	}

	ec.Svc.Logger.Debugf("  Service %s is loaded", serviceID)
	return true, nil
}

// manageLaunchdServiceState manages the service state (started/stopped/restarted)
func manageLaunchdServiceState(serviceName, serviceID, plistPath, domain string, desiredState string, isLoaded bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	switch desiredState {
	case ServiceStateStarted:
		if !isLoaded {
			// Need to bootstrap first
			if err := launchdBootstrap(domain, plistPath, step, ec); err != nil {
				return false, err
			}
			return true, nil
		}
		// Already loaded, try to start it (kickstart)
		if err := launchdKickstart(serviceID, false, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case ServiceStateStopped:
		if !isLoaded {
			ec.Svc.Logger.Debugf("  Service %s already stopped (not loaded)", serviceName)
			return false, nil
		}
		// Kill the service
		if err := launchdKill(serviceID, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case ServiceStateRestarted:
		if !isLoaded {
			// Bootstrap if not loaded
			if err := launchdBootstrap(domain, plistPath, step, ec); err != nil {
				return false, err
			}
			return true, nil
		}
		// Kickstart with -k flag (kill and restart)
		if err := launchdKickstart(serviceID, true, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case ServiceStateReloaded:
		// launchd doesn't have a direct reload, treat as restart
		return manageLaunchdServiceState(serviceName, serviceID, plistPath, domain, ServiceStateRestarted, isLoaded, step, ec)

	default:
		return false, &executor.StepValidationError{
			Field:   "state",
			Message: fmt.Sprintf("unsupported state: %s", desiredState),
		}
	}
}

// manageLaunchdServiceEnabled manages the service enabled status (loaded/unloaded)
func manageLaunchdServiceEnabled(serviceID, plistPath, domain string, shouldBeEnabled bool, isLoaded bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	if shouldBeEnabled && !isLoaded {
		// Need to bootstrap
		if err := launchdBootstrap(domain, plistPath, step, ec); err != nil {
			return false, err
		}
		return true, nil
	}

	if !shouldBeEnabled && isLoaded {
		// Need to bootout
		if err := launchdBootout(domain, plistPath, step, ec); err != nil {
			return false, err
		}
		return true, nil
	}

	// Already in desired state
	ec.Svc.Logger.Debugf("  Service %s already in desired enabled state: %v", serviceID, shouldBeEnabled)
	return false, nil
}

// executeLaunchctlCommand executes a launchctl command with proper error handling.
// Uses becomeAwareCommand for the elevation pattern (F004) — can't use the
// higher-level runBecomeAware because the error path needs the raw output
// to check idempotency markers like "Already loaded" before deciding to
// surface as success vs. CommandError.
func executeLaunchctlCommand(command, domain, plistPath string, step config.Step, ec *executor.ExecutionContext, idempotencyCheck []string, successMsg, errorMsg string) error {
	ec.Svc.Logger.Debugf("  Running launchctl %s %s %s", command, domain, plistPath)
	cmd, err := becomeAwareCommand(step, ec, "launchctl", command, domain, plistPath)
	if err != nil {
		return err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		for _, check := range idempotencyCheck {
			if strings.Contains(outputStr, check) {
				ec.Svc.Logger.Debugf("  %s", successMsg)
				return nil
			}
		}
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("%s: %w (output: %s)", errorMsg, err, outputStr),
		}
	}
	return nil
}

// launchdBootstrap loads a service using launchctl bootstrap
func launchdBootstrap(domain, plistPath string, step config.Step, ec *executor.ExecutionContext) error {
	return executeLaunchctlCommand(
		"bootstrap",
		domain,
		plistPath,
		step,
		ec,
		[]string{"Already loaded", "service already loaded"},
		"Service already loaded",
		"launchctl bootstrap failed",
	)
}

// launchdBootout unloads a service using launchctl bootout
func launchdBootout(domain, plistPath string, step config.Step, ec *executor.ExecutionContext) error {
	return executeLaunchctlCommand(
		"bootout",
		domain,
		plistPath,
		step,
		ec,
		[]string{"Could not find", "not loaded"},
		"Service already unloaded",
		"launchctl bootout failed",
	)
}

// launchdKickstart starts or restarts a service using launchctl kickstart
func launchdKickstart(serviceID string, kill bool, step config.Step, ec *executor.ExecutionContext) error {
	args := []string{"kickstart"}
	if kill {
		args = append(args, "-k")
	}
	args = append(args, serviceID)

	ec.Svc.Logger.Debugf("  Running launchctl %s", strings.Join(args, " "))

	var cmd *exec.Cmd
	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return &executor.SetupError{
				Component: "become",
				Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
			}
		}
		if ec.Svc.SudoPass == "" {
			return &executor.SetupError{
				Component: "sudo",
				Issue:     "no password provided. Use --sudo-pass flag",
			}
		}
		sudoArgs := make([]string, 0, 2+len(args))
		sudoArgs = append(sudoArgs, "-S", "launchctl")
		sudoArgs = append(sudoArgs, args...)
		// #nosec G204 - This is a provisioning tool that manages launchd services with validated commands
		cmd = exec.Command("sudo", sudoArgs...)
		cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
	} else {
		cmd = exec.Command("launchctl", args...)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("launchctl kickstart failed: %w (output: %s)", err, string(output)),
		}
	}

	return nil
}

// launchdKill stops a service using launchctl kill
func launchdKill(serviceID string, step config.Step, ec *executor.ExecutionContext) error {
	ec.Svc.Logger.Debugf("  Running launchctl kill SIGTERM %s", serviceID)

	var cmd *exec.Cmd
	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return &executor.SetupError{
				Component: "become",
				Issue:     fmt.Sprintf("not supported on %s", runtime.GOOS),
			}
		}
		if ec.Svc.SudoPass == "" {
			return &executor.SetupError{
				Component: "sudo",
				Issue:     "no password provided. Use --sudo-pass flag",
			}
		}
		cmd = exec.Command("sudo", "-S", "launchctl", "kill", "SIGTERM", serviceID)
		cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")
	} else {
		cmd = exec.Command("launchctl", "kill", "SIGTERM", serviceID)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// It's okay if service is not running
		if strings.Contains(string(output), "Could not find") || strings.Contains(string(output), "not running") {
			ec.Svc.Logger.Debugf("  Service not running")
			return nil
		}
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return &executor.CommandError{
			ExitCode: exitCode,
			Cause:    fmt.Errorf("launchctl kill failed: %w (output: %s)", err, string(output)),
		}
	}

	return nil
}

// handleWindowsService manages Windows services (placeholder).
func handleWindowsService(_ string, _ *config.ServiceAction, _ config.Step, _ *executor.ExecutionContext) error {
	return &executor.SetupError{
		Component: "windows service",
		Issue:     "Windows service support not yet implemented",
	}
}

// markStepFailed marks a step as failed and registers the result.
func markStepFailed(result *executor.Result, step config.Step, ec *executor.ExecutionContext) {
	result.Failed = true
	result.Rc = 1
	if step.As != "" {
		ec.RegisterResult(result, step.As)
	}
}

// parseFileMode parses a file mode string (octal) and returns os.FileMode.
func parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	// Parse as octal
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

// Run is the unified entry point. Plan mode queries systemctl
// is-active / is-enabled to predict state and enabled-flag changes,
// and compares unit / dropin file contents against the rendered
// templates. Apply mode delegates to HandleService via runApply.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	if ctx.Mode() != actions.ModePlan {
		return runApply(step, ec)
	}

	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		result.Reason = fmt.Sprintf("service state inspection not implemented on %s", runtime.GOOS)
		result.Checkable = false
		return result, nil
	}

	svc := step.OsService
	if svc == nil {
		return result, fmt.Errorf("service action requires service configuration")
	}

	renderedName, err := ec.Svc.Template.Render(svc.Name, ec.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render service name: %w", err)
	}

	var reasons []string

	// State change prediction.
	if svc.State != "" {
		current, _ := getSystemdServiceState(renderedName, *step, ec)
		switch svc.State {
		case ServiceStateStarted:
			if current != "active" {
				reasons = append(reasons, "would start service")
			}
		case ServiceStateStopped:
			if current == "active" {
				reasons = append(reasons, "would stop service")
			}
		case ServiceStateReloaded, ServiceStateRestarted:
			reasons = append(reasons, "would "+svc.State+" service")
		}
	}

	// Enabled-flag change prediction.
	if svc.Enabled != nil {
		isEnabled, err := isSystemdServiceEnabled(renderedName, *step, ec)
		if err == nil && isEnabled != *svc.Enabled {
			if *svc.Enabled {
				reasons = append(reasons, "would enable service")
			} else {
				reasons = append(reasons, "would disable service")
			}
		}
	}

	// Unit file content compare — same logic as manageSystemdUnitFile,
	// just without writing.
	if svc.Unit != nil {
		unitPath := svc.Unit.Dest
		if unitPath == "" {
			unitPath = fmt.Sprintf("/etc/systemd/system/%s.service", renderedName)
		}
		desired, err := renderTemplateOrContent(svc.Unit.SrcTemplate, svc.Unit.Content, "service.unit", ec)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("would write unit file: %s (render error: %v)", unitPath, err))
		} else {
			// #nosec G304 -- unit file path from caller
			existing, readErr := os.ReadFile(unitPath)
			if readErr != nil || string(existing) != desired {
				reasons = append(reasons, fmt.Sprintf("would write unit file: %s", unitPath))
			}
		}
	}

	// Dropin content compare — same logic as manageSystemdDropin.
	if svc.Dropin != nil && svc.Dropin.Name != "" {
		dropinDir := fmt.Sprintf("/etc/systemd/system/%s.service.d", renderedName)
		dropinPath := filepath.Join(dropinDir, svc.Dropin.Name)
		desired, err := renderTemplateOrContent(svc.Dropin.SrcTemplate, svc.Dropin.Content, "service.dropin", ec)
		if err != nil {
			reasons = append(reasons, fmt.Sprintf("would write dropin: %s (render error: %v)", dropinPath, err))
		} else {
			// #nosec G304 -- dropin path from caller
			existing, readErr := os.ReadFile(dropinPath)
			if readErr != nil || string(existing) != desired {
				reasons = append(reasons, fmt.Sprintf("would write dropin: %s", dropinPath))
			}
		}
	}

	if len(reasons) == 0 {
		result.Reason = "service already in desired state"
		return result, nil
	}
	result.WouldChange = true
	result.Reason = strings.Join(reasons, "; ")
	return result, nil
}
