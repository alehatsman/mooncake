package executor

import (
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/utils"
)

// HandleIncludeVars loads variables from a YAML file into the execution context.
func HandleIncludeVars(step config.Step, ec *ExecutionContext) error {
	includeVars := step.IncludeVars

	expandedPath, err := ec.PathUtil.ExpandPath(*includeVars, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "include_vars path", Cause: err}
	}

	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		return &FileOperationError{Operation: "read", Path: expandedPath, Cause: err}
	}

	ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogVariableLoad(len(vars), expandedPath)
		// Still load variables in dry-run mode so subsequent steps can use them
	})

	ec.Variables = utils.MergeVariables(ec.Variables, vars)

	// Emit variables.loaded event
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	ec.EmitEvent(events.EventVarsLoaded, events.VarsLoadedData{
		FilePath: expandedPath,
		Count:    len(vars),
		Keys:     keys,
		DryRun:   ec.DryRun,
	})

	return nil
}
