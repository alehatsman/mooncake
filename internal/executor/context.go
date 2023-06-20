package executor

import "github.com/alehatsman/mooncake/internal/logger"

type ExecutionContext struct {
	Variables   map[string]interface{}
	CurrentDir  string
	CurrentFile string
	Level       int
	Logger      *logger.Logger
	SudoPass    string
}

func (ec *ExecutionContext) Copy() ExecutionContext {
	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	return ExecutionContext{
		Variables:   newVariables,
		CurrentDir:  ec.CurrentDir,
		CurrentFile: ec.CurrentFile,
		Level:       ec.Level,
		Logger:      ec.Logger,
		SudoPass:    ec.SudoPass,
	}
}
