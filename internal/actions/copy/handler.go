// Package copy implements the copy action handler.
//
// The copy action copies files from source to destination with:
// - Checksum verification (before and after copy)
// - Atomic write pattern (temp file + rename)
// - Backup support
// - Idempotency based on size/modtime
package copy

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Handler implements the Handler interface for copy actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the copy action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "copy",
		Description:    "Copy files with checksum verification and atomic writes",
		Category:       actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents:    []string{string(events.EventFileCopied)},
		Version:        "1.0.0",
	}
}

// Validate checks if the copy configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Copy == nil {
		return fmt.Errorf("copy configuration is nil")
	}

	copyAction := step.Copy
	if copyAction.Src == "" {
		return fmt.Errorf("src is required")
	}

	if copyAction.Dest == "" {
		return fmt.Errorf("dest is required")
	}

	return nil
}

// Execute runs the copy action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	copyAction := step.Copy

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Render paths
	renderedSrc, err := ec.PathUtil.ExpandPath(copyAction.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}

	renderedDest, err := ec.PathUtil.ExpandPath(copyAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Verify source exists
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to stat source: %w", err)
	}
	if srcInfo.IsDir() {
		result.Failed = true
		return result, fmt.Errorf("src is a directory, use recursive copy action instead")
	}

	// Verify source checksum if provided
	if copyAction.Checksum != "" {
		ctx.GetLogger().Debugf("  Verifying source checksum: %s", copyAction.Checksum)
		matches, checksumErr := utils.VerifyChecksum(renderedSrc, copyAction.Checksum)
		if checksumErr != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify source checksum: %w", checksumErr)
		}
		if !matches {
			result.Failed = true
			return result, fmt.Errorf("source checksum mismatch")
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

	// Parse mode (use source mode if not specified)
	mode := h.parseFileMode(copyAction.Mode, srcInfo.Mode()&os.ModePerm)

	if !needsCopy {
		ctx.GetLogger().Debugf("  File already up to date: %s", renderedDest)
		return result, nil
	}

	result.Changed = true

	// Create backup if requested and dest exists
	if copyAction.Backup && destExists {
		ctx.GetLogger().Debugf("  Creating backup of: %s", renderedDest)
		backupPath, err := utils.CreateBackup(renderedDest)
		if err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
		ctx.GetLogger().Debugf("  Backup created: %s", backupPath)
	}

	// Copy file
	ctx.GetLogger().Debugf("  Copying file: %s -> %s", renderedSrc, renderedDest)
	if err := h.copyFile(renderedSrc, renderedDest, mode, step, ec, ctx); err != nil {
		result.Failed = true
		return result, err
	}

	// Set ownership if specified
	if copyAction.Owner != "" || copyAction.Group != "" {
		ctx.GetLogger().Debugf("  Setting ownership: %s (owner: %s, group: %s)", renderedDest, copyAction.Owner, copyAction.Group)
		if err := h.setOwnership(renderedDest, copyAction.Owner, copyAction.Group, step, ec); err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	// Verify destination checksum if provided
	if copyAction.Checksum != "" {
		ctx.GetLogger().Debugf("  Verifying destination checksum: %s", copyAction.Checksum)
		matches, err := utils.VerifyChecksum(renderedDest, copyAction.Checksum)
		if err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify destination checksum: %w", err)
		}
		if !matches {
			result.Failed = true
			return result, fmt.Errorf("destination checksum mismatch after copy")
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventFileCopied,
			Data: events.FileCopiedData{
				Src:       renderedSrc,
				Dest:      renderedDest,
				SizeBytes: srcInfo.Size(),
				Mode:      mode.String(),
				Checksum:  copyAction.Checksum,
				DryRun:    ctx.IsDryRun(),
			},
		})
	}

	return result, nil
}

