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
	DryRun       bool

	// Global progress tracking (shared across all contexts)
	GlobalStepsExecuted *int // Pointer so it's shared across copies

	// Statistics tracking (shared across all contexts)
	StatsExecuted *int // Pointer so it's shared across copies
	StatsSkipped  *int // Pointer so it's shared across copies
	StatsFailed   *int // Pointer so it's shared across copies

	// Injected dependencies
	Template  template.Renderer
	Evaluator expression.Evaluator
	PathUtil  *pathutil.PathExpander
	FileTree  *filetree.Walker
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
		DryRun:       ec.DryRun,

		// Share the same global counter pointer
		GlobalStepsExecuted: ec.GlobalStepsExecuted,

		// Share the same statistics pointers
		StatsExecuted: ec.StatsExecuted,
		StatsSkipped:  ec.StatsSkipped,
		StatsFailed:   ec.StatsFailed,

		// Share the same dependency instances
		Template:  ec.Template,
		Evaluator: ec.Evaluator,
		PathUtil:  ec.PathUtil,
		FileTree:  ec.FileTree,
	}
}
