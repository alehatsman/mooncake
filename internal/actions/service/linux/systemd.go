// Package linux implements the systemd backend for os.service.
// Called from the parent package's dispatcher when runtime.GOOS ==
// "linux". Sub-package lives here (rather than as a file in the
// parent) to keep the per-OS LOC out of the parent's soft-cap
// budget (CLAUDE.md §1).
package linux

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

// Handle manages a systemd service: unit-file write, drop-in file
// write, daemon-reload, state (started/stopped/...), enabled flag.
// Each branch is independently idempotent — if nothing diverges from
// the desired state, the unit isn't touched.
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

	if serviceAction.Unit != nil {
		unitChanged, err := manageUnitFile(serviceName, serviceAction.Unit, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
			return err
		}
		if unitChanged {
			changed = true
			operations = append(operations, "unit file updated")
		}
	}

	if serviceAction.Dropin != nil {
		dropinChanged, err := manageDropin(serviceName, serviceAction.Dropin, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
			return err
		}
		if dropinChanged {
			changed = true
			operations = append(operations, "drop-in updated")
		}
	}

	// daemon-reload fires when (a) unit or drop-in changed, or
	// (b) the operator explicitly asked for it via daemon_reload: true.
	if (changed && serviceAction.DaemonReload) || (serviceAction.DaemonReload && !changed) {
		if err := DaemonReload(step, ec); err != nil {
			shared.MarkStepFailed(result, step, ec)
			return err
		}
		operations = append(operations, "daemon-reload")
	}

	if serviceAction.State != "" {
		stateChanged, err := ManageServiceState(serviceName, serviceAction.State, step, ec)
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
		enableChanged, err := ManageServiceEnabled(serviceName, *serviceAction.Enabled, step, ec)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
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

// CapturePriorState queries systemctl is-active and is-enabled for
// the named unit, returning a snapshot suitable for Result.ReverseData.
// Best-effort: failures default to false (not active, not enabled).
func CapturePriorState(serviceName string, step config.Step, ec *executor.ExecutionContext) *shared.OsServiceReverseInfo {
	info := &shared.OsServiceReverseInfo{Name: serviceName, Platform: "linux"}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStartedIntent = false // No `started` flag on ServiceAction yet
		info.HadStateIntent = step.OsService.State != ""
	}

	state, err := GetServiceState(serviceName, step, ec)
	if err == nil {
		info.PriorActive = state == "active"
	}
	if enabled, err := IsServiceEnabled(serviceName, step, ec); err == nil {
		info.PriorEnabled = enabled
	}
	return info
}

