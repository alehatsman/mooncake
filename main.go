package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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

type Step struct {
	Name     string
	When     string
	Template *struct {
		Src  string
		Dest string
	}
	File *struct {
		Path  string
		State string
	}
	Shell string

	Include string
}

func readConfig(path string) ([]Step, error) {
	fmt.Println("Config file: ", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opened file: ", f.Name())

	r := bufio.NewReader(f)
	check(err)

	config := make([]Step, 0)

	decoder := yaml.NewDecoder(r)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

type Context = map[string]interface{}

func readVariables(path string) (Context, error) {
	fmt.Println("Variables file: ", path)

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	fmt.Println("Opened file: ", f.Name())

	r := bufio.NewReader(f)
	if err != nil {
		return nil, err
	}

	variables := make(map[string]interface{})

	decoder := yaml.NewDecoder(r)
	err = decoder.Decode(&variables)

	variables["os"] = runtime.GOOS

	return variables, nil
}

func executeSteps(steps []Step, variables Context) {
	for _, step := range steps {
		if step.When != "" {
			tpl, err := pongo2.FromString(step.When)
			check(err)

			expressionString, err := tpl.Execute(variables)
			check(err)

			expression, err := govaluate.NewEvaluableExpression(expressionString)
			check(err)

			evalResult, err := expression.Evaluate(variables)
			check(err)

			if evalResult == false {
				fmt.Println("Skipping step: ", step.Name)
				continue
			}
		}

		switch {
		case step.Template != nil:
			template := step.Template

			src, err := pongo2.RenderTemplateString(template.Src, variables)
			check(err)

			dest, err := pongo2.RenderTemplateString(template.Dest, variables)

			fmt.Println("Rendering template: ", src, dest)

			templateFile, err := os.Open(src)
			check(err)

			templateBytes, err := ioutil.ReadAll(templateFile)
			check(err)

			tpl, err := pongo2.FromBytes(templateBytes)
			check(err)

			output, err := tpl.Execute(variables)

			err = ioutil.WriteFile(dest, []byte(output), 0644)

		case step.File != nil:
			file := step.File

			if file.State == "directory" {
				renderedPath, err := pongo2.RenderTemplateString(file.Path, variables)
				check(err)

				file.Path = renderedPath
				file.Path = strings.Trim(file.Path, " ")

				fmt.Println("Creating directory: ", file.Path)

				if file.Path == "" {
					fmt.Println("Skipping empty directory")
					continue
				}

				os.MkdirAll(file.Path, 0755)
			}
		case step.Shell != "":
			renderedCommand, err := pongo2.RenderTemplateString(step.Shell, variables)

			fmt.Println("Running shell command: ", renderedCommand)

			command := exec.Command("bash", "-c", renderedCommand)

			output, err := command.Output()
			check(err)

			fmt.Println(string(output))

		case step.Include != "":
			renderedPath, err := pongo2.RenderTemplateString(step.Include, variables)
			check(err)

			fmt.Println("Including file: ", renderedPath)

			includeSteps, err := readConfig(renderedPath)
			check(err)

			executeSteps(includeSteps, variables)
		}
	}
}

// func run(c *cli.Context) error {
// 	bar := progressbar.NewOptions(-1,
// 		progressbar.OptionSetDescription("Fighter provisioning..."),
// 		progressbar.OptionSetPredictTime(false),
// 		progressbar.OptionSpinnerType(76),
// 		progressbar.OptionClearOnFinish(),
// 		progressbar.OptionEnableColorCodes(true),
// 	)
// 	for i := 0; i < 100; i++ {
// 		bar.Add(1)
// 		time.Sleep(40 * time.Millisecond)
// 	}
// 	bar.Finish()
//
// 	fmt.Println("Chookity!")
// 	return nil
// }

func run(c *cli.Context) error {
	configFile := c.String("config")
	variablesFile := c.String("variables")

	variables, err := readVariables(variablesFile)
	check(err)

	steps, err := readConfig(configFile)
	check(err)

	executeSteps(steps, variables)
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
