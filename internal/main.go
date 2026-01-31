package internal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Knetic/govaluate"
	"github.com/fatih/color"
	"github.com/flosch/pongo2/v6"
	"gopkg.in/yaml.v3"
)

func check(e error) {
	if e != nil {
		logger.Errorf("Error: %s", e)
		panic(e)
	}
}

// TODO: add owner, group, mode, etc
type File struct {
	Path    string `yaml:"path"`
	State   string `yaml:"state"`
	Content string `yaml:"content"`
}

// TODO: add owner, group, mode, etc
type Template struct {
	Src  string                  `yaml:"src"`
	Dest string                  `yaml:"dest"`
	Vars *map[string]interface{} `yaml:"vars"`
}

type Shell struct {
	Command string `yaml:"command"`
}

type Step struct {
	Name        string                  `yaml:"name"`
	When        string                  `yaml:"when"`
	Template    *Template               `yaml:"template"`
	File        *File                   `yaml:"file"`
	Shell       *string                 `yaml:"shell"`
	Include     *string                 `yaml:"include"`
	IncludeVars *string                 `yaml:"include_vars"`
	Become      bool                    `yaml:"become"`
	Vars        *map[string]interface{} `yaml:"vars"`
	Tags        []string                `yaml:"tags"`
}

func (s *Step) ValidateOneAction() error {
	actionsCount := 0
	if s.Template != nil {
		actionsCount++
	}
	if s.File != nil {
		actionsCount++
	}
	if s.Shell != nil {
		actionsCount++
	}
	if s.Include != nil {
		actionsCount++
	}
	if s.IncludeVars != nil {
		actionsCount++
	}
	if s.Vars != nil {
		actionsCount++
	}

	if actionsCount > 1 {
		return errors.New(fmt.Sprintf("Step %s has more than one action", s.Name))
	}

	return nil
}

func (s *Step) ValidateHasAction() error {
	if s.Template == nil && s.File == nil && s.Shell == nil &&
		s.Include == nil && s.IncludeVars == nil && s.Vars == nil {
		return fmt.Errorf("Step %s has no action", s.Name)
	}
	return nil
}

func (s *Step) Validate() error {
	err := s.ValidateHasAction()
	if err != nil {
		return err
	}

	err = s.ValidateOneAction()
	if err != nil {
		return err
	}

	return nil
}

func readConfig(path string) ([]Step, error) {
	logger.Debugf("Reading configuration from file: %v", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	config := make([]Step, 0)

	decoder := yaml.NewDecoder(f)

	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	logger.Debugf("Read configuration with %v steps", len(config))

	return config, nil
}

type Context = map[string]interface{}

func addGlobalVariables(variables Context) {
	variables["os"] = runtime.GOOS
	variables["arch"] = runtime.GOARCH
}

func readVariables(path string) (Context, error) {
	if path == "" {
		return make(Context), nil
	}

	logger.Debugf("Reading variables from file: %v", path)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)

	variables := make(map[string]interface{})

	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(&variables)
	if err != nil {
		return nil, err
	}

	logger.Debugf("Read variables: %v", variables)

	return variables, nil
}

func renderTemplate(template string, variables Context) (string, error) {
	pongoTemplate, err := pongo2.FromString(template)

	if err != nil {
		return "", err
	}

	output, err := pongoTemplate.Execute(variables)

	if err != nil {
		return "", err
	}

	return output, nil
}

func evaluateExpression(expression string, variables Context) (interface{}, error) {
	evaluableExpression, err := govaluate.NewEvaluableExpression(expression)
	if err != nil {
		return nil, err
	}

	evalResult, err := evaluableExpression.Evaluate(variables)
	if err != nil {
		return nil, err
	}

	return evalResult, nil
}

func expandPath(originalPath string, currentDir string, context Context) (string, error) {
	logger.Debugf("Expanding path: %v in %v with context: %v", originalPath, currentDir, context)

	expandedPath, err := renderTemplate(originalPath, context)
	if err != nil {
		return "", nil
	}

	expandedPath = strings.Trim(expandedPath, " ")

	if strings.HasPrefix(expandedPath, ".") {
		expandedPath = path.Join(currentDir, expandedPath[1:])
	}

	if strings.HasPrefix(expandedPath, "~/") {
		home := os.Getenv("HOME")
		expandedPath = home + expandedPath[1:]
	}

	return expandedPath, nil
}

func handleWhenExpression(step Step, ec ExecutionContext) (bool, error) {
	whenString := strings.Trim(step.When, " ")

	whenExpression, err := renderTemplate(whenString, ec.Variables)
	if err != nil {
		return false, err
	}

	evalResult, err := evaluateExpression(whenExpression, ec.Variables)
	return !evalResult.(bool), err
}

