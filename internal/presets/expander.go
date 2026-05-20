package presets

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
)

// ExpandPreset expands a preset invocation into its constituent steps.
// It loads the preset definition, validates parameters, and returns the expanded steps
// with the 'parameters' namespace injected into the execution context, along with the
// preset's base directory for relative path resolution.
func ExpandPreset(invocation *config.PresetInvocation) ([]config.Step, map[string]interface{}, string, error) {
	if invocation == nil {
		return nil, nil, "", fmt.Errorf("preset invocation is nil")
	}

	definition, err := LoadPreset(invocation.Name)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load preset '%s': %w", invocation.Name, err)
	}
	return expandLoaded(invocation, definition)
}

// ExpandPresetFromPath is the spec-67 entry point for `use: ./foo.yml` style
// invocations. The caller has already resolved the path to an absolute
// location; this function loads the definition, validates props, and returns
// the expanded steps along with the namespace and base directory.
func ExpandPresetFromPath(invocation *config.PresetInvocation, absPath string) ([]config.Step, map[string]interface{}, string, error) {
	if invocation == nil {
		return nil, nil, "", fmt.Errorf("component invocation is nil")
	}
	definition, err := LoadPresetFromPath(absPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load component '%s': %w", absPath, err)
	}
	return expandLoaded(invocation, definition)
}

// expandLoaded runs the shared post-load steps (validate props, inject
// namespaces, clone steps). Extracted so ExpandPreset and ExpandPresetFromPath
// share one implementation.
func expandLoaded(invocation *config.PresetInvocation, definition *config.PresetDefinition) ([]config.Step, map[string]interface{}, string, error) {

	// Validate and prepare parameters
	userParams := invocation.With
	if userParams == nil {
		userParams = make(map[string]interface{})
	}

	validatedParams, err := ValidateParameters(definition, userParams)
	if err != nil {
		return nil, nil, "", fmt.Errorf("preset '%s' parameter validation failed: %w", invocation.Name, err)
	}

	// Inject both `parameters` (legacy) and `props` (spec-67) namespaces so
	// step expressions can reference either name. They alias the same map;
	// callers that mutate one observe the change in the other.
	parametersNamespace := map[string]interface{}{
		"parameters": validatedParams,
		"props":      validatedParams,
	}

	// Clone steps from preset definition
	// We don't need to modify the steps here - the executor will handle
	// template rendering with the parameters namespace injected
	expandedSteps := make([]config.Step, len(definition.Steps))
	for i, step := range definition.Steps {
		// Create a shallow clone of the step
		expandedSteps[i] = *step.Clone()
	}

	return expandedSteps, parametersNamespace, definition.BaseDir, nil
}
