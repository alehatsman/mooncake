// Package service implements the os.service action handler. The
// handler is a thin runtime-dispatcher: at apply time it switches on
// runtime.GOOS and calls into one of three per-OS sub-packages that
// own the actual systemctl / launchctl / SCM logic.
//
// Layout:
//
//	service/                  ← Handler + dispatcher + plan mode (this file)
//	service/shared/           ← shared types (OsServiceReverseInfo,
//	                             state constants) + shared helpers
//	                             (BecomeAwareCommand, RunBecomeAware,
//	                             WriteFileWithPrivileges, ...)
//	service/linux/            ← systemd backend (Handle, CapturePriorState)
//	service/darwin/           ← launchd backend (Handle, CapturePriorState)
//	service/windows/          ← SCM/PowerShell backend (Handle, CapturePriorState)
//
// Why a runtime dispatcher rather than build-tag-per-OS files: the
// registry (internal/register) and schemagen need to see all
// handlers regardless of build target, so each sub-package must
// compile on every host. None of the per-OS code uses platform-only
// syscalls — every backend shells out via os/exec — so the runtime
// dispatch costs only a one-time GOOS switch.
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/service/darwin"
	"github.com/alehatsman/mooncake/internal/actions/service/linux"
	"github.com/alehatsman/mooncake/internal/actions/service/shared"
	"github.com/alehatsman/mooncake/internal/actions/service/windows"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Service state vocabulary. Re-exported from the shared sub-package
// so callers and tests of this package (Validate, Run, the typed-
// Diff Snapshot) can reference them without an extra import.
const (
	ServiceStateStarted   = shared.StateStarted
	ServiceStateStopped   = shared.StateStopped
	ServiceStateReloaded  = shared.StateReloaded
	ServiceStateRestarted = shared.StateRestarted
)

// OsServiceReverseInfo is re-exported from shared so existing
// callers of `service.OsServiceReverseInfo` (Reverse, runlog, fleet
// wire-encoding) keep working unchanged.
type OsServiceReverseInfo = shared.OsServiceReverseInfo

// Handler implements the os.service action handler.
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
		SupportedPlatforms: []string{"linux", "darwin", "windows"},
		RequiresSudo:       true,
		ImplementsCheck:    true,
	}
}

// Permissions implements actions.Permitter (spec-22). os.service
// always declares Sudo: every supported backend (systemd via
// systemctl, launchd via launchctl, Windows SCM via sc.exe /
// Set-Service) needs elevated privileges to start/stop/enable units
// and reload configuration. Unconditional because the action TYPE
// is the indicator — even a status-only `state: started` check on
// a system service requires the right to introspect via the
// managing daemon.
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

	if serviceAction.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if serviceAction.State != "" {
		isValid := false
		for _, s := range shared.ValidStates {
			if serviceAction.State == s {
				isValid = true
				break
			}
		}
		if !isValid {
			return fmt.Errorf("invalid state %q, must be one of: %v", serviceAction.State, shared.ValidStates)
		}
	}

	return nil
}

// runApply wraps HandleService with reverse-capture: BEFORE
// delegating, query the platform-appropriate "what's the prior
// state?" probe, then bolt the captured snapshot onto
// ec.CurrentResult.ReverseData after HandleService returns
// (HandleService writes its Result via a deferred side-effect into
// ec.CurrentResult — we don't change that contract).
//
// Dispatch by runtime.GOOS:
//   - linux   → linux.CapturePriorState (systemctl is-active / is-enabled)
//   - darwin  → darwin.CapturePriorState (launchctl print loaded?)
//   - windows → windows.CapturePriorState (Get-Service Status + StartType)
//   - other   → no capture; Reverse() returns (nil, nil) for the
//     nil-ReverseData case.
func runApply(step *config.Step, ec *executor.ExecutionContext) (actions.Result, error) {
	var priorInfo *OsServiceReverseInfo
	if step.OsService != nil {
		switch runtime.GOOS {
		case "linux":
			priorInfo = linux.CapturePriorState(step.OsService.Name, *step, ec)
		case "darwin":
			priorInfo = darwin.CapturePriorState(step.OsService.Name, *step, ec)
		case "windows":
			priorInfo = windows.CapturePriorState(step.OsService.Name, *step, ec)
		}
	}

	err := HandleService(*step, ec)

	if ec.CurrentResult != nil && priorInfo != nil && ec.CurrentResult.Changed {
		ec.CurrentResult.ReverseData = priorInfo
	}
	return ec.CurrentResult, err
}

