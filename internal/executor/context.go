package executor

import (
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

type ExecutionContext struct {
	Variables    map[string]interface{}
	CurrentDir   string
	CurrentFile  string
	Level        int
	CurrentIndex int
	TotalSteps   int
	Logger       logger.Logger
	SudoPass     string
	Tags         []string

	// Injected dependencies
	Template   template.Renderer
	Evaluator  expression.Evaluator
	PathUtil   *pathutil.PathExpander
	FileTree   *filetree.Walker
}

func (ec *ExecutionContext) Copy() ExecutionContext {
	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	return ExecutionContext{
		Variables:    newVariables,
		CurrentDir:   ec.CurrentDir,
		CurrentFile:  ec.CurrentFile,
		Level:        ec.Level,
		CurrentIndex: ec.CurrentIndex,
		TotalSteps:   ec.TotalSteps,
		Logger:       ec.Logger,
		SudoPass:     ec.SudoPass,
		Tags:         ec.Tags,

		// Share the same dependency instances
		Template:  ec.Template,
		Evaluator: ec.Evaluator,
		PathUtil:  ec.PathUtil,
		FileTree:  ec.FileTree,
	}
}
