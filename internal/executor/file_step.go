package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
)

const (
	defaultFileMode os.FileMode = 0644
	defaultDirMode  os.FileMode = 0755
)

// formatMode formats a file mode for display in log messages
func formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

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

// createDirectory creates a directory at the specified path.
func createDirectory(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	mode := parseFileMode(file.Mode, defaultDirMode)

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogDirectoryCreate(renderedPath, mode)
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	// Check if directory already exists
	if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
		result.Changed = true
	}

	ec.Logger.Debugf("  Creating directory: %s", renderedPath)
	if err := createDirectoryWithBecome(renderedPath, mode, step, ec); err != nil {
		markStepFailed(result, step, ec)
		return fmt.Errorf("failed to create directory %s: %w", renderedPath, err)
	}

	// Emit directory.created event
	ec.EmitEvent(events.EventDirCreated, events.FileOperationData{
		Path:    renderedPath,
		Mode:    mode.String(),
		Changed: result.Changed,
		DryRun:  ec.DryRun,
	})

	return nil
}

// logDryRunFileOperation logs appropriate dry-run message based on file existence.
func logDryRunFileOperation(dryRun *dryRunLogger, path string, mode os.FileMode, newSize int) {
	// #nosec G304 -- File path from user config is intentional functionality for provisioning
	existingContent, _ := os.ReadFile(path)
	if existingContent != nil {
		dryRun.LogFileUpdate(path, mode, len(existingContent), newSize)
	} else {
		dryRun.LogFileCreate(path, mode, newSize)
	}
}

// logContentPreview logs a preview of content, truncating if necessary.
func logContentPreview(log logger.Logger, content string, maxLen int) {
	if len(content) == 0 {
		return
	}
	preview := content
	if len(content) > maxLen {
		preview = content[:maxLen] + "..."
	}
	log.Debugf("  Content preview:\n%s", preview)
}

// createOrUpdateFile creates a file at the specified path, with optional content.
func createOrUpdateFile(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	mode := parseFileMode(file.Mode, defaultFileMode)

	// Handle empty file
	if file.Content == "" {
		if ec.HandleDryRun(func(dryRun *dryRunLogger) {
			logDryRunFileOperation(dryRun, renderedPath, mode, 0)
			dryRun.LogRegister(step)
		}) {
			return nil
		}

		// Check if file already exists
		if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
			result.Changed = true
		}

		ec.Logger.Debugf("  Creating file: %s", renderedPath)
		fileCreated := result.Changed
		if err := createFileWithBecome(renderedPath, []byte(""), mode, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to create file %s: %w", renderedPath, err)
		}

		// Emit file.created or file.updated event
		eventType := events.EventFileUpdated
		if fileCreated {
			eventType = events.EventFileCreated
		}
		ec.EmitEvent(eventType, events.FileOperationData{
			Path:      renderedPath,
			Mode:      mode.String(),
			SizeBytes: 0,
			Changed:   result.Changed,
			DryRun:    ec.DryRun,
		})

		return nil
	}

	// Handle file with content
	renderedContent, err := ec.Template.Render(file.Content, ec.Variables)
	if err != nil {
		return err
	}

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		logDryRunFileOperation(dryRun, renderedPath, mode, len(renderedContent))
		logContentPreview(ec.Logger, renderedContent, 200)
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	// Check if file content would change
	// #nosec G304 -- File path from user config is intentional functionality for provisioning
	existingContent, err := os.ReadFile(renderedPath)
	if err != nil || string(existingContent) != renderedContent {
		result.Changed = true
	}

	ec.Logger.Debugf("  Creating file: %s", renderedPath)
	fileCreated := result.Changed
	if err := createFileWithBecome(renderedPath, []byte(renderedContent), mode, step, ec); err != nil {
		markStepFailed(result, step, ec)
		return fmt.Errorf("failed to write file %s: %w", renderedPath, err)
	}

	// Emit file.created or file.updated event
	eventType := events.EventFileUpdated
	if fileCreated {
		eventType = events.EventFileCreated
	}
	ec.EmitEvent(eventType, events.FileOperationData{
		Path:      renderedPath,
		Mode:      mode.String(),
		SizeBytes: int64(len(renderedContent)),
		Changed:   result.Changed,
		DryRun:    ec.DryRun,
	})

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

	// Create result object with start time
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false // Will be set to true if we create/modify

	// Finalize timing when function returns
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Dispatch to appropriate handler based on state
	switch file.State {
	case "directory":
		if err := createDirectory(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case actionTypeFile:
		if err := createOrUpdateFile(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	}

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	return nil
}

// createFileWithBecome creates a file with become support
// Uses temporary file + sudo move pattern when become is true
func createFileWithBecome(path string, content []byte, mode os.FileMode, step config.Step, ec *ExecutionContext) error {
	if !step.Become {
		// Regular file creation
		return os.WriteFile(path, content, mode)
	}

	// Validate platform support
	if !security.IsBecomeSupported() {
		return fmt.Errorf("become is not supported on %s", runtime.GOOS)
	}

	// Become required - use temp file + sudo move pattern
	if ec.SudoPass == "" {
		return fmt.Errorf("step requires sudo but no password provided")
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "mooncake-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			ec.Logger.Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
		}
	}()

	// Write content to temp file
	if err := os.WriteFile(tmpPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Use sudo to move and set permissions
	return executeSudoFileOperation(tmpPath, path, mode, step, ec)
}

// createDirectoryWithBecome creates a directory with become support
func createDirectoryWithBecome(path string, mode os.FileMode, step config.Step, ec *ExecutionContext) error {
	if !step.Become {
		// Regular directory creation
		return os.MkdirAll(path, mode)
	}

	// Validate platform support
	if !security.IsBecomeSupported() {
		return fmt.Errorf("become is not supported on %s", runtime.GOOS)
	}

	// Become required - use sudo
	if ec.SudoPass == "" {
		return fmt.Errorf("step requires sudo but no password provided")
	}

	// Build sudo mkdir command
	cmd := fmt.Sprintf("mkdir -p %q && chmod %#o %q", path, mode, path)

	if step.BecomeUser != "" {
		cmd = fmt.Sprintf("%s && chown %q %q", cmd, step.BecomeUser, path)
	}

	return executeSudoCommand(cmd, step, ec)
}

// executeSudoFileOperation moves temp file to destination with sudo
func executeSudoFileOperation(tmpPath, destPath string, mode os.FileMode, step config.Step, ec *ExecutionContext) error {
	// Build command: mv tmpPath destPath && chmod mode destPath
	cmd := fmt.Sprintf("mv %q %q && chmod %#o %q", tmpPath, destPath, mode, destPath)

	if step.BecomeUser != "" {
		cmd = fmt.Sprintf("%s && chown %q %q", cmd, step.BecomeUser, destPath)
	}

	return executeSudoCommand(cmd, step, ec)
}

// executeSudoCommand executes a command with sudo
func executeSudoCommand(command string, step config.Step, ec *ExecutionContext) error {
	// Build sudo arguments
	args := []string{"-S"}
	if step.BecomeUser != "" {
		args = append(args, "-u", step.BecomeUser)
	}
	args = append(args, "--", "bash", "-c", command)

	// #nosec G204 - This is a provisioning tool designed to execute commands.
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = bytes.NewBuffer([]byte(ec.SudoPass + "\n"))

	// Capture output for error reporting
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo command failed: %w\nOutput: %s", err, ec.Redactor.Redact(string(output)))
	}

	return nil
}
