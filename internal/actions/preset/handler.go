// Package preset implements the preset action handler.
// Presets expand into multiple steps with parameter injection.
package preset

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/presets"
)

// Handler implements the preset action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// savedContext captures the current execution context state for restoration.
type savedContext struct {
	variables     map[string]interface{}
	currentDir    string
	presetBaseDir string
}

// captureContext saves the current execution context state.
func captureContext(ec *executor.ExecutionContext) *savedContext {
	saved := &savedContext{
		variables:     make(map[string]interface{}),
		currentDir:    ec.CurrentDir,
		presetBaseDir: ec.PresetBaseDir,
	}
	for k, v := range ec.Variables {
		saved.variables[k] = v
	}
	return saved
}

// restoreContext restores the execution context to the saved state,
// removing any keys added during preset execution.
func (s *savedContext) restore(ec *executor.ExecutionContext, parametersNamespace map[string]interface{}) {
	// Remove parameters namespace
	for k := range parametersNamespace {
		delete(ec.Variables, k)
	}
	// Restore original variables (in case steps modified them)
	ec.Variables = make(map[string]interface{})
	for k, v := range s.variables {
		ec.Variables[k] = v
	}
	// Restore original directories
	ec.CurrentDir = s.currentDir
	ec.PresetBaseDir = s.presetBaseDir
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
	saved := captureContext(ec)
	defer saved.restore(ec, parametersNamespace)

	// Merge parameters namespace into variables
	for k, v := range parametersNamespace {
		ec.Variables[k] = v
	}

	// Set PresetBaseDir to preset base directory for template path resolution
	// This persists across included task files, unlike CurrentDir which changes per file
	if presetBaseDir != "" {
		ec.PresetBaseDir = presetBaseDir
		ec.CurrentDir = presetBaseDir
	}

	// Use planner to expand includes, loops, and other plan-time directives
	// This ensures includes within preset steps are properly expanded
	planner := plan.NewPlanner()
	fullyExpandedSteps, err := planner.ExpandStepsWithContext(expandedSteps, ec.Variables, presetBaseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand preset steps: %w", err)
	}

	ec.Logger.Infof("Preset '%s' expanded to %d steps (after include expansion)", invocation.Name, len(fullyExpandedSteps))

	// Execute fully expanded steps
	anyChanged := false
	for i, expandedStep := range fullyExpandedSteps {
		ec.Logger.Debugf("Executing preset step %d/%d: %s", i+1, len(fullyExpandedSteps), expandedStep.Name)

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
	result.Stdout = fmt.Sprintf("Preset '%s' executed %d steps", invocation.Name, len(fullyExpandedSteps))

	// Emit preset completed event
	ec.EmitEvent(events.EventPresetCompleted, events.PresetData{
		Name:       invocation.Name,
		Parameters: invocation.With,
		StepsCount: len(fullyExpandedSteps),
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

	// Try to expand preset to show step count
	expandedSteps, parametersNamespace, presetBaseDir, err := presets.ExpandPreset(invocation)
	if err != nil {
		// If expansion fails, show error
		ec.Logger.Infof("  [DRY-RUN] Would expand preset '%s' (expansion failed: %v)", invocation.Name, err)
		return nil
	}

	// Merge parameters for full expansion
	variables := make(map[string]interface{})
	for k, v := range ec.Variables {
		variables[k] = v
	}
	for k, v := range parametersNamespace {
		variables[k] = v
	}

	// Use planner to get final step count
	planner := plan.NewPlanner()
	fullyExpandedSteps, err := planner.ExpandStepsWithContext(expandedSteps, variables, presetBaseDir)
	if err != nil {
		// Show initial count if full expansion fails
		ec.Logger.Infof("  [DRY-RUN] Would expand preset '%s' (%d steps, full expansion failed)",
			invocation.Name, len(expandedSteps))
		return nil
	}

	paramCount := 0
	if invocation.With != nil {
		paramCount = len(invocation.With)
	}

	ec.Logger.Infof("  [DRY-RUN] Would expand preset '%s' (parameters: %d, steps: %d)",
		invocation.Name, paramCount, len(fullyExpandedSteps))

	return nil
}
