package main

import (
	"bufio"
	"bytes"
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
	"github.com/flosch/pongo2/v6"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

type File struct {
	Path  string `yaml:"path"`
	State string `yaml:"state"`
}

type Template struct {
	Src  string `yaml:"src"`
	Dest string `yaml:"dest"`
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
		return fmt.Errorf("Step %s has more than one action", s.Name)
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
	fmt.Println("Config file: ", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opened file: ", f.Name())

	config := make([]Step, 0)

	decoder := yaml.NewDecoder(f)

	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

type Context = map[string]interface{}

func addGlobalVariables(variables Context) {
	variables["os"] = runtime.GOOS
}

func readVariables(path string) (Context, error) {
	fmt.Println("Variables file: ", path)

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opened file: ", file.Name())

	reader := bufio.NewReader(file)

	variables := make(map[string]interface{})

	decoder := yaml.NewDecoder(reader)
	err = decoder.Decode(&variables)
	if err != nil {
		return nil, err
	}

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
	vars := step.Vars

	fmt.Println("Adding variables: ", vars)

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

	fmt.Println("Rendering template: ", src, dest)

	templateFile, err := os.Open(src)
	if err != nil {
		return err
	}

	templateBytes, err := ioutil.ReadAll(templateFile)
	if err != nil {
		return err
	}

	output, err := renderTemplate(string(templateBytes), ec.Variables)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, []byte(output), 0644)
	return err
}

func handleFile(step Step, ec ExecutionContext) error {
	file := step.File

	if file.State == "directory" {
		renderedPath, err := expandPath(file.Path, ec.CurrentDir, ec.Variables)
		if err != nil {
			return err
		}

		file.Path = renderedPath

		fmt.Println("Creating directory: ", file.Path)

		if file.Path == "" {
			fmt.Println("Skipping empty path")
			return nil
		}

		os.MkdirAll(file.Path, 0755)
	}

	return nil
}

func printCommandOutputPipe(pipe io.Reader) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Println("Output: ", scanner.Text())
	}
}

func handleShell(step Step, ec ExecutionContext) error {
	shell := step.Shell

	fmt.Println("Rendering command: ", *shell, "with vars: ", ec.Variables)

	renderedCommand, err := renderTemplate(*shell, ec.Variables)
	if err != nil {
		return err
	}

	fmt.Println("Executing shell command: ", renderedCommand)

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

	fmt.Println("Including file: ", renderedPath)

	includeSteps, err := readConfig(renderedPath)
	if err != nil {
		return err
	}

	newCurrentDir := getDirectoryOfFile(renderedPath)

	newExecutionContext := ec.Copy()
	newExecutionContext.CurrentDir = newCurrentDir

	executeSteps(includeSteps, newExecutionContext)
	return nil
}

type ExecutionContext struct {
	Variables  Context
	CurrentDir string
}

func (ec *ExecutionContext) Copy() ExecutionContext {
	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	currentDir := ec.CurrentDir

	return ExecutionContext{
		Variables:  newVariables,
		CurrentDir: currentDir,
	}
}

func executeSteps(steps []Step, ec ExecutionContext) {
	fmt.Println("Executing steps, currentDir: ", ec.CurrentDir)

	for _, step := range steps {
		err := step.Validate()
		check(err)

		fmt.Println("Executing step: ", step.Name)

		if step.When != "" {
			shouldSkip, err := handleWhenExpression(step, ec)
			check(err)
			if shouldSkip {
				fmt.Println("Skipping step: ", step.Name)
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
	}
}

func getDirectoryOfFile(path string) string {
	return path[0:strings.LastIndex(path, "/")]
}

func run(c *cli.Context) error {
	configFilePath := c.String("config")
	variablesFile := c.String("variables")

	variables, err := readVariables(variablesFile)
	if err != nil {
		variables = make(map[string]interface{})
	}

	addGlobalVariables(variables)

	configFilePath, err = filepath.Abs(configFilePath)
	check(err)

	steps, err := readConfig(configFilePath)
	check(err)

	currentDir := getDirectoryOfFile(configFilePath)

	executionContext := ExecutionContext{
		Variables:  variables,
		CurrentDir: currentDir,
	}

	executeSteps(steps, executionContext)
	return nil
}

func main() {
	app := &cli.App{
		Name:                 "mooncake",
		Usage:                "Space fighters provisioning tool, Chookity!",
		EnableBashCompletion: true,
		Action:               run,
		Commands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Run a space fighter",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "config",
						Aliases: []string{"c"},
					},
					&cli.StringFlag{
						Name:    "variables",
						Aliases: []string{"v"},
					},
				},
				Action: run,
			},
			{
				Name:  "watch",
				Usage: "Watch a space fighter",
				Action: func(c *cli.Context) error {
					fmt.Println("Running space fighter...")
					return nil
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
