// Package include_vars implements the include_vars action handler.
//
// The include_vars action loads variables from YAML files into the execution context.
// This is useful for organizing variables across multiple files.
package include_vars

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Handler implements the Handler interface for include_vars actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the include_vars action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "include_vars",
		Description:    "Load variables from YAML files",
		Category:       actions.CategoryData,
		SupportsDryRun: true,
		SupportsBecome: false,
		EmitsEvents:    []string{string(events.EventVarsLoaded)},
		Version:        "1.0.0",
	}
}

// Validate checks if the include_vars configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.IncludeVars == nil {
		return fmt.Errorf("include_vars configuration is nil")
	}

	if *step.IncludeVars == "" {
		return fmt.Errorf("include_vars path is empty")
	}

	return nil
}

// Execute runs the include_vars action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	includeVars := step.IncludeVars

	// We need access to PathUtil which isn't in the Context interface
	// Cast to concrete type for now
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Expand path (handles ~, variables, relative paths)
	expandedPath, err := ec.PathUtil.ExpandPath(*includeVars, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	// Read variables from file
	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables from %s: %w", expandedPath, err)
	}

	// Merge variables into context
	variables := ctx.GetVariables()
	mergedVars := utils.MergeVariables(variables, vars)
	for k, v := range mergedVars {
		variables[k] = v
	}

	// Emit variables.loaded event
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}

	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventVarsLoaded,
			Data: events.VarsLoadedData{
				FilePath: expandedPath,
				Count:    len(vars),
				Keys:     keys,
				DryRun:   ctx.IsDryRun(),
			},
		})
	}

	// Create result
	result := executor.NewResult()
	result.Changed = false // Loading variables doesn't count as "changed"

	return result, nil
}

// DryRun logs what variables would be loaded.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	includeVars := step.IncludeVars

	// Get path (attempt expansion but don't fail)
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	expandedPath, err := ec.PathUtil.ExpandPath(*includeVars, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		expandedPath = *includeVars
	}

	// Try to read file to get count
	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Would load variables from: %s (file not readable in dry-run)", expandedPath)
		return nil
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would load %d variables from: %s", len(vars), expandedPath)

	// Still load variables in dry-run mode so subsequent steps can use them
	variables := ctx.GetVariables()
	mergedVars := utils.MergeVariables(variables, vars)
	for k, v := range mergedVars {
		variables[k] = v
	}

	return nil
}
