package executor

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/utils"
)

// HandleCopy copies a file from source to destination with optional checksum verification.
func HandleCopy(step config.Step, ec *ExecutionContext) error {
	copyAction := step.Copy

	if copyAction.Src == "" {
		return &StepValidationError{Field: "src", Message: "required for copy"}
	}
	if copyAction.Dest == "" {
		return &StepValidationError{Field: "dest", Message: "required for copy"}
	}

	// Render paths
	renderedSrc, err := ec.PathUtil.ExpandPath(copyAction.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "src path", Cause: err}
	}

	renderedDest, err := ec.PathUtil.ExpandPath(copyAction.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "dest path", Cause: err}
	}

	// Create result object
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Verify source exists
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		return &FileOperationError{Operation: "read", Path: renderedSrc, Cause: err}
	}
	if srcInfo.IsDir() {
		return &StepValidationError{Field: "src", Message: "is a directory, use recursive copy action instead"}
	}

	// Verify source checksum if provided
	if copyAction.Checksum != "" {
		matches, checksumErr := utils.VerifyChecksum(renderedSrc, copyAction.Checksum)
		if checksumErr != nil {
			return &FileOperationError{Operation: "read", Path: renderedSrc, Cause: checksumErr}
		}
		if !matches {
			return &StepValidationError{Field: "checksum", Message: "source checksum mismatch"}
		}
	}

	// Check if destination exists
	destInfo, err := os.Stat(renderedDest)
	destExists := err == nil

	// Determine if copy is needed
	needsCopy := !destExists || copyAction.Force
	if destExists && !copyAction.Force {
		// Compare file sizes and modification times for idempotency
		if destInfo.Size() == srcInfo.Size() && destInfo.ModTime().Equal(srcInfo.ModTime()) {
			needsCopy = false
		} else {
			needsCopy = true
		}
	}

	mode := parseFileMode(copyAction.Mode, srcInfo.Mode()&os.ModePerm)

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if needsCopy {
			dryRun.LogFileCopy(renderedSrc, renderedDest, mode, srcInfo.Size())
		} else {
			dryRun.LogFileCopyNoChange(renderedSrc, renderedDest)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if !needsCopy {
		// Already up to date
		return nil
	}

	result.Changed = true

	// Create backup if requested and dest exists
	if copyAction.Backup && destExists {
		ec.Logger.Debugf("  Creating backup of: %s", renderedDest)
		backupPath, err := utils.CreateBackup(renderedDest)
		if err != nil {
			markStepFailed(result, step, ec)
			return &FileOperationError{Operation: "create", Path: renderedDest + ".backup", Cause: err}
		}
		ec.Logger.Debugf("  Backup created: %s", backupPath)
	}

	// Copy file
	ec.Logger.Debugf("  Copying file: %s -> %s", renderedSrc, renderedDest)
	if err := copyFile(renderedSrc, renderedDest, mode, step, ec); err != nil {
		markStepFailed(result, step, ec)
		return err
	}

	// Set ownership if specified
	if copyAction.Owner != "" || copyAction.Group != "" {
		ec.Logger.Debugf("  Setting ownership: %s (owner: %s, group: %s)", renderedDest, copyAction.Owner, copyAction.Group)
		if err := setOwnership(renderedDest, copyAction.Owner, copyAction.Group, false, step, ec); err != nil {
			markStepFailed(result, step, ec)
			return err
		}
	}

	// Verify destination checksum if provided
	if copyAction.Checksum != "" {
		matches, err := utils.VerifyChecksum(renderedDest, copyAction.Checksum)
		if err != nil {
			markStepFailed(result, step, ec)
			return &FileOperationError{Operation: "read", Path: renderedDest, Cause: err}
		}
		if !matches {
			markStepFailed(result, step, ec)
			return &StepValidationError{Field: "checksum", Message: "destination checksum mismatch after copy"}
		}
	}

	// Emit file.copied event
	ec.EmitEvent(events.EventFileCopied, events.FileCopiedData{
		Src:       renderedSrc,
		Dest:      renderedDest,
		SizeBytes: srcInfo.Size(),
		Mode:      mode.String(),
		Checksum:  copyAction.Checksum,
		DryRun:    ec.DryRun,
	})

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	return nil
}

// copyFile copies a file from src to dest with atomic write pattern.
func copyFile(src, dest string, mode os.FileMode, step config.Step, ec *ExecutionContext) error {
	// #nosec G304 -- File path from user config is intentional functionality
	srcFile, err := os.Open(src)
	if err != nil {
		return &FileOperationError{Operation: "read", Path: src, Cause: err}
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			ec.Logger.Debugf("Failed to close source file: %v", closeErr)
		}
	}()

	// Create temporary file for atomic write
	tmpFile, err := os.CreateTemp("", "mooncake-copy-*")
	if err != nil {
		return &FileOperationError{Operation: "create", Path: "temp file in " + os.TempDir(), Cause: err}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			ec.Logger.Debugf("Failed to close temp file: %v", closeErr)
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			ec.Logger.Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
		}
	}()

	// Copy contents
	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		return &FileOperationError{Operation: "write", Path: tmpPath, Cause: err}
	}

	// Close temp file before moving
	if err := tmpFile.Close(); err != nil {
		return &FileOperationError{Operation: "write", Path: tmpPath, Cause: err}
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return &FileOperationError{Operation: "chmod", Path: tmpPath, Cause: err}
	}

	// Move temp file to destination (atomic)
	if step.Become {
		// Use sudo for final move
		cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
		if err := executeSudoCommand(cmd, step, ec); err != nil {
			return &FileOperationError{Operation: "write", Path: dest, Cause: err}
		}
	} else {
		if err := os.Rename(tmpPath, dest); err != nil {
			return &FileOperationError{Operation: "write", Path: dest, Cause: err}
		}
	}

	return nil
}