// manageUnitFile creates or updates a systemd unit file in
// /etc/systemd/system/<name>.service (or step.Unit.Dest if set).
// Idempotent: a byte-identical existing file is left alone.
func manageUnitFile(serviceName string, unit *config.ServiceUnit, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	unitPath := unit.Dest
	if unitPath == "" {
		unitPath = fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	}

	content, err := shared.RenderTemplateOrContent(unit.SrcTemplate, unit.Content, "service.unit", ec)
	if err != nil {
		return false, err
	}

	// #nosec G304 - This is a provisioning tool that manages service unit files
	existingContent, readErr := os.ReadFile(unitPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Unit file %s already up to date", unitPath)
		return false, nil
	}

	if err := shared.WriteFileWithPrivileges(unitPath, []byte(content), unit.Mode, step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Unit file written: %s", unitPath)
	return true, nil
}

// manageDropin creates or updates a systemd drop-in file in
// /etc/systemd/system/<name>.service.d/<dropin.Name>. Idempotent.
func manageDropin(serviceName string, dropin *config.ServiceDropin, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	if dropin.Name == "" {
		return false, &executor.StepValidationError{Field: "service.dropin.name", Message: "drop-in name is required"}
	}

	dropinDir := fmt.Sprintf("/etc/systemd/system/%s.service.d", serviceName)
	dropinPath := filepath.Join(dropinDir, dropin.Name)

	content, err := shared.RenderTemplateOrContent(dropin.SrcTemplate, dropin.Content, "service.dropin", ec)
	if err != nil {
		return false, err
	}

	// #nosec G304 - This is a provisioning tool that manages service drop-in files
	existingContent, readErr := os.ReadFile(dropinPath)
	if readErr == nil && string(existingContent) == content {
		ec.Svc.Logger.Debugf("  Drop-in file %s already up to date", dropinPath)
		return false, nil
	}

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

	if err := shared.WriteFileWithPrivileges(dropinPath, []byte(content), "0644", step, ec); err != nil {
		return false, err
	}

	ec.Svc.Logger.Debugf("  Drop-in file written: %s", dropinPath)
	return true, nil
}

// DaemonReload runs `systemctl daemon-reload`. Exported so plan-mode
// (in the parent package) can call it directly.
func DaemonReload(step config.Step, ec *executor.ExecutionContext) error {
	ec.Svc.Logger.Debugf("  Running systemctl daemon-reload")
	_, err := shared.RunBecomeAware(step, ec, "daemon-reload", "systemctl", "daemon-reload")
	return err
}

// ManageServiceState dispatches `systemctl <verb> <name>` for the
// requested state. Returns (changed, error). started/stopped are
// idempotent (no-op when already in target state); restart/reload
// always emit the verb (the operator's intent is "force a cycle").
func ManageServiceState(serviceName, desiredState string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	currentState, err := GetServiceState(serviceName, step, ec)
	if err != nil {
		return false, err
	}

	var action string
	switch desiredState {
	case shared.StateStarted:
		if currentState == "active" {
			ec.Svc.Logger.Debugf("  Service %s already active", serviceName)
			return false, nil
		}
		action = "start"
	case shared.StateStopped:
		if currentState == "inactive" || currentState == "failed" {
			ec.Svc.Logger.Debugf("  Service %s already stopped", serviceName)
			return false, nil
		}
		action = "stop"
	case shared.StateRestarted:
		action = "restart"
	case shared.StateReloaded:
		action = "reload"
	default:
		return false, &executor.StepValidationError{
			Field:   "state",
			Message: fmt.Sprintf("unsupported state: %s", desiredState),
		}
	}

	ec.Svc.Logger.Debugf("  Running systemctl %s %s", action, serviceName)
	if _, err := shared.RunBecomeAware(step, ec, "systemctl "+action, "systemctl", action, serviceName); err != nil {
		return false, err
	}
	return true, nil
}

// GetServiceState returns "active" / "inactive" / "failed" / etc. as
// reported by systemctl is-active. is-active exits non-zero for
// inactive services, but we want the state string regardless, so the
// command error is intentionally ignored.
func GetServiceState(serviceName string, step config.Step, ec *executor.ExecutionContext) (string, error) {
	cmd, err := shared.BecomeAwareCommand(step, ec, "systemctl", "is-active", serviceName)
	if err != nil {
		return "", err
	}
	output, _ := cmd.Output() // is-active rc != 0 for inactive — ignore
	state := strings.TrimSpace(string(output))
	ec.Svc.Logger.Debugf("  Service %s current state: %s", serviceName, state)
	return state, nil
}

// ManageServiceEnabled flips the unit's enabled state via systemctl
// enable/disable. Idempotent: no-op when the unit is already in the
// requested state.
func ManageServiceEnabled(serviceName string, shouldBeEnabled bool, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	isEnabled, err := IsServiceEnabled(serviceName, step, ec)
	if err != nil {
		return false, err
	}

	if isEnabled == shouldBeEnabled {
		ec.Svc.Logger.Debugf("  Service %s enabled status already correct: %v", serviceName, isEnabled)
		return false, nil
	}

	action := "disable"
	if shouldBeEnabled {
		action = "enable"
	}

	ec.Svc.Logger.Debugf("  Running systemctl %s %s", action, serviceName)
	if _, err := shared.RunBecomeAware(step, ec, "systemctl "+action, "systemctl", action, serviceName); err != nil {
		return false, err
	}
	return true, nil
}

// IsServiceEnabled checks if a systemd unit is enabled. systemctl
// is-enabled exits non-zero for disabled / masked / not-found units;
// we treat the output strings "enabled" / "static" / "indirect" as
// enabled and everything else as disabled.
func IsServiceEnabled(serviceName string, step config.Step, ec *executor.ExecutionContext) (bool, error) {
	cmd, err := shared.BecomeAwareCommand(step, ec, "systemctl", "is-enabled", serviceName)
	if err != nil {
		return false, err
	}
	output, _ := cmd.Output() // is-enabled rc != 0 for disabled — ignore
	status := strings.TrimSpace(string(output))
	isEnabled := status == "enabled" || status == "static" || status == "indirect"
	ec.Svc.Logger.Debugf("  Service %s enabled status: %s (treated as enabled: %v)", serviceName, status, isEnabled)
	return isEnabled, nil
}
