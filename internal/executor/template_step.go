package executor

import (
	"fmt"
	"io"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/fatih/color"
)

func HandleTemplate(step config.Step, ec *ExecutionContext) error {
	template := step.Template

	src, err := ec.PathUtil.ExpandPath(template.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	dest, err := ec.PathUtil.ExpandPath(template.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	// Only show detailed templating line when in with_filetree iteration
	if _, inFileTree := ec.Variables["item"]; inFileTree {
		tag := color.CyanString("[%d/%d]", ec.CurrentIndex+1, ec.TotalSteps)
		ec.Logger.Infof("%s templating src=\"%s\" dest=\"%s\"", tag, src, dest)
	}

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

	output, err := ec.Template.Render(string(templateBytes), variables)
	if err != nil {
		return err
	}

	mode := parseFileMode(template.Mode, 0644)
	if err := os.WriteFile(dest, []byte(output), mode); err != nil {
		return fmt.Errorf("failed to write template output to %s: %w", dest, err)
	}

	return nil
}
