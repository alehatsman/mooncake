// Package include implements the include action handler.
// Include loads and executes steps from external YAML files.
package include

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the include action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "include",
		Description:    "Load and execute steps from external YAML file",
		Category:       actions.CategorySystem,
		SupportsDryRun: true,
	}
}

// Validate validates the include action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Include == nil {
		return fmt.Errorf("include action requires include path")
	}
	if *step.Include == "" {
		return fmt.Errorf("include path cannot be empty")
	}
	return nil
}

// Execute executes the include action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	includePath := *step.Include

	// Render the path (may contain variables like {{ env }})
	renderedPath, err := ec.Template.Render(includePath, ec.Variables)
	if err != nil {
		return nil, fmt.Errorf("failed to render include path '%s': %w", includePath, err)
	}

	// Resolve relative path based on current directory
	var fullPath string
	if filepath.IsAbs(renderedPath) {
		fullPath = renderedPath
	} else {
		fullPath = filepath.Join(ec.CurrentDir, renderedPath)
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("include file not found: %s", fullPath)
	}

	// Read file
	data, err := os.ReadFile(fullPath) // #nosec G304 -- path validation done above
	if err != nil {
		return nil, fmt.Errorf("failed to read include file '%s': %w", fullPath, err)
	}

	// Parse YAML - expecting an array of steps
	var includedSteps []config.Step
	if err := yaml.Unmarshal(data, &includedSteps); err != nil {
		return nil, fmt.Errorf("failed to parse include file '%s': %w", fullPath, err)
	}

	// Emit include started event
	ec.EmitEvent(events.EventIncludeStarted, events.IncludeData{
		Path:       fullPath,
		StepsCount: len(includedSteps),
	})

	ec.Logger.Infof("Including %d steps from '%s'", len(includedSteps), renderedPath)

	// Note: We intentionally do NOT change ec.CurrentDir here.
	// The current directory should remain as set by the parent context (e.g., preset base directory).
	// This allows templates and other file references in included files to resolve correctly
	// relative to the preset/config root, not relative to the included file's directory.

	// Execute included steps
	anyChanged := false
	for i, includedStep := range includedSteps {
		ec.Logger.Debugf("Executing included step %d/%d: %s", i+1, len(includedSteps), includedStep.Name)

		if err := executor.ExecuteStep(includedStep, ec); err != nil {
			return nil, fmt.Errorf("include '%s' step %d failed: %w", renderedPath, i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}

	// Create include result
	result := executor.NewResult()
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Included %d steps from '%s'", len(includedSteps), renderedPath)

	// Emit include completed event
	ec.EmitEvent(events.EventIncludeCompleted, events.IncludeData{
		Path:       fullPath,
		StepsCount: len(includedSteps),
		Changed:    anyChanged,
	})

	ec.Logger.Infof("Include '%s' completed: changed=%v", renderedPath, anyChanged)

	return result, nil
}

// DryRun logs what the include would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	includePath := *step.Include

	// Render the path
	renderedPath, err := ec.Template.Render(includePath, ec.Variables)
	if err != nil {
		ec.Logger.Infof("  [DRY-RUN] Would include: %s (path rendering would fail: %v)", includePath, err)
		return nil
	}

	// Resolve relative path
	var fullPath string
	if filepath.IsAbs(renderedPath) {
		fullPath = renderedPath
	} else {
		fullPath = filepath.Join(ec.CurrentDir, renderedPath)
	}

	// Check if file exists
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		ec.Logger.Infof("  [DRY-RUN] Would include: %s (file not found)", renderedPath)
		return nil
	}

	// Try to read and count steps
	data, err := os.ReadFile(fullPath) // #nosec G304 -- dry-run only
	if err != nil {
		ec.Logger.Infof("  [DRY-RUN] Would include: %s (read would fail: %v)", renderedPath, err)
		return nil
	}

	var includedSteps []config.Step
	if err := yaml.Unmarshal(data, &includedSteps); err != nil {
		ec.Logger.Infof("  [DRY-RUN] Would include: %s (parse would fail: %v)", renderedPath, err)
		return nil
	}

	ec.Logger.Infof("  [DRY-RUN] Would include %d steps from '%s'", len(includedSteps), renderedPath)
	return nil
}
