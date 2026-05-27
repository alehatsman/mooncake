// Package include_vars implements the include_vars action handler.
//
// The include_vars action loads variables from YAML files into the execution context.
// This is useful for organizing variables across multiple files.

//nolint:revive,staticcheck // include_vars name matches action name for consistency
package include_vars

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
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
		Name:               "vars.load",
		Description:        "Load variables from YAML files",
		Category:           actions.CategoryData,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventVarsLoaded)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate checks if the include_vars configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.VarsLoad == nil {
		return fmt.Errorf("include_vars configuration is nil")
	}

	if *step.VarsLoad == "" {
		return fmt.Errorf("include_vars path is empty")
	}

	return nil
}

// Run is the Spec 16 entry point. include_vars only mutates the
// variable scope (by reading a YAML file), not the system. Plan mode
// reports Checkable=true with WouldChange=false; apply mode reads
// the file and merges its keys into the variable scope.
//
// Like vars, the planner usually handles this at plan time; Run is
// here for completeness.
//
// F011: legacy Execute / DryRun pair folded into Run.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.Reason = "include_vars (no system change)"
		return r, nil
	}

	includeVars := step.VarsLoad

	// PathUtil isn't on the actions.Context interface; cast to the
	// concrete ExecutionContext for now.
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	expandedPath, err := ec.Svc.PathUtil.ExpandPath(*includeVars, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read variables from %s: %w", expandedPath, err)
	}

	ctx.MergeUserVars(vars)

	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}

	if publisher := ctx.GetEventPublisher(); publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventVarsLoaded,
			Data: events.VarsLoadedData{
				FilePath: expandedPath,
				Count:    len(vars),
				Keys:     keys,
				DryRun:   false,
			},
		})
	}

	result := executor.NewResult()
	result.Operation = executor.OpNoop
	result.Changed = false
	return result, nil
}
