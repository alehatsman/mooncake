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

// Run is the Spec 16 entry point. vars steps only mutate the variable
// scope, not the system. Plan mode reports Checkable=true with
// WouldChange=false; apply mode merges the vars into scope and emits
// EventVarsSet.
//
// Note: the planner already evaluates vars at plan time and strips
// them from the step list, so this handler rarely runs in practice.
// Run is here for completeness and to satisfy the Runner contract.
//
// F011: legacy Execute / DryRun pair folded into Run.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	vars := step.Vars
	if vars == nil {
		return nil, fmt.Errorf("vars is nil")
	}

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.Reason = "vars (no system change)"
		return r, nil
	}

	logger := ctx.GetLogger()
	logger.Debugf("Handling vars: %+v", vars)
	for k, v := range *vars {
		logger.Debugf("  %v: %v", k, v)
	}

	ctx.MergeUserVars(*vars)

	keys := make([]string, 0, len(*vars))
	for k := range *vars {
		keys = append(keys, k)
	}

	if publisher := ctx.GetEventPublisher(); publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventVarsSet,
			Data: events.VarsSetData{
				Count:  len(*vars),
				Keys:   keys,
				DryRun: false,
			},
		})
	}

	result := executor.NewResult()
	result.Operation = executor.OpNoop
	result.Changed = false // Setting variables doesn't count as "changed"
	return result, nil
}
