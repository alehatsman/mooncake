package components

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/config"
)

// ExpandComponent expands a component invocation into its constituent steps.
// It loads the component definition, validates props, and returns the expanded steps
// with the 'props' namespace injected into the execution context, along with the
// component's base directory for relative path resolution.
func ExpandComponent(name string, props map[string]interface{}) ([]config.Step, map[string]interface{}, string, error) {
	if name == "" {
		return nil, nil, "", fmt.Errorf("component invocation has empty name")
	}

	definition, err := LoadComponent(name)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load component '%s': %w", name, err)
	}
	return expandLoaded(name, props, definition)
}

// ExpandComponentFromPath is the spec-67 entry point for `use: ./foo.yml` style
// invocations. The caller has already resolved the path to an absolute
// location; this function loads the definition, validates props, and returns
// the expanded steps along with the namespace and base directory.
func ExpandComponentFromPath(name string, props map[string]interface{}, absPath string) ([]config.Step, map[string]interface{}, string, error) {
	if name == "" {
		return nil, nil, "", fmt.Errorf("component invocation has empty name")
	}
	definition, err := LoadComponentFromPath(absPath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to load component '%s': %w", absPath, err)
	}
	return expandLoaded(name, props, definition)
}

// ExpandLoadedDefinition expands an already-loaded component definition.
// Callers that need the loaded definition before expansion — the alias branch
// of the component handler, which merges module-level default props filtered to
// the component's declared params (#57) — load via LoadComponentFromPath, adjust
// props, then call this instead of ExpandComponentFromPath (which would re-load).
func ExpandLoadedDefinition(name string, props map[string]interface{}, definition *config.ComponentDefinition) ([]config.Step, map[string]interface{}, string, error) {
	if name == "" {
		return nil, nil, "", fmt.Errorf("component invocation has empty name")
	}
	return expandLoaded(name, props, definition)
}

// expandLoaded runs the shared post-load steps (validate props, inject
// namespaces, clone steps). Extracted so ExpandComponent and ExpandComponentFromPath
// share one implementation.
func expandLoaded(name string, props map[string]interface{}, definition *config.ComponentDefinition) ([]config.Step, map[string]interface{}, string, error) {

	// Validate and prepare parameters
	userParams := props
	if userParams == nil {
		userParams = make(map[string]interface{})
	}

	validatedParams, err := ValidateProps(definition, userParams)
	if err != nil {
		return nil, nil, "", fmt.Errorf("component '%s' parameter validation failed: %w", name, err)
	}

	// Inject the `props` namespace so step expressions can reference the
	// component's inputs as {{ props.x }}. The legacy `parameters` alias is
	// gone: one name, no aliasing.
	propsNamespace := map[string]interface{}{
		"props": validatedParams,
	}

	// Clone steps from component definition
	// We don't need to modify the steps here - the executor will handle
	// template rendering with the props namespace injected
	expandedSteps := make([]config.Step, len(definition.Steps))
	for i, step := range definition.Steps {
		// Create a shallow clone of the step
		expandedSteps[i] = *step.Clone()
	}

	return expandedSteps, propsNamespace, definition.BaseDir, nil
}
