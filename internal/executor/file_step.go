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

// handleDirectoryState creates a directory at the specified path.
func handleDirectoryState(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	mode := parseFileMode(file.Mode, 0755)

	if ec.DryRun {
		dryRun := newDryRunLogger(ec.Logger)
		dryRun.LogFileOperation("directory", renderedPath, mode)
		dryRun.LogRegister(step)
		return nil
	}

	// Check if directory already exists
	if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
		result.Changed = true
	}

	ec.Logger.Debugf("  Creating directory: %s", renderedPath)
	if err := os.MkdirAll(renderedPath, mode); err != nil {
		markStepFailed(result, step, ec)
		return fmt.Errorf("failed to create directory %s: %w", renderedPath, err)
	}

	return nil
}

// handleFileState creates a file at the specified path, with optional content.
func handleFileState(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	mode := parseFileMode(file.Mode, 0644)

	// Handle empty file
	if file.Content == "" {
		if ec.DryRun {
			dryRun := newDryRunLogger(ec.Logger)
			dryRun.LogFileOperation("file", renderedPath, mode)
			dryRun.LogRegister(step)
			return nil
		}

		// Check if file already exists
		if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
			result.Changed = true
		}

		ec.Logger.Debugf("  Creating file: %s", renderedPath)
		if err := os.WriteFile(renderedPath, []byte(""), mode); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to create file %s: %w", renderedPath, err)
		}

		return nil
	}

	// Handle file with content
	renderedContent, err := ec.Template.Render(file.Content, ec.Variables)
	if err != nil {
		return err
	}

	if ec.DryRun {
		dryRun := newDryRunLogger(ec.Logger)
		dryRun.LogFileOperation("file", renderedPath, mode)
		ec.Logger.Debugf("  Content preview (first 100 chars): %.100s", renderedContent)
		dryRun.LogRegister(step)
		return nil
	}

	// Check if file content would change
	// #nosec G304 -- File path from user config is intentional functionality for provisioning
	existingContent, err := os.ReadFile(renderedPath)
	if err != nil || string(existingContent) != renderedContent {
		result.Changed = true
	}

	ec.Logger.Debugf("  Creating file: %s", renderedPath)
	if err := os.WriteFile(renderedPath, []byte(renderedContent), mode); err != nil {
		markStepFailed(result, step, ec)
		return fmt.Errorf("failed to write file %s: %w", renderedPath, err)
	}

	return nil
}

// HandleFile creates or manages a file or directory step.
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

	// Dispatch to appropriate handler based on state
	switch file.State {
	case "directory":
		if err := handleDirectoryState(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case "file":
		if err := handleFileState(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	}

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	return nil
}
