package service

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode queries systemctl
// is-active / is-enabled to predict whether state and enabled-status
// changes are needed, then reports a structured prediction. Execute
// mode delegates to the existing HandleService machinery, which
// already performs all the necessary platform-specific operations.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	if ctx.Mode() != actions.ModePlan {
		// Execute mode: delegate to the legacy machinery.
		return nil, HandleService(*step, ec)
	}

	// Plan mode: predict via systemctl queries (Linux only for now).
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
			// Not idempotent — always counts as a change.
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

	// Unit file / dropin management would change something if those
	// are set; we don't attempt to compare file contents here (more
	// involved). Mark as would-change to err on the side of accurate
	// preview.
	if svc.Unit != nil {
		reasons = append(reasons, "would manage unit file")
	}
	if svc.Dropin != nil {
		reasons = append(reasons, "would manage dropin file")
	}

	if len(reasons) == 0 {
		result.Reason = "service already in desired state"
		return result, nil
	}
	result.WouldChange = true
	result.Reason = strings.Join(reasons, "; ")
	return result, nil
}
