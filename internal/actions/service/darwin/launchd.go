// Package darwin implements the launchd backend for os.service.
// Called from the parent package's dispatcher when runtime.GOOS ==
// "darwin". Sub-package lives here (rather than as a file in the
// parent) to keep the per-OS LOC out of the parent's soft-cap
// budget (CLAUDE.md §1).
package darwin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/service/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handle manages a launchd service: plist file, load/unload via
// bootstrap/bootout, and state via kickstart/kill. Each branch is
// independently idempotent.
func Handle(serviceName string, serviceAction *config.ServiceAction, step config.Step, ec *executor.ExecutionContext) error {
	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	changed := false
	operations := []string{}

	// User agent vs. system daemon: become: true selects the system
	// LaunchDaemons / system domain; otherwise we manage the user's
	// LaunchAgents under their home and the gui/<uid> domain.
	isSystem := step.ShouldBecome()
	domain := GetDomain(isSystem)

	if serviceAction.Unit != nil {
		plistChanged, err := managePlist(serviceName, serviceAction.Unit, isSystem, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
			return err
		}
		if plistChanged {
			changed = true
			operations = append(operations, "plist updated")
		}
	}

	plistPath := GetPlistPath(serviceName, serviceAction.Unit, isSystem)
	serviceID := fmt.Sprintf("%s/%s", domain, serviceName)

	isLoaded, err := IsServiceLoaded(serviceID, step, ec)
	if err != nil {
		shared.MarkStepFailed(result, step, ec)
		return err
	}

	if serviceAction.State != "" {
		stateChanged, err := manageState(serviceName, serviceID, plistPath, domain, serviceAction.State, isLoaded, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
			return err
		}
		if stateChanged {
			changed = true
			operations = append(operations, fmt.Sprintf("service %s", serviceAction.State))
		}
	}

	if serviceAction.Enabled != nil {
		enableChanged, err := manageEnabled(serviceID, plistPath, domain, *serviceAction.Enabled, isLoaded, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
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

	result.Changed = changed
	result.Rc = 0
	result.Failed = false

	ec.EmitEvent(events.EventServiceManaged, events.ServiceManagementData{
		Service:    serviceName,
		State:      serviceAction.State,
		Enabled:    serviceAction.Enabled,
		Changed:    changed,
		Operations: operations,
		DryRun:     ec.Mode() == actions.ModePlan,
	})

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

// CapturePriorState is the darwin analogue of the systemd capture.
// launchd's surface is narrower than systemd's: IsServiceLoaded
// returns one bool that conflates two systemd concepts (active +
// enabled). For reverse purposes we map that single signal to
// PriorEnabled; PriorActive carries the same bit as metadata.
// Reverse() on darwin only honors the HadEnabledIntent path.
func CapturePriorState(serviceName string, step config.Step, ec *executor.ExecutionContext) *shared.OsServiceReverseInfo {
	info := &shared.OsServiceReverseInfo{Name: serviceName, Platform: "darwin"}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStateIntent = step.OsService.State != ""
	}
	isSystem := step.ShouldBecome()
	domain := GetDomain(isSystem)
	serviceID := fmt.Sprintf("%s/%s", domain, serviceName)
	loaded, err := IsServiceLoaded(serviceID, step, ec)
	if err == nil {
		info.PriorEnabled = loaded
		// Conservative carry-through: PriorActive mirrors the loaded
		// bit. Reverse() doesn't consume it on darwin, but it makes
		// fleet-status / debugging views consistent across OSes.
		info.PriorActive = loaded
	}
	return info
}

// GetDomain returns the appropriate launchd domain (system or
// per-user). Exported because CapturePriorState needs it too.
func GetDomain(isSystem bool) string {
	if isSystem {
		return "system"
	}
	return fmt.Sprintf("gui/%d", os.Getuid())
}

// GetPlistPath returns the plist file path for a launchd service.
// Defaults to /Library/LaunchDaemons for system services and
// ~/Library/LaunchAgents for user services.
func GetPlistPath(serviceName string, unit *config.ServiceUnit, isSystem bool) string {
	if unit != nil && unit.Dest != "" {
		return unit.Dest
	}

	if isSystem {
		return fmt.Sprintf("/Library/LaunchDaemons/%s.plist", serviceName)
	}

	homeDir := os.Getenv("HOME")
	return fmt.Sprintf("%s/Library/LaunchAgents/%s.plist", homeDir, serviceName)
}

func managePlist(serviceName string, unit *config.ServiceUnit, isSystem bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	plistPath := GetPlistPath(serviceName, unit, isSystem)

	content, err := shared.RenderTemplateOrContent(unit.SrcTemplate, unit.Content, "service.unit", ec)
	if err != nil {
		return false, err
	}

	// #nosec G304 - This is a provisioning tool that reads plist files from validated paths
	existingContent, readErr := os.ReadFile(plistPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Plist file %s already up to date", plistPath)
		return false, nil
	}

	plistDir := filepath.Dir(plistPath)
	// #nosec G301 - Plist directories need to be readable by launchd (0755 is appropriate)
	if err := os.MkdirAll(plistDir, 0755); err != nil {
		return false, &executor.FileOperationError{Operation: "mkdir", Path: plistDir, Cause: err}
	}

	if err := shared.WriteFileWithPrivileges(plistPath, []byte(content), unit.Mode, step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Plist file written: %s", plistPath)
	return true, nil
}

// IsServiceLoaded checks if a launchd service is loaded via
// `launchctl print <serviceID>`. Returns (false, nil) for
// not-loaded; non-nil error only for transport/auth issues.
//
// F005 final-mile rationale: routing through BecomeAwareCommand
// catches the Linux + become asymmetry the previous inline `sudo`
// construction missed.
func IsServiceLoaded(serviceID string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	cmd, err := shared.BecomeAwareCommand(step, ec, "launchctl", "print", serviceID)
	if err != nil {
		return false, err
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "Could not find service") {
			ec.Svc.Logger.Debugf("  Service %s is not loaded", serviceID)
			return false, nil
		}
		ec.Svc.Logger.Debugf("  Error checking service status: %v", err)
		return false, nil
	}

	ec.Svc.Logger.Debugf("  Service %s is loaded", serviceID)
	return true, nil
}

func manageState(serviceName, serviceID, plistPath, domain string, desiredState string, isLoaded bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	switch desiredState {
	case shared.StateStarted:
		if !isLoaded {
			if err := bootstrap(domain, plistPath, step, ec); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := kickstart(serviceID, false, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case shared.StateStopped:
		if !isLoaded {
			ec.Svc.Logger.Debugf("  Service %s already stopped (not loaded)", serviceName)
			return false, nil
		}
		if err := kill(serviceID, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case shared.StateRestarted:
		if !isLoaded {
			if err := bootstrap(domain, plistPath, step, ec); err != nil {
				return false, err
			}
			return true, nil
		}
		if err := kickstart(serviceID, true, step, ec); err != nil {
			return false, err
		}
		return true, nil

	case shared.StateReloaded:
		// launchd doesn't have a direct reload, treat as restart.
		return manageState(serviceName, serviceID, plistPath, domain, shared.StateRestarted, isLoaded, step, ec)

	default:
		return false, &executor.StepValidationError{
			Field:   "state",
			Message: fmt.Sprintf("unsupported state: %s", desiredState),
		}
	}
}

func manageEnabled(serviceID, plistPath, domain string, shouldBeEnabled bool, isLoaded bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	if shouldBeEnabled && !isLoaded {
		if err := bootstrap(domain, plistPath, step, ec); err != nil {
			return false, err
		}
		return true, nil
	}

	if !shouldBeEnabled && isLoaded {
		if err := bootout(domain, plistPath, step, ec); err != nil {
			return false, err
		}
		return true, nil
	}

	ec.Svc.Logger.Debugf("  Service %s already in desired enabled state: %v", serviceID, shouldBeEnabled)
	return false, nil
}

// executeLaunchctlCommand runs a launchctl verb with idempotency
// markers — if the verb's stderr names a "service already loaded /
// unloaded / not found" condition, we treat the rc!=0 as success
// and avoid the error wrap. Can't share shared.RunBecomeAware
// because that path doesn't inspect stderr for idempotency markers.
func executeLaunchctlCommand(command, domain, plistPath string, step config.Step, ec *executor.ExecutionContext, idempotencyCheck []string, successMsg, errorMsg string) error {
	ec.Svc.Logger.Debugf("  Running launchctl %s %s %s", command, domain, plistPath)
	cmd, err := shared.BecomeAwareCommand(step, ec, "launchctl", command, domain, plistPath)
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

func bootstrap(domain, plistPath string, step config.Step, ec *executor.ExecutionContext) error {
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

func bootout(domain, plistPath string, step config.Step, ec *executor.ExecutionContext) error {
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

func kickstart(serviceID string, killAndRestart bool, step config.Step, ec *executor.ExecutionContext) error {
	args := []string{"kickstart"}
	if killAndRestart {
		args = append(args, "-k")
	}
	args = append(args, serviceID)

	ec.Svc.Logger.Debugf("  Running launchctl %s", strings.Join(args, " "))

	cmd, err := shared.BecomeAwareCommand(step, ec, "launchctl", args...)
	if err != nil {
		return err
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

func kill(serviceID string, step config.Step, ec *executor.ExecutionContext) error {
	ec.Svc.Logger.Debugf("  Running launchctl kill SIGTERM %s", serviceID)

	cmd, err := shared.BecomeAwareCommand(step, ec, "launchctl", "kill", "SIGTERM", serviceID)
	if err != nil {
		return err
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
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
