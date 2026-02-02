package presets

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
)

// ExpandPreset expands a preset invocation into its constituent steps.
// It loads the preset definition, validates parameters, and returns the expanded steps
// with the 'parameters' namespace injected into the execution context.
func ExpandPreset(invocation *config.PresetInvocation) ([]config.Step, map[string]interface{}, error) {
	if invocation == nil {
		return nil, nil, fmt.Errorf("preset invocation is nil")
	}

	// Load preset definition
	definition, err := LoadPreset(invocation.Name)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load preset '%s': %w", invocation.Name, err)
	}

	// Validate and prepare parameters
	userParams := invocation.With
	if userParams == nil {
		userParams = make(map[string]interface{})
	}

	validatedParams, err := ValidateParameters(definition, userParams)
	if err != nil {
		return nil, nil, fmt.Errorf("preset '%s' parameter validation failed: %w", invocation.Name, err)
	}

	// Create parameters namespace for template expansion
	parametersNamespace := map[string]interface{}{
		"parameters": validatedParams,
	}

	// Clone steps from preset definition
	// We don't need to modify the steps here - the executor will handle
	// template rendering with the parameters namespace injected
	expandedSteps := make([]config.Step, len(definition.Steps))
	for i, step := range definition.Steps {
		// Create a shallow clone of the step
		expandedSteps[i] = *step.Clone()
	}

	return expandedSteps, parametersNamespace, nil
}