// HandleService manages services across different platforms. The
// per-OS branch dispatches to a sub-package; this function owns the
// shared name/state validation that runs before any platform code.
func HandleService(step config.Step, ec *executor.ExecutionContext) error {
	serviceAction := step.OsService
	if serviceAction == nil {
		return &executor.SetupError{Component: "service", Issue: "no service configuration specified"}
	}

	if serviceAction.Name == "" {
		return &executor.StepValidationError{Field: "name", Message: "service name is required"}
	}

	renderedName, err := ec.Svc.Template.Render(serviceAction.Name, ec.GetVariables())
	if err != nil {
		return &executor.RenderError{Field: "service.name", Cause: err}
	}

	if serviceAction.State != "" {
		isValid := false
		for _, s := range shared.ValidStates {
			if serviceAction.State == s {
				isValid = true
				break
			}
		}
		if !isValid {
			return &executor.StepValidationError{
				Field:   "state",
				Message: fmt.Sprintf("invalid state %q, must be one of: %v", serviceAction.State, shared.ValidStates),
			}
		}
	}

	switch runtime.GOOS {
	case "linux":
		return linux.Handle(renderedName, serviceAction, step, ec)
	case "darwin":
		return darwin.Handle(renderedName, serviceAction, step, ec)
	case "windows":
		return windows.Handle(renderedName, serviceAction, step, ec)
	default:
		return &executor.SetupError{
			Component: "service",
			Issue:     fmt.Sprintf("service management not supported on %s", runtime.GOOS),
		}
	}
}

// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` on an os.service step actually retries — useful when a
// systemctl race, slow unit startup, or transient PolicyKit denial
// causes a flaky failure. Run handles its own escalation via
// shared.BecomeAwareCommand (spec-69 phase-5-audit exemption — see
// service/shared.go); RunRaw just opts the action into the
// executor's centralized retry loop without disturbing that path.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

// Run is the unified entry point. Plan mode queries systemctl
// is-active / is-enabled to predict state and enabled-flag changes,
// and compares unit / dropin file contents against the rendered
// templates. Apply mode delegates to HandleService via runApply.
//
// Plan-mode introspection is linux-only today: launchd's plan
// surface (compare loaded? vs. desired) and windows SCM's plan
// surface (compare current state/start-type vs. desired) are both
// out of scope for v1 — plan returns "not implemented" for those
// platforms and the apply path still works.
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
	result.Operation = executor.OpUpdate

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
	result.Target = renderedName

	var reasons []string

	// State change prediction.
	if svc.State != "" {
		current, _ := linux.GetServiceState(renderedName, *step, ec)
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
		isEnabled, err := linux.IsServiceEnabled(renderedName, *step, ec)
		if err == nil && isEnabled != *svc.Enabled {
			if *svc.Enabled {
				reasons = append(reasons, "would enable service")
			} else {
				reasons = append(reasons, "would disable service")
			}
		}
	}

	// Unit file content compare — same logic as the systemd
	// backend's manageUnitFile, just without writing.
	if svc.Unit != nil {
		unitPath := svc.Unit.Dest
		if unitPath == "" {
			unitPath = fmt.Sprintf("/etc/systemd/system/%s.service", renderedName)
		}
		desired, err := shared.RenderTemplateOrContent(svc.Unit.SrcTemplate, svc.Unit.Content, "service.unit", ec)
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

	// Dropin content compare — same logic as systemd backend's
	// manageDropin.
	if svc.Dropin != nil && svc.Dropin.Name != "" {
		dropinDir := fmt.Sprintf("/etc/systemd/system/%s.service.d", renderedName)
		dropinPath := filepath.Join(dropinDir, svc.Dropin.Name)
		desired, err := shared.RenderTemplateOrContent(svc.Dropin.SrcTemplate, svc.Dropin.Content, "service.dropin", ec)
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
