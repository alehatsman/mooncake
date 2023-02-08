package main

import (
	"bufio"
	"bytes"
	"fmt"
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
	Name     string    `yaml:"name"`
	When     string    `yaml:"when"`
	Template *Template `yaml:"template"`
	File     *File     `yaml:"file"`
	Shell    *string   `yaml:"shell"`
	Include  string    `yaml:"include"`
	Become   bool      `yaml:"become"`
}

func readConfig(path string) ([]Step, error) {
	fmt.Println("Config file: ", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opened file: ", f.Name())

	r := bufio.NewReader(f)
	config := make([]Step, 0)

	decoder := yaml.NewDecoder(r)
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

func executeSteps(steps []Step, currentDir string, variables Context) {
	fmt.Println("Executing steps, currentDir: ", currentDir)

	for _, step := range steps {
		if step.When != "" {
			expressionString, err := renderTemplate(step.When, variables)
			check(err)

			evalResult, err := evaluateExpression(expressionString, variables)

			if evalResult == false {
				fmt.Println("Skipping step: ", step.Name)
				continue
			}
		}

		switch {
		case step.Template != nil:
			template := step.Template

			src, err := expandPath(template.Src, currentDir, variables)
			check(err)

			dest, err := expandPath(template.Dest, currentDir, variables)
			check(err)

			fmt.Println("Rendering template: ", src, dest)

			templateFile, err := os.Open(src)
			check(err)

			templateBytes, err := ioutil.ReadAll(templateFile)
			check(err)

			output, err := renderTemplate(string(templateBytes), variables)
			check(err)

			err = ioutil.WriteFile(dest, []byte(output), 0644)

		case step.File != nil:
			file := step.File

			if file.State == "directory" {
				renderedPath, err := expandPath(file.Path, currentDir, variables)
				check(err)

				file.Path = renderedPath

				fmt.Println("Creating directory: ", file.Path)

				if file.Path == "" {
					fmt.Println("Skipping empty directory")
					continue
				}

				os.MkdirAll(file.Path, 0755)
			}

		case step.Shell != nil:
			shell := step.Shell

			renderedCommand, err := renderTemplate(*shell, variables)
			check(err)

			var command *exec.Cmd

			if step.Become {
				command = exec.Command("sudo", "bash")
				command.Stdin = bytes.NewBuffer([]byte(renderedCommand))
			} else {
				command = exec.Command("bash", "-c", renderedCommand)
			}

			stdout, err := command.StdoutPipe()
			check(err)

			if err := command.Start(); err != nil {
				log.Fatal(err)
			}

			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				m := scanner.Text()
				fmt.Println(m)
			}

			if err := scanner.Err(); err != nil {
				log.Fatal(err)
			}

			if err := command.Wait(); err != nil {
				log.Fatal(err)
			}

		case step.Include != "":
			renderedPath, err := expandPath(step.Include, currentDir, variables)
			check(err)

			fmt.Println("Including file: ", renderedPath)

			includeSteps, err := readConfig(renderedPath)
			check(err)

			currentDir := getDirectoryOfFile(renderedPath)

			executeSteps(includeSteps, currentDir, variables)
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

	executeSteps(steps, currentDir, variables)
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
