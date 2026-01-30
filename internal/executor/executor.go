package executor

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)


func addGlobalVariables(variables map[string]interface{}) {
	variables["os"] = runtime.GOOS
	variables["arch"] = runtime.GOARCH
}

func handleVars(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling vars: %+v", step.Vars)

	vars := step.Vars

	for k, v := range *vars {
		ec.Logger.Debugf("  %v: %v", k, v)
	}

	if ec.DryRun {
		ec.Logger.Infof("  [DRY-RUN] Would set %d variables", len(*vars))
		// Still set variables in dry-run mode so subsequent steps can use them
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
	if err != nil {
		return false, err
	}

	ec.Logger.Debugf("evalResult: %v", evalResult)

	// Handle nil or non-bool results
	if evalResult == nil {
		return true, nil // Skip if expression evaluates to nil
	}

	boolResult, ok := evalResult.(bool)
	if !ok {
		return false, fmt.Errorf("when expression did not evaluate to boolean: %v", evalResult)
	}

	return !boolResult, nil
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

	if ec.DryRun {
		ec.Logger.Debugf("  [DRY-RUN] Would include %d steps from: %s", len(includeSteps), renderedPath)
	}

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

	// Determine step name
	var hasStepName bool
	stepName := step.Name

	// For with_filetree, show the actual file name instead of generic step name
	if item, ok := ec.Variables["item"].(filetree.FileTreeItem); ok {
		if item.Name != "" {
			stepName = item.Name
			hasStepName = true
		}
	} else if item, ok := ec.Variables["item"]; ok {
		// For with_items, show the item value
		stepName = fmt.Sprintf("%v", item)
		hasStepName = true
	} else if step.Name != "" {
		hasStepName = true
	}

	// Log skipped steps
	if shouldSkip && step.Include == nil && hasStepName {
		skipInfo := ""
		if skipReason == "when" {
			skipInfo = fmt.Sprintf(" (when: %s)", step.When)
		} else if skipReason == "tags" {
			if len(ec.Tags) > 0 {
				skipInfo = fmt.Sprintf(" (tags filter: %s)", strings.Join(ec.Tags, ", "))
			}
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       stepName + skipInfo,
			Level:      ec.Level,
			GlobalStep: 0,
			Status:     "skipped",
		})

		return nil
	}

	// Debug level: show tags even when not skipped
	if !shouldSkip && len(step.Tags) > 0 {
		ec.Logger.Debugf("  tags: [%s]", strings.Join(step.Tags, ", "))
	}

	if shouldSkip {
		return nil
	}

	// Increment global step counter for non-skipped steps
	if ec.GlobalStepsExecuted != nil {
		*ec.GlobalStepsExecuted++
	}

	// Log running step for non-include steps with names
	// Skip printing for template steps in with_filetree - let the handler print the action instead
	_, inFileTree := ec.Variables["item"].(filetree.FileTreeItem)
	if step.Include == nil && hasStepName && !(step.Template != nil && inFileTree) {
		globalStep := 0
		if ec.GlobalStepsExecuted != nil {
			globalStep = *ec.GlobalStepsExecuted
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       stepName,
			Level:      ec.Level,
			GlobalStep: globalStep,
			Status:     "running",
		})
	}

	// Debug: show what action is being performed for steps without names
	if step.Name == "" {
		if step.Vars != nil {
			ec.Logger.Debugf("Setting variables")
		} else if step.IncludeVars != nil {
			ec.Logger.Debugf("Loading variables from %s", *step.IncludeVars)
		}
	}

	var stepErr error

	switch {
	case step.IncludeVars != nil:
		stepErr = HandleIncludeVars(step, ec)

	case step.Vars != nil:
		stepErr = handleVars(step, ec)

	case step.Template != nil:
		stepErr = HandleTemplate(step, ec)

	case step.File != nil:
		stepErr = HandleFile(step, ec)

	case step.Shell != nil:
		stepErr = HandleShell(step, ec)

	case step.Include != nil:
		globalStep := 0
		if ec.GlobalStepsExecuted != nil {
			globalStep = *ec.GlobalStepsExecuted
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       "Including: " + *step.Include,
			Level:      ec.Level,
			GlobalStep: globalStep,
			Status:     "running",
		})

		stepErr = handleInclude(step, ec)
	}

	// Handle errors
	if stepErr != nil {
		ec.Logger.Errorf("%v", stepErr)

		// Mark step as failed
		if step.Include == nil && hasStepName && !(step.Template != nil && inFileTree) {
			globalStep := 0
			if ec.GlobalStepsExecuted != nil {
				globalStep = *ec.GlobalStepsExecuted
			}

			ec.Logger.LogStep(logger.StepInfo{
				Name:       stepName,
				Level:      ec.Level,
				GlobalStep: globalStep,
				Status:     "error",
			})
		}

		return stepErr
	}

	// Mark step as successful
	if step.Include == nil && hasStepName && !(step.Template != nil && inFileTree) {
		globalStep := 0
		if ec.GlobalStepsExecuted != nil {
			globalStep = *ec.GlobalStepsExecuted
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       stepName,
			Level:      ec.Level,
			GlobalStep: globalStep,
			Status:     "success",
		})
	}

	return nil
}

