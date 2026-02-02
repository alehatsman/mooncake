// Package preset implements the preset action handler.
// Presets expand into multiple steps with parameter injection.
package preset

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/presets"
)

// Handler implements the preset action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "preset",
		Description:    "Execute a preset by expanding it into steps",
		Category:       actions.CategorySystem,
		SupportsDryRun: true,
	}
}

// Validate validates the preset action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Preset == nil {
		return fmt.Errorf("preset action requires preset configuration")
	}
	if step.Preset.Name == "" {
		return fmt.Errorf("preset name is required")
	}
	return nil
}

// Execute executes the preset action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	invocation := step.Preset

	// Expand preset into steps
	expandedSteps, parametersNamespace, presetBaseDir, err := presets.ExpandPreset(invocation)
	if err != nil {
		return nil, fmt.Errorf("failed to expand preset '%s': %w", invocation.Name, err)
	}

	// Emit preset expanded event
	ec.EmitEvent(events.EventPresetExpanded, events.PresetData{
		Name:       invocation.Name,
		Parameters: invocation.With,
		StepsCount: len(expandedSteps),
	})

	ec.Logger.Infof("Expanding preset '%s' into %d steps", invocation.Name, len(expandedSteps))

	// Save current context for restoration
	savedVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		savedVariables[k] = v
	}
	savedCurrentDir := ec.CurrentDir

	// Merge parameters namespace into variables
	for k, v := range parametersNamespace {
		ec.Variables[k] = v
	}

	// Set CurrentDir to preset base directory for relative path resolution
	if presetBaseDir != "" {
		ec.CurrentDir = presetBaseDir
	}

	// Restore context after execution
	defer func() {
		// Remove parameters namespace
		for k := range parametersNamespace {
			delete(ec.Variables, k)
		}
		// Restore original variables (in case steps modified them)
		for k, v := range savedVariables {
			ec.Variables[k] = v
		}
		// Restore original directory
		ec.CurrentDir = savedCurrentDir
	}()

	// Execute expanded steps
	anyChanged := false
	for i, expandedStep := range expandedSteps {
		ec.Logger.Debugf("Executing preset step %d/%d: %s", i+1, len(expandedSteps), expandedStep.Name)

		if err := executor.ExecuteStep(expandedStep, ec); err != nil {
			return nil, fmt.Errorf("preset '%s' step %d failed: %w", invocation.Name, i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}

	// Create preset result
	result := executor.NewResult()
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Preset '%s' executed %d steps", invocation.Name, len(expandedSteps))

	// Emit preset completed event
	ec.EmitEvent(events.EventPresetCompleted, events.PresetData{
		Name:       invocation.Name,
		Parameters: invocation.With,
		StepsCount: len(expandedSteps),
		Changed:    anyChanged,
	})

	ec.Logger.Infof("Preset '%s' completed: changed=%v", invocation.Name, anyChanged)

	return result, nil
}

// DryRun logs what the preset would expand.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	invocation := step.Preset
	paramCount := 0
	if invocation.With != nil {
		paramCount = len(invocation.With)
	}

	ec.Logger.Infof("  [DRY-RUN] Would expand preset '%s' (parameters: %d)", invocation.Name, paramCount)

	// Optionally, we could expand and show the steps in dry-run mode
	// For now, just showing the preset name and parameter count
	return nil
}
