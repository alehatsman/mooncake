package service

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Windows-service backend for os.service. Despite the file having no
// build tag, every function here is only reached at runtime via the
// GOOS == "windows" branch in HandleService — the linux/darwin
// builds compile this file but never execute it. Keeping it
// tag-free makes the test seam easier (override the windowsExec
// hook from any platform's tests) and avoids forking another build-
// tag pair just for the win32 service code path.
//
// v1 scope (matches the kernel-honest principle: ship what we can
// stand behind):
//
//   - State management: started ↔ Running, stopped ↔ Stopped,
//     restarted = stop+start, reloaded → refused (no SCM analog;
//     Windows services that need "reload" usually do their own
//     SIGHUP-equivalent via control codes — outside os.service's
//     remit).
//   - Enabled management: Set-Service -StartupType. Automatic when
//     Enabled=true; Disabled when Enabled=false. Manual stays a
//     migration target operators can opt into via a future
//     `start_type:` field — v1 doesn't expose that knob.
//   - Read state: Get-Service -ErrorAction SilentlyContinue. JSON-
//     piped via ConvertTo-Json so the parser is exact.
//
// Out of scope (errors with a clear message):
//
//   - Unit / Dropin: Windows services have no unit-file shape that
//     maps to systemd / launchd. Operators create services
//     externally (sc.exe create, MSI installer, registry) and use
//     os.service for state/enabled management. A future
//     os.windows_service action could own the create / configure
//     surface.
//   - Missing service: Get-Service returns nothing. We refuse
//     loudly rather than silently no-op, so plans that target a
//     service that doesn't exist fail at apply time with a
//     diagnostic instead of marking changed=false.
//   - "reloaded" state: SCM has SERVICE_CONTROL_PARAMCHANGE but no
//     general "reload config" semantic across the action universe.
//     A future per-service custom-control field could opt in if
//     real demand surfaces.

// windowsServiceState mirrors the fields we read from `Get-Service`
// via ConvertTo-Json. PowerShell renders enums as integers when
// piped through ConvertTo-Json — Status=Running becomes 4, etc. We
// map back to the string form locally for legibility.
type windowsServiceState struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	Status      int    `json:"Status"`    // 1=Stopped 2=StartPending 3=StopPending 4=Running 5=ContinuePending 6=PausePending 7=Paused
	StartType   int    `json:"StartType"` // 2=Automatic 3=Manual 4=Disabled (System=0, Boot=1 also exist but apply to drivers)
	CanStop     bool   `json:"CanStop"`
}

const (
	winStatusStopped = 1
	winStatusRunning = 4

	winStartTypeAutomatic = 2
	winStartTypeManual    = 3
	winStartTypeDisabled  = 4
)

// windowsExec is the test seam for shelling to powershell. Tests
// override this with a recorder; production calls realWindowsExec.
// Signature matches both runPowerShell-style (no stdout) and
// captureWindowsExec (returns stdout for parsing) — callers route
// through the two thin wrappers below.
var windowsExec = realWindowsExec

func realWindowsExec(script string) (string, error) {
	// #nosec G204 -- script is built from validated config fields
	// plus PowerShell cmdlet names; operator strings go through
	// quoteWindowsPS to escape single quotes.
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	stdout, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("powershell exit %d: %s", ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("powershell: %w", err)
	}
	return string(stdout), nil
}

