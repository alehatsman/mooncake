package executor

import (
	"fmt"
	"io"
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

	ec.Logger.Debugf("src: %s", src)

	dest, err := utils.ExpandPath(template.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	ec.Logger.Debugf("dest: %s", dest)

	tag := color.New(color.BgMagenta).Sprintf(" tmpl ")
	message := fmt.Sprintf("Rendering template: %s", dest)
	ec.Logger.Infof("%s %s", tag, message)

	templateFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer templateFile.Close()

	templateBytes, err := io.ReadAll(templateFile)
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

	mode := parseFileMode(template.Mode, 0644)
	if err := os.WriteFile(dest, []byte(output), mode); err != nil {
		return fmt.Errorf("failed to write template output to %s: %w", dest, err)
	}

	return nil
}