func HandleWithItems(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling with_items: %+v", step.WithItems)

	withItems := step.WithItems
	ec.Logger.Debugf("with_items: %v", *withItems)

	// Extract variable name from template syntax: "{{ varname }}" -> "varname"
	varName := strings.TrimSpace(*withItems)
	varName = strings.TrimPrefix(varName, "{{")
	varName = strings.TrimSuffix(varName, "}}")
	varName = strings.TrimSpace(varName)

	ec.Logger.Debugf("looking up variable: %s", varName)

	// Look up the variable
	listValue, exists := ec.Variables[varName]
	if !exists {
		return fmt.Errorf("with_items variable not found: %s", varName)
	}

	// Convert to slice
	var list []interface{}
	switch v := listValue.(type) {
	case []interface{}:
		list = v
	case []string:
		list = make([]interface{}, len(v))
		for i, s := range v {
			list[i] = s
		}
	default:
		return fmt.Errorf("with_items value is not a list: %T", listValue)
	}

	ec.Logger.Debugf("list has %d items", len(list))

	// Log the step name once before iterating through list
	if step.Name != "" {
		globalStep := 0
		if ec.GlobalStepsExecuted != nil {
			globalStep = *ec.GlobalStepsExecuted
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       step.Name,
			Level:      ec.Level,
			GlobalStep: globalStep,
			Status:     "running",
		})
	}

	curEc := ec.Copy()
	curEc.Level += 1
	curEc.Logger = ec.Logger.WithPadLevel(curEc.Level)
	curEc.TotalSteps = len(list)

	for i, item := range list {
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

func HandleWithFileTree(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling with_filetree: %+v", step.WithFileTree)

	withFileTree := step.WithFileTree

	ec.Logger.Debugf("with_filetree: %v", *withFileTree)

	path, err := ec.PathUtil.ExpandPath(*withFileTree, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	fileTree, err := ec.FileTree.GetFileTree(path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Debugf("fileTree: %+v", fileTree)

	// Log the step name once before iterating through files
	if step.Name != "" {
		globalStep := 0
		if ec.GlobalStepsExecuted != nil {
			globalStep = *ec.GlobalStepsExecuted
		}

		ec.Logger.LogStep(logger.StepInfo{
			Name:       step.Name,
			Level:      ec.Level,
			GlobalStep: globalStep,
			Status:     "running",
		})
	}

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
	ec.Logger.Debugf("Executing: %v", ec.CurrentFile)

	// Set total steps for this execution context
	ec.TotalSteps = len(steps)

	for i, step := range steps {
		ec.CurrentIndex = i

		if step.WithFileTree != nil {
			if err := HandleWithFileTree(step, ec); err != nil {
				return err
			}
			continue
		}

		if step.WithItems != nil {
			if err := HandleWithItems(step, ec); err != nil {
				return err
			}
			continue
		}

		if err := ExecuteStep(step, ec); err != nil {
			return err
		}
	}
	return nil
}

type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
	SudoPass       string
	Tags           []string
	DryRun         bool
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

	// Initialize global step counter (shared across all contexts via pointer)
	globalExecuted := 0

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
		DryRun:       startConfig.DryRun,

		// Global progress tracking
		GlobalStepsExecuted: &globalExecuted,

		// Inject dependencies
		Template:  renderer,
		Evaluator: evaluator,
		PathUtil:  pathExpander,
		FileTree:  fileTreeWalker,
	}

	return ExecuteSteps(steps, &executionContext)
}
