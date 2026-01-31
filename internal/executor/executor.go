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

	ec.Logger.Debugf("Reading configuration from file: %v", renderedPath)
	includeSteps, err := config.ReadConfig(renderedPath)
	if err != nil {
		return err
	}
	ec.Logger.Debugf("Read configuration with %v steps", len(includeSteps))

	newCurrentDir := utils.GetDirectoryOfFile(renderedPath)

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
	var err error
	if step.When != "" {
		shouldSkip, err = handleWhenExpression(step, ec)
		if err != nil {
			return err
		}
	}

	if step.Include == nil {
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		message := tag + " " + step.Name
		if step.When != "" {
			message += " when: " + step.When
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
}

func Start(startConfig StartConfig, log logger.Logger) error {
	log.Mooncake()

	log.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return errors.New("config file path is empty")
	}

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	expandedPath, err := utils.ExpandPath(startConfig.VarsFilePath, currentDir, nil)
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

	configFilePath, err := utils.ExpandPath(startConfig.ConfigFilePath, currentDir, nil)
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
	}

	return ExecuteSteps(steps, &executionContext)
}
