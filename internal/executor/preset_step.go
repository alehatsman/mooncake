package executor

import (
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/presets"
)

// HandlePreset executes a preset by expanding it into steps and executing them.
func HandlePreset(step config.Step, ec *ExecutionContext) error {
	invocation := step.Preset
	if invocation == nil {
		return fmt.Errorf("preset invocation is nil")
	}

	// Create result for the preset
	result := NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		ec.CurrentResult = result
	}()

	// Dry-run mode
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogPresetOperation(invocation, len(step.Preset.With))
		dryRun.LogRegister(step)
	}) {
		// Register result even in dry-run mode
		if step.Register != "" {
			result.RegisterTo(ec.Variables, step.Register)
		}
		return nil
	}

	// Expand preset into steps
	expandedSteps, parametersNamespace, err := presets.ExpandPreset(invocation)
	if err != nil {
		return fmt.Errorf("failed to expand preset '%s': %w", invocation.Name, err)
	}

	// Emit preset expanded event
	ec.EventPublisher.Publish(events.Event{
		Type: events.EventPresetExpanded,
		Data: events.PresetData{
			Name:       invocation.Name,
			Parameters: invocation.With,
			StepsCount: len(expandedSteps),
		},
	})

	ec.Logger.Infof("Expanding preset '%s' into %d steps", invocation.Name, len(expandedSteps))

	// Save current variables and inject parameters namespace
	savedVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		savedVariables[k] = v
	}

	// Merge parameters namespace into variables
	for k, v := range parametersNamespace {
		ec.Variables[k] = v
	}

	// Restore variables after execution
	defer func() {
		// Remove parameters namespace
		for k := range parametersNamespace {
			delete(ec.Variables, k)
		}
		// Restore original variables (in case steps modified them)
		for k, v := range savedVariables {
			ec.Variables[k] = v
		}
	}()

	// Execute expanded steps
	anyChanged := false
	for i, expandedStep := range expandedSteps {
		ec.Logger.Debugf("Executing preset step %d/%d: %s", i+1, len(expandedSteps), expandedStep.Name)

		if err := ExecuteStep(expandedStep, ec); err != nil {
			return fmt.Errorf("preset '%s' step %d failed: %w", invocation.Name, i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}

	// Set preset result
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Preset '%s' executed %d steps", invocation.Name, len(expandedSteps))

	// Emit preset completed event
	ec.EventPublisher.Publish(events.Event{
		Type: events.EventPresetCompleted,
		Data: events.PresetData{
			Name:       invocation.Name,
			Parameters: invocation.With,
			StepsCount: len(expandedSteps),
			Changed:    anyChanged,
		},
	})

	ec.Logger.Infof("Preset '%s' completed: changed=%v", invocation.Name, anyChanged)

	// Register result if requested
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
	}

	return nil
}
