package executor

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/utils"
	"github.com/fatih/color"
)

func HandleFile(step config.Step, ec *ExecutionContext) error {
	file := step.File

	if file.Path == "" {
		ec.Logger.Infof("Skipping")
		return nil
	}

	renderedPath, err := utils.ExpandPath(file.Path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	if file.State == "directory" {
		tag := color.New(color.BgMagenta).Sprintf(" dir ")
		message := fmt.Sprintf("Creating directory: %s", renderedPath)
		ec.Logger.Infof("%s %s", tag, message)

		os.MkdirAll(renderedPath, 0755)
	}

	if file.State == "file" {
		tag := color.New(color.BgMagenta).Sprintf(" file ")
		message := fmt.Sprintf("Creating file: %s", renderedPath)
		ec.Logger.Infof("%s %s", tag, message)

		if file.Content == "" {
			err := ioutil.WriteFile(renderedPath, []byte(""), 0644)
			return err
		} else {
			renderedContent, err := utils.Render(file.Content, ec.Variables)
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(renderedPath, []byte(renderedContent), 0644)
			return err
		}
	}

	return nil
}