// DryRun logs what would be done without actually doing it.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	copyAction := step.Copy

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render paths
	renderedSrc, err := ec.PathUtil.ExpandPath(copyAction.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedSrc = copyAction.Src
	}

	renderedDest, err := ec.PathUtil.ExpandPath(copyAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedDest = copyAction.Dest
	}

	// Check if source exists
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		ctx.GetLogger().Errorf("  [DRY-RUN] Source file not found: %s", renderedSrc)
		return fmt.Errorf("source not found: %s", renderedSrc)
	}

	// Check if destination exists
	destInfo, err := os.Stat(renderedDest)
	destExists := err == nil

	// Determine if copy is needed
	needsCopy := !destExists || copyAction.Force
	if destExists && !copyAction.Force {
		if destInfo.Size() == srcInfo.Size() && destInfo.ModTime().Equal(srcInfo.ModTime()) {
			needsCopy = false
		} else {
			needsCopy = true
		}
	}

	mode := h.parseFileMode(copyAction.Mode, srcInfo.Mode()&os.ModePerm)

	if needsCopy {
		ctx.GetLogger().Infof("  [DRY-RUN] Would copy file: %s -> %s (size: %d bytes, mode: %s)",
			renderedSrc, renderedDest, srcInfo.Size(), h.formatMode(mode))
	} else {
		ctx.GetLogger().Infof("  [DRY-RUN] File already up to date: %s", renderedDest)
	}

	if copyAction.Checksum != "" {
		ctx.GetLogger().Debugf("  Would verify checksum: %s", copyAction.Checksum)
	}

	if copyAction.Backup && destExists && needsCopy {
		ctx.GetLogger().Debugf("  Would create backup before overwrite")
	}

	if copyAction.Owner != "" || copyAction.Group != "" {
		ctx.GetLogger().Debugf("  Would set ownership: owner=%s group=%s", copyAction.Owner, copyAction.Group)
	}

	return nil
}

// Helper functions

func (h *Handler) formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

func (h *Handler) parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

func (h *Handler) copyFile(src, dest string, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext, ctx actions.Context) error {
	// #nosec G304 -- File path from user config is intentional
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close source file: %v", closeErr)
		}
	}()

	// Create temporary file for atomic write
	tmpFile, err := os.CreateTemp("", "mooncake-copy-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close temp file: %v", closeErr)
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			ctx.GetLogger().Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
		}
	}()

	// Copy contents
	if _, err := io.Copy(tmpFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	// Close temp file before moving
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	// Move temp file to destination (atomic)
	if step.Become {
		if !security.IsBecomeSupported() {
			return fmt.Errorf("become not supported on %s", runtime.GOOS)
		}
		if ec.SudoPass == "" {
			return fmt.Errorf("step requires sudo but no password provided")
		}
		// Use sudo for final move
		cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			return fmt.Errorf("failed to move file with sudo: %w", err)
		}
	} else {
		if err := os.Rename(tmpPath, dest); err != nil {
			return fmt.Errorf("failed to move file: %w", err)
		}
	}

	return nil
}

func (h *Handler) setOwnership(path, owner, group string, step *config.Step, ec *executor.ExecutionContext) error {
	if owner == "" && group == "" {
		return nil
	}

	if step.Become || runtime.GOOS != "linux" {
		return h.chownWithBecome(path, owner, group, step, ec)
	}

	// Parse owner and group
	uid := -1
	gid := -1
	var err error

	if owner != "" {
		uid, err = h.parseUserID(owner)
		if err != nil {
			return fmt.Errorf("failed to parse owner: %w", err)
		}
	}

	if group != "" {
		gid, err = h.parseGroupID(group)
		if err != nil {
			return fmt.Errorf("failed to parse group: %w", err)
		}
	}

	return os.Chown(path, uid, gid)
}

func (h *Handler) chownWithBecome(path, owner, group string, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.Become {
		return fmt.Errorf("chown requires become: true")
	}

	if !security.IsBecomeSupported() {
		return fmt.Errorf("become not supported on %s", runtime.GOOS)
	}

	if ec.SudoPass == "" {
		return fmt.Errorf("step requires sudo but no password provided")
	}

	ownerGroup := ""
	if owner != "" && group != "" {
		ownerGroup = owner + ":" + group
	} else if owner != "" {
		ownerGroup = owner
	} else if group != "" {
		ownerGroup = ":" + group
	}

	cmd := fmt.Sprintf("chown %s %q", ownerGroup, path)
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) parseUserID(owner string) (int, error) {
	// Try as UID first
	if uid, err := strconv.Atoi(owner); err == nil {
		return uid, nil
	}

	// Lookup username
	u, err := user.Lookup(owner)
	if err != nil {
		return -1, fmt.Errorf("user not found: %s", owner)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, fmt.Errorf("invalid UID: %s", u.Uid)
	}

	return uid, nil
}

func (h *Handler) parseGroupID(group string) (int, error) {
	// Try as GID first
	if gid, err := strconv.Atoi(group); err == nil {
		return gid, nil
	}

	// Lookup group name
	g, err := user.LookupGroup(group)
	if err != nil {
		return -1, fmt.Errorf("group not found: %s", group)
	}

	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return -1, fmt.Errorf("invalid GID: %s", g.Gid)
	}

	return gid, nil
}

func (h *Handler) executeSudoCommand(command string, step *config.Step, ec *executor.ExecutionContext) error {
	// #nosec G204 - This is a provisioning tool designed to execute commands
	cmd := exec.Command("sudo", "-S", "sh", "-c", command)
	cmd.Stdin = bytes.NewBufferString(ec.SudoPass + "\n")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