// handleWindowsService manages Windows services via PowerShell
// LocalAccounts cmdlets. Mirrors the structure of
// handleLaunchdService — state management, enabled management,
// changed-bookkeeping, event emission — minus the unit-file path
// (refused in v1).
func handleWindowsService(serviceName string, serviceAction *config.ServiceAction, step config.Step, ec *executor.ExecutionContext) error {
	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	// v1 refusal: Unit / Dropin have no SCM analog. Surface as a
	// SetupError so the operator sees the scope constraint at
	// validate-ish time rather than a confusing PowerShell error.
	if serviceAction.Unit != nil {
		markStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue:     "unit: not supported on windows (v1 manages state + enabled only; create services via sc.exe / MSI / registry first)",
		}
	}
	if serviceAction.Dropin != nil {
		markStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue:     "dropin: not supported on windows (no SCM analog for systemd drop-in files)",
		}
	}

	current, err := readWindowsService(serviceName)
	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}
	if current == nil {
		markStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue: fmt.Sprintf("service %q not found in SCM; v1 doesn't create services (use sc.exe create or an MSI first)",
				serviceName),
		}
	}

	changed := false
	operations := []string{}

	if serviceAction.State != "" {
		stateChanged, err := manageWindowsServiceState(serviceName, serviceAction.State, current)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if stateChanged {
			changed = true
			operations = append(operations, fmt.Sprintf("service %s", serviceAction.State))
		}
	}

	if serviceAction.Enabled != nil {
		enabledChanged, err := manageWindowsServiceEnabled(serviceName, *serviceAction.Enabled, current)
		if err != nil {
			markStepFailed(result, step, ec)
			return err
		}
		if enabledChanged {
			changed = true
			if *serviceAction.Enabled {
				operations = append(operations, "startup → Automatic")
			} else {
				operations = append(operations, "startup → Disabled")
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

// readWindowsService queries SCM via Get-Service. Returns nil
// (not an error) when the service doesn't exist — distinguishing
// "missing" from "PowerShell broken" lets the caller emit a clear
// diagnostic.
func readWindowsService(name string) (*windowsServiceState, error) {
	script := fmt.Sprintf(
		"$s = Get-Service -Name %s -ErrorAction SilentlyContinue; "+
			"if ($s) { $s | Select-Object Name,DisplayName,Status,StartType,CanStop | ConvertTo-Json -Compress }",
		quoteWindowsPS(name),
	)
	out, err := windowsExec(script)
	if err != nil {
		return nil, fmt.Errorf("Get-Service %s: %w", name, err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	var state windowsServiceState
	if err := json.Unmarshal([]byte(trimmed), &state); err != nil {
		return nil, fmt.Errorf("parse Get-Service output: %w (raw: %q)", err, trimmed)
	}
	return &state, nil
}

func manageWindowsServiceState(name, desiredState string, current *windowsServiceState) (bool, error) {
	switch desiredState {
	case ServiceStateStarted:
		if current.Status == winStatusRunning {
			return false, nil
		}
		if _, err := windowsExec(fmt.Sprintf("Start-Service -Name %s", quoteWindowsPS(name))); err != nil {
			return false, fmt.Errorf("Start-Service %s: %w", name, err)
		}
		return true, nil
	case ServiceStateStopped:
		if current.Status == winStatusStopped {
			return false, nil
		}
		if !current.CanStop {
			return false, fmt.Errorf("service %s is not stoppable (CanStop=false; usually a system service)", name)
		}
		if _, err := windowsExec(fmt.Sprintf("Stop-Service -Name %s", quoteWindowsPS(name))); err != nil {
			return false, fmt.Errorf("Stop-Service %s: %w", name, err)
		}
		return true, nil
	case ServiceStateRestarted:
		// Restart-Service handles "wasn't running" cleanly — it
		// stops if running, then starts. Always Changed=true since
		// we deliberately bounced the service.
		if _, err := windowsExec(fmt.Sprintf("Restart-Service -Name %s -Force", quoteWindowsPS(name))); err != nil {
			return false, fmt.Errorf("Restart-Service %s: %w", name, err)
		}
		return true, nil
	case ServiceStateReloaded:
		return false, fmt.Errorf("service %s: state=reloaded has no SCM analog on windows", name)
	}
	return false, fmt.Errorf("service %s: unknown state %q", name, desiredState)
}

// manageWindowsServiceEnabled maps the Enabled bool to SCM's
// StartType: true → Automatic, false → Disabled. Manual is
// intentionally not exposed — it's a "user has to click Start"
// state that no os.service spec wants by default.
func manageWindowsServiceEnabled(name string, enabled bool, current *windowsServiceState) (bool, error) {
	wantType := winStartTypeDisabled
	wantTypeName := "Disabled"
	if enabled {
		wantType = winStartTypeAutomatic
		wantTypeName = "Automatic"
	}
	if current.StartType == wantType {
		return false, nil
	}
	// Manual is a no-op intent — if the operator put the service
	// at Manual by hand, an `enabled: true` should still move it
	// to Automatic. Same logic in reverse for `enabled: false`.
	script := fmt.Sprintf("Set-Service -Name %s -StartupType %s",
		quoteWindowsPS(name), wantTypeName)
	if _, err := windowsExec(script); err != nil {
		return false, fmt.Errorf("Set-Service -StartupType %s: %w", wantTypeName, err)
	}
	return true, nil
}

// quoteWindowsPS is the local copy of platform_windows.go's quotePS.
// Kept package-private here (rather than shared) because the os.user
// and os.service packages are intentionally independent — sharing a
// helper would create a coupling that's not worth the saved 5 lines.
func quoteWindowsPS(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// captureWindowsPriorState is the windows analogue of
// captureSystemdPriorState / captureLaunchdPriorState. Snapshots the
// service's pre-apply (Status, StartType) tuple plus the intent
// flags from the step. Best-effort: probe failures yield an info
// payload with zero values (matches the linux/darwin behaviour).
//
// Reverse policy (enforced in reverse.go via Platform tag): like
// darwin, only HadEnabledIntent is honored on windows. SCM's
// StartType (Automatic / Disabled) inverts cleanly; Running /
// Stopped is transient and inverting it across a reboot is murky
// (the service may have auto-started before reverse-apply runs).
// State reverse on windows would over-promise.
func captureWindowsPriorState(serviceName string, step config.Step, _ *executor.ExecutionContext) *OsServiceReverseInfo {
	info := &OsServiceReverseInfo{Name: serviceName, Platform: "windows"}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStateIntent = step.OsService.State != ""
	}
	current, err := readWindowsService(serviceName)
	if err != nil || current == nil {
		return info // probe failed / service missing — zero values
	}
	info.PriorActive = current.Status == winStatusRunning
	// Map SCM StartType to a single boolean: Automatic is
	// "enabled" for our purposes; Manual + Disabled both count as
	// "not enabled" since neither auto-starts on boot. The intent
	// behind Enabled=true on windows is "this service starts with
	// the OS"; Automatic is the only value that delivers that.
	info.PriorEnabled = current.StartType == winStartTypeAutomatic
	return info
}
