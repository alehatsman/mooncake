package executor

import (
	"errors"
	"os"
	"runtime"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/utils"
	"github.com/fatih/color"
)

func check(e error) {
	if e != nil {
		logger.Errorf("Error: %s", e)
		panic(e)
	}
}

func addGlobalVariables(variables map[string]interface{}) {
	variables["os"] = runtime.GOOS
	variables["arch"] = runtime.GOARCH
}

func handleVars(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling vars: %+v", step.Vars)

	vars := step.Vars

	for k, v := range *vars {
		logger.Infof("  %v: %v", k, v)
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

	whenExpression, err := utils.Render(whenString, ec.Variables)
	if err != nil {
		return false, err
	}

	ec.Logger.Debugf("whenExpression: %v", whenExpression)

	evalResult, err := utils.Evaluate(whenExpression, ec.Variables)

	ec.Logger.Debugf("evalResult: %v", evalResult)

	return !evalResult.(bool), err
}

func handleInclude(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Expanding path: %v in %v with context: %v", step.Include, ec.CurrentDir, ec.Variables)

	renderedPath, err := utils.ExpandPath(*step.Include, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	includeSteps, err := config.ReadConfig(renderedPath)
	if err != nil {
		return err
	}

	newCurrentDir := utils.GetDirectoryOfFile(renderedPath)

	newExecutionContext := ec.Copy()
	newExecutionContext.CurrentDir = newCurrentDir
	newExecutionContext.Level = ec.Level + 1
	newExecutionContext.Logger = logger.WithPadLevel(ec.Level + 1)

	ExecuteSteps(includeSteps, &newExecutionContext)
	return nil
}

func ExecuteStep(step config.Step, ec *ExecutionContext) error {
	err := step.Validate()
	check(err)

	shouldSkip := false
	if step.When != "" {
		shouldSkip, err = handleWhenExpression(step, ec)
		check(err)
	}

	if step.Include == nil {
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		message := tag + " " + step.Name
		if step.When != "" {
			message += " when: " + step.When
			shouldSkip, err := handleWhenExpression(step, ec)
			check(err)
			if shouldSkip {
				message += " (skipped)"
			}
		}
		ec.Logger.Infof(message)
	}

	if shouldSkip {
		return nil
	}

	switch {
	case step.IncludeVars != nil:
		err := HandleIncludeVars(step, ec)
		check(err)

	case step.Vars != nil:
		err := handleVars(step, ec)
		check(err)

	case step.Template != nil:
		err := HandleTemplate(step, ec)
		check(err)

	case step.File != nil:
		err := HandleFile(step, ec)
		check(err)

	case step.Shell != nil:
		err := HandleShell(step, ec)
		check(err)

	case step.Include != nil:
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex, ec.TotalSteps)
		ec.Logger.Infof(tag+" Including: %v", *step.Include)

		err := handleInclude(step, ec)
		check(err)
	}

	return nil
}

func HandleWithFileTree(step config.Step, ec *ExecutionContext) error {
	ec.Logger.Debugf("Handling with_filetree: %+v", step.WithFileTree)

	withFileTree := step.WithFileTree

	ec.Logger.Infof("with_filetree: %v", *withFileTree)

	path, err := utils.ExpandPath(*withFileTree, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	fileTree, err := utils.GetFileTree(path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Debugf("fileTree: %+v", fileTree)

	curEc := ec.Copy()
	curEc.Level += 1
	curEc.Logger = logger.WithPadLevel(curEc.Level)
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

func ExecuteSteps(steps []config.Step, ec *ExecutionContext) {
	ec.Logger.Infof(color.CyanString("[1/%d]", ec.TotalSteps)+" Executing: %v", ec.CurrentFile)

	for i, step := range steps {
		ec.CurrentIndex = i

		if step.WithFileTree != nil {
			err := HandleWithFileTree(step, ec)
			check(err)
			continue
		}

		err := ExecuteStep(step, ec)
		check(err)

		logger.Infof("\n")
	}
}

type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
	SudoPass       string
}

func Start(startConfig StartConfig) error {
	logger.Mooncake()

	logger.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return errors.New("Config file path is empty")
	}

	currentDir, err := os.Getwd()
	check(err)

	expandedPath, err := utils.ExpandPath(startConfig.VarsFilePath, currentDir, nil)
	check(err)

	variables, err := config.ReadVariables(expandedPath)
	if err != nil {
		variables = make(map[string]interface{})
	}

	addGlobalVariables(variables)

	configFilePath, err := utils.ExpandPath(startConfig.ConfigFilePath, currentDir, nil)
	check(err)

	steps, err := config.ReadConfig(configFilePath)
	check(err)

	logger.Debugf("variables: %v", variables)

	executionContext := ExecutionContext{
		Variables:    variables,
		CurrentDir:   currentDir,
		CurrentFile:  configFilePath,
		Level:        0,
		CurrentIndex: 0,
		TotalSteps:   len(steps),
		Logger:       logger.WithPadLevel(0),
		SudoPass:     startConfig.SudoPass,
	}

	ExecuteSteps(steps, &executionContext)
	return nil
}
