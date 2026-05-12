package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode queries
// systemctl is-active / is-enabled to predict state and enabled-flag
// changes, and compares unit / dropin file contents against the
// rendered templates. Execute mode delegates to HandleService.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	if ctx.Mode() != actions.ModePlan {
		return nil, HandleService(*step, ec)
	}

	result := executor.NewResult()
	result.Checkable = true

	if runtime.GOOS != "linux" {
		result.Reason = fmt.Sprintf("service state inspection not implemented on %s", runtime.GOOS)
		result.Checkable = false
		return result, nil
	}

	svc := step.Service
	if svc == nil {
		return result, fmt.Errorf("service action requires service configuration")
	}

	renderedName, err := ec.Template.Render(svc.Name, ec.Variables)
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
