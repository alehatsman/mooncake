// Package windows implements the Windows SCM backend for os.service.
// Called from the parent package's dispatcher when runtime.GOOS ==
// "windows". Sub-package lives here (rather than as a file in the
// parent) to keep the per-OS LOC out of the parent's soft-cap
// budget (CLAUDE.md §1).
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
package windows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/service/shared"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// ServiceState mirrors the fields we read from `Get-Service` via
// ConvertTo-Json. PowerShell renders enums as integers when piped
// through ConvertTo-Json — Status=Running becomes 4, etc. We map
// back to the string form locally for legibility.
type ServiceState struct {
	Name        string `json:"Name"`
	DisplayName string `json:"DisplayName"`
	Status      int    `json:"Status"`    // 1=Stopped 2=StartPending 3=StopPending 4=Running 5=ContinuePending 6=PausePending 7=Paused
	StartType   int    `json:"StartType"` // 2=Automatic 3=Manual 4=Disabled (System=0, Boot=1 also exist but apply to drivers)
	CanStop     bool   `json:"CanStop"`
}

const (
	statusStopped = 1
	statusRunning = 4

	startTypeAutomatic = 2
	startTypeDisabled  = 4
)

// Exec is the test seam for shelling to powershell. Tests override
// this with a recorder; production calls RealExec. F2: ctx is the
// run-wide cancel.
var Exec = RealExec

// RealExec runs a PowerShell snippet via `powershell -NoProfile
// -NonInteractive -Command <script>`. Stderr from a non-zero exit is
// folded into the returned error so callers don't lose the SCM-side
// diagnostic.
func RealExec(ctx context.Context, script string) (string, error) {
	// #nosec G204 -- script is built from validated config fields
	// plus PowerShell cmdlet names; operator strings go through
	// quotePS to escape single quotes.
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	stdout, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("powershell exit %d: %s", ee.ExitCode(), strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("powershell: %w", err)
	}
	return string(stdout), nil
}

