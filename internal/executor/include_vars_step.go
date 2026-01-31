package executor

import (
	"github.com/alehatsman/mooncake/internal/config"
)

func HandleIncludeVars(step config.Step, ec *ExecutionContext) error {
	includeVars := step.IncludeVars

	expandedPath, err := ec.PathUtil.ExpandPath(*includeVars, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		return err
	}

	if ec.DryRun {
		dryRun := newDryRunLogger(ec.Logger)
		dryRun.LogVariableLoad(len(vars), expandedPath)
		// Still load variables in dry-run mode so subsequent steps can use them
	}

	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	for k, v := range vars {
		newVariables[k] = v
	}

	ec.Variables = newVariables

	return nil
}
