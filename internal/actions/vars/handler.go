// Package vars implements the vars action handler.
//
// The vars action sets variables that are available to subsequent steps.
// Variables can be used in templates and when conditions.
package vars

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Handler implements the Handler interface for vars actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the vars action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "vars",
		Description:        "Set variables for use in subsequent steps",
		Category:           actions.CategoryData,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventVarsSet)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate checks if the vars configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Vars == nil {
		return fmt.Errorf("vars configuration is nil")
	}

	return nil
}

// Execute runs the vars action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	vars := step.Vars
	if vars == nil {
		return nil, fmt.Errorf("vars is nil")
	}

	logger := ctx.GetLogger()
	logger.Debugf("Handling vars: %+v", vars)

	for k, v := range *vars {
		logger.Debugf("  %v: %v", k, v)
	}

	// Merge variables into context
	// Note: We need to modify the context's variables directly
	// The Context interface doesn't provide a SetVariables method
	// so we access it through the underlying ExecutionContext
	variables := ctx.GetVariables()
	mergedVars := utils.MergeVariables(variables, *vars)

	// Update the variables map in place
	for k, v := range mergedVars {
		variables[k] = v
	}

	// Emit variables.set event
	keys := make([]string, 0, len(*vars))
	for k := range *vars {
		keys = append(keys, k)
	}

	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventVarsSet,
			Data: events.VarsSetData{
				Count:  len(*vars),
				Keys:   keys,
				DryRun: ctx.IsDryRun(),
			},
		})
	}

	// Create result
	result := executor.NewResult()
	result.Changed = false // Setting variables doesn't count as "changed"

	return result, nil
}

// DryRun logs what variables would be set.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	vars := step.Vars
	if vars == nil {
		return fmt.Errorf("vars is nil")
	}

	// Log what would be set
	ctx.GetLogger().Infof("  [DRY-RUN] Would set %d variables", len(*vars))

	// Still set variables in dry-run mode so subsequent steps can use them
	variables := ctx.GetVariables()
	mergedVars := utils.MergeVariables(variables, *vars)
	for k, v := range mergedVars {
		variables[k] = v
	}

	return nil
}
