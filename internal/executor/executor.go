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

	whenExpression, err := utils.Render(whenString, ec.Variables)
	if err != nil {
		return false, err
	}

	evalResult, err := utils.Evaluate(whenExpression, ec.Variables)
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

func ExecuteSteps(steps []config.Step, ec *ExecutionContext) {
	ec.Logger.Infof(color.CyanString("[0/%d]", len(steps))+" Executing: %v", ec.CurrentFile)

	for i, step := range steps {
		err := step.Validate()
		check(err)

		shouldSkip := false
		if step.When != "" {
			shouldSkip, err = handleWhenExpression(step, ec)
			check(err)
		}

		if step.Include == nil {
			tag := color.CyanString("[%d/%d]", i+1, len(steps))
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
			continue
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
			tag := color.CyanString("[%d/%d]", i, len(steps))
			ec.Logger.Infof(tag+" Including: %v", *step.Include)

			err := handleInclude(step, ec)
			check(err)
		}

		logger.Infof("\n")
	}
}

type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
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
		Variables:   variables,
		CurrentDir:  currentDir,
		CurrentFile: configFilePath,
		Level:       0,
		Logger:      logger.WithPadLevel(0),
	}

	ExecuteSteps(steps, &executionContext)
	return nil
}