func handleIncludeVars(step Step, ec *ExecutionContext) error {
	includeVars := step.IncludeVars

	expandedPath, err := expandPath(*includeVars, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	vars, err := readVariables(expandedPath)
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

func handleVars(step Step, ec *ExecutionContext) error {
	logger.Debugf("Handling vars: %+v", step.Vars)

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

func handleTemplate(step Step, ec ExecutionContext) error {
	template := step.Template

	src, err := expandPath(template.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	dest, err := expandPath(template.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	logger.Infof("  src: %v", src)
	logger.Infof("  dest: %v", dest)
	logger.Infof("  vars: %+v", template.Vars)

	templateFile, err := os.Open(src)
	if err != nil {
		return err
	}

	templateBytes, err := ioutil.ReadAll(templateFile)
	if err != nil {
		return err
	}

	variables := make(map[string]interface{})
	for k, v := range ec.Variables {
		variables[k] = v
	}

	if template.Vars != nil {
		for k, v := range *template.Vars {
			variables[k] = v
		}
	}

	output, err := renderTemplate(string(templateBytes), variables)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, []byte(output), 0644)
	if err != nil {
		logger.Errorf("error: %v", err)
	}
	return err
}

func handleFile(step Step, ec ExecutionContext) error {
	file := step.File

	if file.Path == "" {
		logger.Infof("Skipping empty path")
		return nil
	}

	renderedPath, err := expandPath(file.Path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	if file.State == "directory" {
		logger.Debugf("Creating directory: ", renderedPath)
		os.MkdirAll(renderedPath, 0755)
	}

	if file.State == "file" {
		logger.Debugf("Creating file: ", renderedPath)

		if file.Content == "" {
			err := ioutil.WriteFile(renderedPath, []byte(""), 0644)
			return err
		} else {
			renderedContent, err := renderTemplate(file.Content, ec.Variables)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(renderedPath, []byte(renderedContent), 0644)
			return err
		}
	}

	return nil
}

func printCommandOutputPipe(pipe io.Reader) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		logger.Debugf("  %v", scanner.Text())
	}
}

func handleShell(step Step, ec ExecutionContext) error {
	shell := *step.Shell

	logger.Debugf("Rendering command: %v with vars: %+v", shell, ec.Variables)

	shell = strings.Trim(shell, " ")
	shell = strings.Trim(shell, "\n")

	renderedCommand, err := renderTemplate(shell, ec.Variables)
	if err != nil {
		return err
	}

	logger.Infof("%v", renderedCommand)

	var command *exec.Cmd

	if step.Become {
		command = exec.Command("sudo", "bash")
		command.Stdin = bytes.NewBuffer([]byte(renderedCommand))
	} else {
		command = exec.Command("bash", "-c", renderedCommand)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		log.Fatal(err)
	}

	stdout, err := command.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	if err := command.Start(); err != nil {
		log.Fatal(err)
	}

	go printCommandOutputPipe(stdout)
	go printCommandOutputPipe(stderr)

	if err := command.Wait(); err != nil {
		log.Fatal(err)
	}

	return err
}

func handleInclude(step Step, ec ExecutionContext) error {
	renderedPath, err := expandPath(*step.Include, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	logger.Infof("Including file: %v", renderedPath)

	includeSteps, err := readConfig(renderedPath)
	if err != nil {
		return err
	}

	newCurrentDir := GetDirectoryOfFile(renderedPath)

	newExecutionContext := ec.Copy()
	newExecutionContext.CurrentDir = newCurrentDir

	executeSteps(includeSteps, newExecutionContext)
	return nil
}

type ExecutionContext struct {
	Variables   Context
	CurrentDir  string
	CurrentFile string
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
	}
}

func executeSteps(steps []Step, ec ExecutionContext) {
	color.Green("[0/%d] %v", len(steps), ec.CurrentFile)

	for i, step := range steps {
		err := step.Validate()
		check(err)

		color.Green("[%d/%d] %s\n", i+1, len(steps), step.Name)

		if step.When != "" {
			color.Green("when: %v", step.When)
			shouldSkip, err := handleWhenExpression(step, ec)
			check(err)
			if shouldSkip {
				color.Green("skipping")
				continue
			}
		}

		switch {
		case step.IncludeVars != nil:
			err := handleIncludeVars(step, &ec)
			check(err)

		case step.Vars != nil:
			err := handleVars(step, &ec)
			check(err)

		case step.Template != nil:
			err := handleTemplate(step, ec)
			check(err)

		case step.File != nil:
			err := handleFile(step, ec)
			check(err)

		case step.Shell != nil:
			err := handleShell(step, ec)
			check(err)

		case step.Include != nil:
			err := handleInclude(step, ec)
			check(err)
		}

		logger.Infof("")
	}
}

func GetDirectoryOfFile(path string) string {
	return path[0:strings.LastIndex(path, "/")]
}

type StartConfig struct {
	ConfigFilePath string
	VarsFilePath   string
}

func Start(config StartConfig) error {
	color.Green("Running space fighter...")
	logger.Infof("config: %v", config)

	if config.ConfigFilePath == "" {
		return errors.New("Config file path is empty")
	}

	variables, err := readVariables(config.VarsFilePath)
	if err != nil {
		variables = make(map[string]interface{})
	}

	addGlobalVariables(variables)

	configFilePath, err := filepath.Abs(config.ConfigFilePath)
	check(err)

	steps, err := readConfig(configFilePath)
	check(err)

	currentDir := GetDirectoryOfFile(configFilePath)

	executionContext := ExecutionContext{
		Variables:   variables,
		CurrentDir:  currentDir,
		CurrentFile: configFilePath,
	}

	executeSteps(steps, executionContext)
	return nil
}
