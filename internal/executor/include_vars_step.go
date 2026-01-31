package executor

import (
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/utils"
)

func HandleIncludeVars(step config.Step, ec *ExecutionContext) error {
	includeVars := step.IncludeVars

	expandedPath, err := utils.ExpandPath(*includeVars, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	vars, err := config.ReadVariables(expandedPath)
	if err != nil {
		return err
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
