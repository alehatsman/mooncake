package executor

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/fatih/color"
)


func addGlobalVariables(variables map[string]interface{}) {
	variables["os"] = runtime.GOOS
	variables["arch"] = runtime.GOARCH
}

func handleVars(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling vars: %+v", step.Vars)

	vars := step.Vars

	for k, v := range *vars {
		ec.Logger.Infof("  %v: %v", k, v)
	}

	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	for k, v := range *vars {
		newVariables[k] = v
	}

	ec.Variables = newVariables
	return nil
}

func handleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error) {
	whenString := strings.Trim(step.When, " ")

	ec.Logger.Debugf("variables: %v", ec.Variables)

	whenExpression, err := ec.Template.Render(whenString, ec.Variables)
	if err != nil {
		return false, err
	}

	ec.Logger.Debugf("whenExpression: %v", whenExpression)

	evalResult, err := ec.Evaluator.Evaluate(whenExpression, ec.Variables)

	ec.Logger.Debugf("evalResult: %v", evalResult)

	return !evalResult.(bool), err
}

func shouldSkipByTags(step config.Step, ec *ExecutionContext) bool {
	// If no tags filter specified, execute all steps
	if len(ec.Tags) == 0 {
		return false
	}

	// If step has no tags and tags filter is specified, skip it
	if len(step.Tags) == 0 {
		return true
	}

	// Check if step has any of the requested tags
	for _, stepTag := range step.Tags {
		for _, filterTag := range ec.Tags {
			if stepTag == filterTag {
				return false // Found a match, don't skip
			}
		}
	}

	// No matching tags found, skip the step
	return true
}

func handleInclude(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Expanding path: %v in %v with context: %v", step.Include, ec.CurrentDir, ec.Variables)

	renderedPath, err := ec.PathUtil.ExpandPath(*step.Include, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Debugf("Reading configuration from file: %v", renderedPath)
	includeSteps, err := config.ReadConfig(renderedPath)
	if err != nil {
		return err
	}
	ec.Logger.Debugf("Read configuration with %v steps", len(includeSteps))

	newCurrentDir := pathutil.GetDirectoryOfFile(renderedPath)

	newExecutionContext := ec.Copy()
	newExecutionContext.CurrentDir = newCurrentDir
	newExecutionContext.Level = ec.Level + 1
	newExecutionContext.Logger = ec.Logger.WithPadLevel(ec.Level + 1)

	return ExecuteSteps(includeSteps, &newExecutionContext)
}

func ExecuteStep(step config.Step, ec *ExecutionContext) error {
	if err := step.Validate(); err != nil {
		return err
	}

	shouldSkip := false
	skipReason := ""
	var err error

	// Check when condition
	if step.When != "" {
		shouldSkip, err = handleWhenExpression(step, ec)
		if err != nil {
			return err
		}
		if shouldSkip {
			skipReason = "when"
		}
	}

	// Check tags filter
	if !shouldSkip && shouldSkipByTags(step, ec) {
		shouldSkip = true
		skipReason = "tags"
	}

	if step.Include == nil {
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		message := tag + " " + step.Name
		if step.When != "" {
			message += " when: " + step.When
		}
		if len(step.Tags) > 0 {
			message += " tags: [" + strings.Join(step.Tags, ", ") + "]"
		}
		if shouldSkip {
			message += " (skipped by " + skipReason + ")"
		}
		ec.Logger.Infof(message)
	}

	if shouldSkip {
		return nil
	}

	switch {
	case step.IncludeVars != nil:
		if err := HandleIncludeVars(step, ec); err != nil {
			return err
		}

	case step.Vars != nil:
		if err := handleVars(step, ec); err != nil {
			return err
		}

	case step.Template != nil:
		if err := HandleTemplate(step, ec); err != nil {
			return err
		}

	case step.File != nil:
		if err := HandleFile(step, ec); err != nil {
			return err
		}

	case step.Shell != nil:
		if err := HandleShell(step, ec); err != nil {
			return err
		}

	case step.Include != nil:
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		ec.Logger.Infof(tag+" Including: %v", *step.Include)

		if err := handleInclude(step, ec); err != nil {
			return err
		}
	}

	return nil
}

func HandleWithFileTree(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling with_filetree: %+v", step.WithFileTree)

	withFileTree := step.WithFileTree

	ec.Logger.Infof("with_filetree: %v", *withFileTree)

	path, err := ec.PathUtil.ExpandPath(*withFileTree, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	fileTree, err := ec.FileTree.GetFileTree(path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Debugf("fileTree: %+v", fileTree)

	curEc := ec.Copy()
	curEc.Level += 1
	curEc.Logger = ec.Logger.WithPadLevel(curEc.Level)
	curEc.TotalSteps = len(fileTree)

	for i, item := range fileTree {
		curEc = curEc.Copy()
		curEc.CurrentIndex = i
		curEc.Variables["item"] = item

		err := ExecuteStep(step, &curEc)
		if err != nil {
			return err
		}
	}

	return nil
}

func ExecuteSteps(steps []config.Step, ec *ExecutionContext) error {
	ec.Logger.Infof(color.CyanString("[1/%d]", ec.TotalSteps)+" Executing: %v", ec.CurrentFile)

	for i, step := range steps {
		ec.CurrentIndex = i

		if step.WithFileTree != nil {
			if err := HandleWithFileTree(step, ec); err != nil {
				return err
			}
			continue
		}

		if err := ExecuteStep(step, ec); err != nil {
			return err
		}

		ec.Logger.Infof("\n")
	}
	return nil
}

type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
	SudoPass       string
	Tags           []string
}

func Start(startConfig StartConfig, log logger.Logger) error {
	log.Mooncake()

	log.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return errors.New("config file path is empty")
	}

	// Create dependencies
	renderer := template.NewPongo2Renderer()
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	expandedPath, err := pathExpander.ExpandPath(startConfig.VarsFilePath, currentDir, nil)
	if err != nil {
		return err
	}

	log.Debugf("Reading variables from file: %v", expandedPath)
	variables, err := config.ReadVariables(expandedPath)
	if err != nil {
		variables = make(map[string]interface{})
	}
	log.Debugf("Read variables: %v", variables)

	addGlobalVariables(variables)

	configFilePath, err := pathExpander.ExpandPath(startConfig.ConfigFilePath, currentDir, nil)
	if err != nil {
		return err
	}

	log.Debugf("Reading configuration from file: %v", configFilePath)
	steps, err := config.ReadConfig(configFilePath)
	if err != nil {
		return err
	}
	log.Debugf("Read configuration with %v steps", len(steps))

	executionContext := ExecutionContext{
		Variables:    variables,
		CurrentDir:   currentDir,
		CurrentFile:  configFilePath,
		Level:        0,
		CurrentIndex: 0,
		TotalSteps:   len(steps),
		Logger:       log.WithPadLevel(0),
		SudoPass:     startConfig.SudoPass,
		Tags:         startConfig.Tags,

		// Inject dependencies
		Template:  renderer,
		Evaluator: evaluator,
		PathUtil:  pathExpander,
		FileTree:  fileTreeWalker,
	}

	return ExecuteSteps(steps, &executionContext)
}
