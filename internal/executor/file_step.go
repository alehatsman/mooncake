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

	// Create result object
	result := NewResult()
	result.Changed = false // Will be set to true if we create/modify

	if file.State == "directory" {
		mode := parseFileMode(file.Mode, 0755)

		if ec.DryRun {
			ec.Logger.Infof("  [DRY-RUN] Would create directory: %s (mode: %04o)", renderedPath, mode)
			if step.Register != "" {
				ec.Logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
			}
			return nil
		}

		// Check if directory already exists
		if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
			result.Changed = true
		}

		ec.Logger.Debugf("  Creating directory: %s", renderedPath)
		if err := os.MkdirAll(renderedPath, mode); err != nil {
			result.Failed = true
			result.Rc = 1
			if step.Register != "" {
				ec.Variables[step.Register] = result.ToMap()
				ec.Variables[step.Register+"_stdout"] = result.Stdout
				ec.Variables[step.Register+"_stderr"] = result.Stderr
				ec.Variables[step.Register+"_rc"] = result.Rc
				ec.Variables[step.Register+"_failed"] = result.Failed
				ec.Variables[step.Register+"_changed"] = result.Changed
			}
			return fmt.Errorf("failed to create directory %s: %w", renderedPath, err)
		}
	}

	if file.State == "file" {
		mode := parseFileMode(file.Mode, 0644)

		if file.Content == "" {
			if ec.DryRun {
				ec.Logger.Infof("  [DRY-RUN] Would create empty file: %s (mode: %04o)", renderedPath, mode)
				if step.Register != "" {
					ec.Logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
				}
				return nil
			}

			// Check if file already exists with same content
			if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
				result.Changed = true
			}

			ec.Logger.Debugf("  Creating file: %s", renderedPath)
			if err := os.WriteFile(renderedPath, []byte(""), mode); err != nil {
				result.Failed = true
				result.Rc = 1
				if step.Register != "" {
					ec.Variables[step.Register] = result.ToMap()
					ec.Variables[step.Register+"_stdout"] = result.Stdout
					ec.Variables[step.Register+"_stderr"] = result.Stderr
					ec.Variables[step.Register+"_rc"] = result.Rc
					ec.Variables[step.Register+"_failed"] = result.Failed
					ec.Variables[step.Register+"_changed"] = result.Changed
				}
				return fmt.Errorf("failed to create file %s: %w", renderedPath, err)
			}
		} else {
			renderedContent, err := ec.Template.Render(file.Content, ec.Variables)
			if err != nil {
				return err
			}

			if ec.DryRun {
				ec.Logger.Infof("  [DRY-RUN] Would create file: %s (mode: %04o)", renderedPath, mode)
				ec.Logger.Debugf("  Content preview (first 100 chars): %.100s", renderedContent)
				if step.Register != "" {
					ec.Logger.Debugf("  [DRY-RUN] Would register result as: %s", step.Register)
				}
				return nil
			}

			// Check if file content would change
			existingContent, err := os.ReadFile(renderedPath)
			if err != nil || string(existingContent) != renderedContent {
				result.Changed = true
			}

			ec.Logger.Debugf("  Creating file: %s", renderedPath)
			if err := os.WriteFile(renderedPath, []byte(renderedContent), mode); err != nil {
				result.Failed = true
				result.Rc = 1
				if step.Register != "" {
					ec.Variables[step.Register] = result.ToMap()
					ec.Variables[step.Register+"_stdout"] = result.Stdout
					ec.Variables[step.Register+"_stderr"] = result.Stderr
					ec.Variables[step.Register+"_rc"] = result.Rc
					ec.Variables[step.Register+"_failed"] = result.Failed
					ec.Variables[step.Register+"_changed"] = result.Changed
				}
				return fmt.Errorf("failed to write file %s: %w", renderedPath, err)
			}
		}
	}

	// Register the result if register is specified
	if step.Register != "" {
		ec.Variables[step.Register] = result.ToMap()
		ec.Variables[step.Register+"_stdout"] = result.Stdout
		ec.Variables[step.Register+"_stderr"] = result.Stderr
		ec.Variables[step.Register+"_rc"] = result.Rc
		ec.Variables[step.Register+"_failed"] = result.Failed
		ec.Variables[step.Register+"_changed"] = result.Changed
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	return nil
}
