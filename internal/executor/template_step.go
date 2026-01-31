package executor

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/utils"
	"github.com/fatih/color"
)

func HandleTemplate(step config.Step, ec *ExecutionContext) error {
	template := step.Template

	src, err := utils.ExpandPath(template.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	dest, err := utils.ExpandPath(template.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	tag := color.New(color.BgMagenta).Sprintf(" tmpl ")
	message := fmt.Sprintf("Rendering template: %s", dest)
	ec.Logger.Infof("%s %s", tag, message)

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

	output, err := utils.Render(string(templateBytes), variables)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dest, []byte(output), 0644)
	if err != nil {
		ec.Logger.Errorf("Error: %s", err)
	}
	return err
}
