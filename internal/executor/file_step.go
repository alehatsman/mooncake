package executor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
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

// removeFileOrDirectory removes a file or directory at the specified path.
func removeFileOrDirectory(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	// Safety checks
	if renderedPath == "" || renderedPath == "/" || renderedPath == "C:\\" {
		return fmt.Errorf("refusing to remove empty, root, or system path: %q", renderedPath)
	}

	// Check if path exists
	info, err := os.Stat(renderedPath)
	if os.IsNotExist(err) {
		// Already absent - idempotent, no change
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	// Determine what we're removing
	isDir := info.IsDir()
	size := info.Size()

	// Dry-run
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if isDir {
			dryRun.LogDirectoryRemove(renderedPath)
		} else {
			dryRun.LogFileRemove(renderedPath, size)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	result.Changed = true

	// Remove with become if needed
	if step.Become {
		return removeWithBecome(renderedPath, isDir, file.Force, step, ec)
	}

	// Regular removal
	if isDir {
		if file.Force {
			ec.Logger.Debugf("  Removing directory (recursive): %s", renderedPath)
			if err := os.RemoveAll(renderedPath); err != nil {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove directory: %w", err)
			}
		} else {
			ec.Logger.Debugf("  Removing directory: %s", renderedPath)
			if err := os.Remove(renderedPath); err != nil {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove directory (use force: true for non-empty): %w", err)
			}
		}

		// Emit directory.removed event
		ec.EmitEvent(events.EventDirRemoved, events.FileRemovedData{
			Path:   renderedPath,
			WasDir: true,
			DryRun: ec.DryRun,
		})
	} else {
		ec.Logger.Debugf("  Removing file: %s", renderedPath)
		if err := os.Remove(renderedPath); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to remove file: %w", err)
		}

		// Emit file.removed event
		ec.EmitEvent(events.EventFileRemoved, events.FileRemovedData{
			Path:      renderedPath,
			WasDir:    false,
			SizeBytes: size,
			DryRun:    ec.DryRun,
		})
	}

	return nil
}

// touchFile creates an empty file or updates timestamps on existing file.
func touchFile(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	// Check if file exists
	info, err := os.Stat(renderedPath)
	fileExists := err == nil

	mode := parseFileMode(file.Mode, defaultFileMode)

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if fileExists {
			dryRun.LogFileTouch(renderedPath)
		} else {
			dryRun.LogFileCreate(renderedPath, mode, 0)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if !fileExists {
		// Create empty file
		result.Changed = true
		ec.Logger.Debugf("  Creating empty file: %s", renderedPath)
		if err := createFileWithBecome(renderedPath, []byte(""), mode, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to create file: %w", err)
		}

		// Emit file.created event
		ec.EmitEvent(events.EventFileCreated, events.FileOperationData{
			Path:      renderedPath,
			Mode:      mode.String(),
			SizeBytes: 0,
			Changed:   true,
			DryRun:    ec.DryRun,
		})

		return nil
	}

	// File exists - update timestamp
	ec.Logger.Debugf("  Touching file: %s", renderedPath)
	now := time.Now()

	if step.Become {
		// Use touch command with sudo
		cmd := fmt.Sprintf("touch %q", renderedPath)
		if err := executeSudoCommand(cmd, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to touch file: %w", err)
		}
	} else {
		if err := os.Chtimes(renderedPath, now, now); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to update timestamps: %w", err)
		}
	}

	// Emit file.updated event (timestamp changed)
	ec.EmitEvent(events.EventFileUpdated, events.FileOperationData{
		Path:      renderedPath,
		Mode:      info.Mode().String(),
		SizeBytes: info.Size(),
		Changed:   false, // Content didn't change, only timestamp
		DryRun:    ec.DryRun,
	})

	return nil
}

// createSymlink creates a symbolic link from src to dest.
func createSymlink(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	if file.Src == "" {
		return fmt.Errorf("src is required for link state")
	}

	// Render source path
	renderedSrc, err := ec.PathUtil.ExpandPath(file.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return fmt.Errorf("failed to render src path: %w", err)
	}

	// Check existing link
	existingTarget, err := os.Readlink(renderedPath)
	linkExists := err == nil
	isCorrectLink := linkExists && existingTarget == renderedSrc

	// Check if path exists but is not a symlink
	if !linkExists && err != nil {
		if _, statErr := os.Stat(renderedPath); statErr == nil {
			// Path exists but is not a symlink
			if !file.Force {
				return fmt.Errorf("path exists and is not a symlink (use force: true to replace)")
			}
		}
	}

	changed := !isCorrectLink

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if changed {
			dryRun.LogSymlinkCreate(renderedSrc, renderedPath, file.Force)
		} else {
			dryRun.LogSymlinkNoChange(renderedSrc, renderedPath)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if isCorrectLink {
		// Already points to correct target
		return nil
	}

	result.Changed = true

	// Remove existing path if force is set or if it's a wrong symlink
	if linkExists || file.Force {
		ec.Logger.Debugf("  Removing existing path: %s", renderedPath)
		if step.Become {
			cmd := fmt.Sprintf("rm -f %q", renderedPath)
			if err := executeSudoCommand(cmd, step, ec); err != nil {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove existing path: %w", err)
			}
		} else {
			if err := os.Remove(renderedPath); err != nil && !os.IsNotExist(err) {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove existing path: %w", err)
			}
		}
	}

	// Create symlink
	ec.Logger.Debugf("  Creating symlink: %s -> %s", renderedPath, renderedSrc)
	if step.Become {
		cmd := fmt.Sprintf("ln -s %q %q", renderedSrc, renderedPath)
		if err := executeSudoCommand(cmd, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	} else {
		if err := os.Symlink(renderedSrc, renderedPath); err != nil {
			markStepFailed(result, step, ec)
			// Check for Windows-specific errors
			if runtime.GOOS == "windows" {
				return fmt.Errorf("failed to create symlink (Windows requires administrator privileges or developer mode): %w", err)
			}
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	// Emit link.created event
	ec.EmitEvent(events.EventLinkCreated, events.LinkCreatedData{
		Src:    renderedSrc,
		Dest:   renderedPath,
		Type:   "symlink",
		DryRun: ec.DryRun,
	})

	return nil
}

// createHardlink creates a hard link from src to dest.
func createHardlink(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	if file.Src == "" {
		return fmt.Errorf("src is required for hardlink state")
	}

	// Render source path
	renderedSrc, err := ec.PathUtil.ExpandPath(file.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return fmt.Errorf("failed to render src path: %w", err)
	}

	// Verify source exists and is not a directory
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		return fmt.Errorf("source file does not exist: %w", err)
	}
	if srcInfo.IsDir() {
		return fmt.Errorf("cannot create hardlink to directory (OS limitation)")
	}

	// Check if destination exists and points to same file
	destInfo, err := os.Stat(renderedPath)
	linkExists := err == nil
	isSameFile := false
	if linkExists {
		// Compare inode numbers to see if they're the same file
		isSameFile = os.SameFile(srcInfo, destInfo)
	}

	changed := !isSameFile

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if changed {
			dryRun.LogHardlinkCreate(renderedSrc, renderedPath, file.Force)
		} else {
			dryRun.LogHardlinkNoChange(renderedSrc, renderedPath)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if isSameFile {
		// Already linked to same file
		return nil
	}

	result.Changed = true

	// Remove existing path if force is set
	if linkExists {
		if !file.Force {
			return fmt.Errorf("destination exists (use force: true to replace)")
		}
		ec.Logger.Debugf("  Removing existing path: %s", renderedPath)
		if step.Become {
			cmd := fmt.Sprintf("rm -f %q", renderedPath)
			if err := executeSudoCommand(cmd, step, ec); err != nil {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove existing path: %w", err)
			}
		} else {
			if err := os.Remove(renderedPath); err != nil {
				markStepFailed(result, step, ec)
				return fmt.Errorf("failed to remove existing path: %w", err)
			}
		}
	}

	// Create hardlink
	ec.Logger.Debugf("  Creating hardlink: %s -> %s", renderedPath, renderedSrc)
	if step.Become {
		cmd := fmt.Sprintf("ln %q %q", renderedSrc, renderedPath)
		if err := executeSudoCommand(cmd, step, ec); err != nil {
			markStepFailed(result, step, ec)
			// Check for cross-filesystem error
			if err.Error() != "" {
				return fmt.Errorf("failed to create hardlink (may be cross-filesystem): %w", err)
			}
			return fmt.Errorf("failed to create hardlink: %w", err)
		}
	} else {
		if err := os.Link(renderedSrc, renderedPath); err != nil {
			markStepFailed(result, step, ec)
			// Check for cross-filesystem error
			return fmt.Errorf("failed to create hardlink (ensure both paths are on same filesystem): %w", err)
		}
	}

	// Emit link.created event
	ec.EmitEvent(events.EventLinkCreated, events.LinkCreatedData{
		Src:    renderedSrc,
		Dest:   renderedPath,
		Type:   "hardlink",
		DryRun: ec.DryRun,
	})

	return nil
}

// parseUserID parses a username or UID string into a UID.
func parseUserID(owner string) (int, error) {
	// Try parsing as numeric UID first
	if uid, err := strconv.Atoi(owner); err == nil {
		return uid, nil
	}

	// Lookup user by name
	u, err := user.Lookup(owner)
	if err != nil {
		return -1, fmt.Errorf("user not found: %s", owner)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, fmt.Errorf("invalid UID for user %s: %w", owner, err)
	}

	return uid, nil
}

// parseGroupID parses a group name or GID string into a GID.
func parseGroupID(group string) (int, error) {
	// Try parsing as numeric GID first
	if gid, err := strconv.Atoi(group); err == nil {
		return gid, nil
	}

	// Lookup group by name
	g, err := user.LookupGroup(group)
	if err != nil {
		return -1, fmt.Errorf("group not found: %s", group)
	}

	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return -1, fmt.Errorf("invalid GID for group %s: %w", group, err)
	}

	return gid, nil
}

// setOwnership changes the owner and/or group of a file or directory.
func setOwnership(path string, owner, group string, recurse bool, step config.Step, ec *ExecutionContext) error {
	if owner == "" && group == "" {
		return nil // Nothing to do
	}

	// Parse owner and group
	uid := -1
	gid := -1
	var err error

	if owner != "" {
		uid, err = parseUserID(owner)
		if err != nil {
			return err
		}
	}

	if group != "" {
		gid, err = parseGroupID(group)
		if err != nil {
			return err
		}
	}

	// Change ownership
	if step.Become {
		return chownWithBecome(path, owner, group, recurse, step, ec)
	}

	// Regular chown (requires appropriate permissions)
	if recurse {
		// For directories, walk and change ownership recursively
		// This is a simplified version - a production implementation would use filepath.Walk
		return fmt.Errorf("recursive ownership change without sudo not yet implemented (use become: true)")
	}

	if err := os.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("failed to change ownership: %w", err)
	}

	return nil
}

// chownWithBecome changes ownership using sudo.
func chownWithBecome(path, owner, group string, recurse bool, step config.Step, ec *ExecutionContext) error {
	// Validate platform support
	if !security.IsBecomeSupported() {
		return fmt.Errorf("become is not supported on %s", runtime.GOOS)
	}

	if ec.SudoPass == "" {
		return fmt.Errorf("step requires sudo but no password provided")
	}

	// Build chown command
	ownerGroup := ""
	if owner != "" && group != "" {
		ownerGroup = fmt.Sprintf("%s:%s", owner, group)
	} else if owner != "" {
		ownerGroup = owner
	} else if group != "" {
		ownerGroup = fmt.Sprintf(":%s", group)
	}

	var cmd string
	if recurse {
		cmd = fmt.Sprintf("chown -R %q %q", ownerGroup, path)
	} else {
		cmd = fmt.Sprintf("chown %q %q", ownerGroup, path)
	}

	return executeSudoCommand(cmd, step, ec)
}

// setPermissions changes permissions and/or ownership on an existing file.
func setPermissions(file *config.File, renderedPath string, result *Result, step config.Step, ec *ExecutionContext) error {
	// Check if path exists
	info, err := os.Stat(renderedPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %w", err)
	}

	isDir := info.IsDir()
	currentMode := info.Mode() & os.ModePerm
	targetMode := parseFileMode(file.Mode, currentMode)

	// Determine if changes are needed
	modeChanged := file.Mode != "" && targetMode != currentMode
	ownershipChanged := file.Owner != "" || file.Group != ""
	changed := modeChanged || ownershipChanged

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if changed {
			dryRun.LogPermissionsChange(renderedPath, file.Mode, file.Owner, file.Group, file.Recurse)
		} else {
			dryRun.LogPermissionsNoChange(renderedPath)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if !changed {
		// No changes needed
		return nil
	}

	result.Changed = true

	// Change mode if specified
	if modeChanged {
		ec.Logger.Debugf("  Changing permissions: %s (mode: %s)", renderedPath, formatMode(targetMode))
		if file.Recurse && isDir {
			// Recursive mode change with sudo if needed
			if step.Become {
				cmd := fmt.Sprintf("chmod -R %#o %q", targetMode, renderedPath)
				if err := executeSudoCommand(cmd, step, ec); err != nil {
					markStepFailed(result, step, ec)
					return fmt.Errorf("failed to change permissions: %w", err)
				}
			} else {
				// For non-sudo recursive, we'd need filepath.Walk - simplified for now
				return fmt.Errorf("recursive permission change without sudo not yet implemented (use become: true)")
			}
		} else {
			// Single file/directory
			if step.Become {
				cmd := fmt.Sprintf("chmod %#o %q", targetMode, renderedPath)
				if err := executeSudoCommand(cmd, step, ec); err != nil {
					markStepFailed(result, step, ec)
					return fmt.Errorf("failed to change permissions: %w", err)
				}
			} else {
				if err := os.Chmod(renderedPath, targetMode); err != nil {
					markStepFailed(result, step, ec)
					return fmt.Errorf("failed to change permissions: %w", err)
				}
			}
		}
	}

	// Change ownership if specified
	if ownershipChanged {
		ec.Logger.Debugf("  Changing ownership: %s (owner: %s, group: %s)", renderedPath, file.Owner, file.Group)
		if err := setOwnership(renderedPath, file.Owner, file.Group, file.Recurse, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return fmt.Errorf("failed to change ownership: %w", err)
		}
	}

	// Emit permissions.changed event
	ec.EmitEvent(events.EventPermissionsChanged, events.PermissionsChangedData{
		Path:      renderedPath,
		Mode:      targetMode.String(),
		Owner:     file.Owner,
		Group:     file.Group,
		Recursive: file.Recurse,
		DryRun:    ec.DryRun,
	})

	return nil
}

// removeWithBecome removes a file or directory using sudo
func removeWithBecome(path string, isDir bool, force bool, step config.Step, ec *ExecutionContext) error {
	// Validate platform support
	if !security.IsBecomeSupported() {
		return fmt.Errorf("become is not supported on %s", runtime.GOOS)
	}

	if ec.SudoPass == "" {
		return fmt.Errorf("step requires sudo but no password provided")
	}

	// Build remove command
	var cmd string
	if isDir && force {
		cmd = fmt.Sprintf("rm -rf %q", path)
	} else {
		cmd = fmt.Sprintf("rm %q", path)
	}

	return executeSudoCommand(cmd, step, ec)
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
	case "absent":
		if err := removeFileOrDirectory(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case "touch":
		if err := touchFile(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case "link":
		if err := createSymlink(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case "hardlink":
		if err := createHardlink(file, renderedPath, result, step, ec); err != nil {
			return err
		}
	case "perms":
		if err := setPermissions(file, renderedPath, result, step, ec); err != nil {
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