// Handle manages Windows services via PowerShell SCM cmdlets.
// Mirrors the structure of darwin.Handle — state management,
// enabled management, changed-bookkeeping, event emission — minus
// the unit-file path (refused in v1).
func Handle(serviceName string, serviceAction *config.ServiceAction, step config.Step, ec *executor.ExecutionContext) error {
	result := executor.NewResult()
	result.Operation = executor.OpUpdate
	result.Target = serviceName
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		if !result.Changed && !result.Failed {
			result.Operation = executor.OpNoop
		}
		ec.CurrentResult = result
	}()

	// v1 refusal: Unit / Dropin have no SCM analog. Surface as a
	// SetupError so the operator sees the scope constraint at
	// validate-ish time rather than a confusing PowerShell error.
	if serviceAction.Unit != nil {
		shared.MarkStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue:     "unit: not supported on windows (v1 manages state + enabled only; create services via sc.exe / MSI / registry first)",
		}
	}
	if serviceAction.Dropin != nil {
		shared.MarkStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue:     "dropin: not supported on windows (no SCM analog for systemd drop-in files)",
		}
	}

	current, err := ReadService(ec.Svc.Ctx, serviceName)
	if err != nil {
		shared.MarkStepFailed(result, step, ec)
		return err
	}
	if current == nil {
		shared.MarkStepFailed(result, step, ec)
		return &executor.SetupError{
			Component: "windows service",
			Issue: fmt.Sprintf("service %q not found in SCM; v1 doesn't create services (use sc.exe create or an MSI first)",
				serviceName),
		}
	}

	changed := false
	operations := []string{}

	if serviceAction.State != "" {
		stateChanged, err := manageState(ec.Svc.Ctx, serviceName, serviceAction.State, current)
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
		enabledChanged, err := manageEnabled(ec.Svc.Ctx, serviceName, *serviceAction.Enabled, current)
		if err != nil {
			shared.MarkStepFailed(result, step, ec)
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

// CapturePriorState is the windows analogue of the systemd/launchd
// captures. Snapshots the service's pre-apply (Status, StartType)
// tuple plus the intent flags from the step. Best-effort: probe
// failures yield an info payload with zero values (matches the
// linux/darwin behaviour).
//
// Reverse policy (enforced in the parent's reverse.go via Platform
// tag): like darwin, only HadEnabledIntent is honored on windows.
// SCM's StartType (Automatic / Disabled) inverts cleanly; Running
// / Stopped is transient and inverting it across a reboot is murky
// (the service may have auto-started before reverse-apply runs).
// State reverse on windows would over-promise.
func CapturePriorState(serviceName string, step config.Step, ec *executor.ExecutionContext) *shared.OsServiceReverseInfo {
	info := &shared.OsServiceReverseInfo{Name: serviceName, Platform: "windows"}
	if step.OsService != nil {
		info.HadEnabledIntent = step.OsService.Enabled != nil
		info.HadStateIntent = step.OsService.State != ""
	}
	captureCtx := context.Background()
	if ec != nil && ec.Svc != nil && ec.Svc.Ctx != nil {
		captureCtx = ec.Svc.Ctx
	}
	current, err := ReadService(captureCtx, serviceName)
	if err != nil || current == nil {
		return info // probe failed / service missing — zero values
	}
	info.PriorActive = current.Status == statusRunning
	// Map SCM StartType to a single boolean: Automatic is "enabled"
	// for our purposes; Manual + Disabled both count as "not
	// enabled" since neither auto-starts on boot. The intent behind
	// Enabled=true on windows is "this service starts with the OS";
	// Automatic is the only value that delivers that.
	info.PriorEnabled = current.StartType == startTypeAutomatic
	return info
}

// ReadService queries SCM via Get-Service. Returns nil (not an
// error) when the service doesn't exist — distinguishing "missing"
// from "PowerShell broken" lets the caller emit a clear diagnostic.
func ReadService(ctx context.Context, name string) (*ServiceState, error) {
	script := fmt.Sprintf(
		"$s = Get-Service -Name %s -ErrorAction SilentlyContinue; "+
			"if ($s) { $s | Select-Object Name,DisplayName,Status,StartType,CanStop | ConvertTo-Json -Compress }",
		QuotePS(name),
	)
	out, err := Exec(ctx, script)
	if err != nil {
		return nil, fmt.Errorf("Get-Service %s: %w", name, err)
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	var state ServiceState
	if err := json.Unmarshal([]byte(trimmed), &state); err != nil {
		return nil, fmt.Errorf("parse Get-Service output: %w (raw: %q)", err, trimmed)
	}
	return &state, nil
}

func manageState(ctx context.Context, name, desiredState string, current *ServiceState) (bool, error) {
	switch desiredState {
	case shared.StateStarted:
		if current.Status == statusRunning {
			return false, nil
		}
		if _, err := Exec(ctx, fmt.Sprintf("Start-Service -Name %s", QuotePS(name))); err != nil {
			return false, fmt.Errorf("Start-Service %s: %w", name, err)
		}
		return true, nil
	case shared.StateStopped:
		if current.Status == statusStopped {
			return false, nil
		}
		if !current.CanStop {
			return false, fmt.Errorf("service %s is not stoppable (CanStop=false; usually a system service)", name)
		}
		if _, err := Exec(ctx, fmt.Sprintf("Stop-Service -Name %s", QuotePS(name))); err != nil {
			return false, fmt.Errorf("Stop-Service %s: %w", name, err)
		}
		return true, nil
	case shared.StateRestarted:
		// Restart-Service handles "wasn't running" cleanly — it
		// stops if running, then starts. Always Changed=true since
		// we deliberately bounced the service.
		if _, err := Exec(ctx, fmt.Sprintf("Restart-Service -Name %s -Force", QuotePS(name))); err != nil {
			return false, fmt.Errorf("Restart-Service %s: %w", name, err)
		}
		return true, nil
	case shared.StateReloaded:
		return false, fmt.Errorf("service %s: state=reloaded has no SCM analog on windows", name)
	}
	return false, fmt.Errorf("service %s: unknown state %q", name, desiredState)
}

// manageEnabled maps the Enabled bool to SCM's StartType: true →
// Automatic, false → Disabled. Manual is intentionally not exposed
// — it's a "user has to click Start" state that no os.service spec
// wants by default.
func manageEnabled(ctx context.Context, name string, enabled bool, current *ServiceState) (bool, error) {
	wantType := startTypeDisabled
	wantTypeName := "Disabled"
	if enabled {
		wantType = startTypeAutomatic
		wantTypeName = "Automatic"
	}
	if current.StartType == wantType {
		return false, nil
	}
	// Manual is a no-op intent — if the operator put the service at
	// Manual by hand, an `enabled: true` should still move it to
	// Automatic. Same logic in reverse for `enabled: false`.
	script := fmt.Sprintf("Set-Service -Name %s -StartupType %s",
		QuotePS(name), wantTypeName)
	if _, err := Exec(ctx, script); err != nil {
		return false, fmt.Errorf("Set-Service -StartupType %s: %w", wantTypeName, err)
	}
	return true, nil
}

// QuotePS is the local copy of platform_windows.go's quotePS. Kept
// package-private at this level (rather than shared) because the
// os.user and os.service packages are intentionally independent —
// sharing a helper would create a coupling that's not worth the
// saved 5 lines.
func QuotePS(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
