package executor

import (
	"fmt"
	"os"
	"strconv"

	"github.com/alehatsman/mooncake/internal/config"
)

// parseFileMode parses a mode string (e.g., "0644") into os.FileMode
// Returns default mode if mode is empty or invalid
func parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	// Parse as octal
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

func HandleFile(step config.Step, ec *ExecutionContext) error {
	file := step.File

	if file.Path == "" {
		ec.Logger.Infof("Skipping")
		return nil
	}

	renderedPath, err := ec.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ec.Variables)
	if err != nil {
		return err
	}

	if file.State == "directory" {
		ec.Logger.Debugf("  Creating directory: %s", renderedPath)

		mode := parseFileMode(file.Mode, 0755)
		if err := os.MkdirAll(renderedPath, mode); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", renderedPath, err)
		}
	}

	if file.State == "file" {
		ec.Logger.Debugf("  Creating file: %s", renderedPath)

		mode := parseFileMode(file.Mode, 0644)

		if file.Content == "" {
			if err := os.WriteFile(renderedPath, []byte(""), mode); err != nil {
				return fmt.Errorf("failed to create file %s: %w", renderedPath, err)
			}
		} else {
			renderedContent, err := ec.Template.Render(file.Content, ec.Variables)
			if err != nil {
				return err
			}

			if err := os.WriteFile(renderedPath, []byte(renderedContent), mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", renderedPath, err)
			}
		}
	}

	return nil
}
