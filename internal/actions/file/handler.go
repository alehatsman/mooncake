// Package file implements the file action handler.
//
// The file action manages files, directories, and links with support for:
// - Creating/updating files with content
// - Creating directories
// - Removing files and directories
// - Creating symbolic and hard links
// - Setting permissions and ownership
// - Touch operations (update timestamps)
package file

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
)

const (
	defaultFileMode os.FileMode = 0644
	defaultDirMode  os.FileMode = 0755
	actionTypeFile              = "file"
	stateLink                   = "link"
	stateHardlink               = "hardlink"
)

// defaultModeFor returns the mode applied when a file step omits an explicit
// `mode:`. Both Execute and DryRun must consult this so the preview matches
// what is actually performed. See spec-16-unify-dryrun-execute for the
// longer-term plan to remove the parallel paths entirely.
func defaultModeFor(state string) os.FileMode {
	if state == "directory" {
		return defaultDirMode
	}
	return defaultFileMode
}

// Handler implements the Handler interface for file actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the file action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "file.write",
		Description:    "Manage files, directories, links, and permissions",
		Category:       actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileCreated),
			string(events.EventFileUpdated),
			string(events.EventFileRemoved),
			string(events.EventDirCreated),
			string(events.EventDirRemoved),
			string(events.EventLinkCreated),
			string(events.EventPermissionsChanged),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on path/ownership operation
		ImplementsCheck:    true,       // Checks existence, permissions, ownership before changes
	}
}

// Validate checks if the file configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileWrite == nil {
		return fmt.Errorf("file configuration is nil")
	}

	file := step.FileWrite
	if file.Path == "" {
		// Generate hint from schema
		hint := actions.GetActionHint("file", "path")

		// Add context-aware note if user used 'src' instead of 'path'
		note := ""
		if file.Src != "" {
			note = "\nNote: You provided 'src' but 'path' is required. The 'src' parameter is only used with state='link' or state='hardlink'.\n"
		}

		return fmt.Errorf("file path is empty%s%s", note, hint)
	}

	// Validate state
	validStates := map[string]bool{
		"file": true, "directory": true, "absent": true,
		"touch": true, stateLink: true, stateHardlink: true, "perms": true,
	}
	if file.State != "" && !validStates[file.State] {
		hint := actions.GetActionHint("file", "state")
		return fmt.Errorf("invalid state: %s%s", file.State, hint)
	}

	// Validate link operations require src
	if (file.State == stateLink || file.State == stateHardlink) && file.Src == "" {
		hint := actions.GetActionHint("file", "src")
		return fmt.Errorf("state %s requires src parameter%s", file.State, hint)
	}

	return nil
}

// Execute runs the file action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	file := step.FileWrite

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Expand path
	renderedPath, err := ec.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Determine state (default to "file")
	state := file.State
	if state == "" {
		state = actionTypeFile
	}

	// Dispatch based on state
	switch state {
	case "directory":
		err = h.createDirectory(ctx, ec, file, renderedPath, result, step)
	case "absent":
		err = h.removeFileOrDirectory(ctx, ec, file, renderedPath, result, step)
	case "touch":
		err = h.touchFile(ctx, ec, file, renderedPath, result, step)
	case stateLink:
		err = h.createSymlink(ctx, ec, file, renderedPath, result, step)
	case stateHardlink:
		err = h.createHardlink(ctx, ec, file, renderedPath, result, step)
	case "perms":
		err = h.setPermissions(ctx, ec, file, renderedPath, result, step)
	case actionTypeFile:
		err = h.createOrUpdateFile(ctx, ec, file, renderedPath, result, step)
	default:
		return nil, fmt.Errorf("unknown file state: %s", state)
	}

	if err != nil {
		return result, err
	}

	return result, nil
}

// DryRun logs what would be done without actually doing it.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	file := step.FileWrite

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	renderedPath, err := ec.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedPath = file.Path
	}

	state := file.State
	if state == "" {
		state = actionTypeFile
	}

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))

	switch state {
	case "directory":
		ctx.GetLogger().Infof("  [DRY-RUN] Would create directory: %s (mode: %s)", renderedPath, h.formatMode(mode))
	case "absent":
		ctx.GetLogger().Infof("  [DRY-RUN] Would remove: %s", renderedPath)
	case "touch":
		ctx.GetLogger().Infof("  [DRY-RUN] Would touch file: %s", renderedPath)
	case stateLink:
		ctx.GetLogger().Infof("  [DRY-RUN] Would create symlink: %s -> %s", renderedPath, file.Src)
	case stateHardlink:
		ctx.GetLogger().Infof("  [DRY-RUN] Would create hardlink: %s -> %s", renderedPath, file.Src)
	case "perms":
		ctx.GetLogger().Infof("  [DRY-RUN] Would set permissions: %s (mode: %s)", renderedPath, h.formatMode(mode))
	case actionTypeFile:
		contentSize := len(file.Content)
		ctx.GetLogger().Infof("  [DRY-RUN] Would create/update file: %s (size: %d bytes, mode: %s)",
			renderedPath, contentSize, h.formatMode(mode))
	}

	if file.Owner != "" || file.Group != "" {
		ctx.GetLogger().Infof("             Ownership: owner=%s group=%s", file.Owner, file.Group)
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

// State handler methods will be added below...

func (h *Handler) createDirectory(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	mode := h.parseFileMode(file.Mode, defaultModeFor("directory"))

	// Check if directory already exists
	if _, err := os.Stat(renderedPath); os.IsNotExist(err) {
		result.Changed = true
	}

	ctx.GetLogger().Debugf("  Creating directory: %s", renderedPath)
	if err := h.createDirectoryWithBecome(renderedPath, mode, step, ec); err != nil {
		result.Failed = true
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set ownership if specified
	if file.Owner != "" || file.Group != "" {
		if err := h.setOwnership(renderedPath, file.Owner, file.Group, file.Recurse, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventDirCreated,
			Data: events.FileOperationData{
				Path:    renderedPath,
				Mode:    h.formatMode(mode),
				Changed: result.Changed,
				DryRun:  ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

func (h *Handler) createOrUpdateFile(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	mode := h.parseFileMode(file.Mode, defaultModeFor(actionTypeFile))

	// Render content
	renderedContent, err := ctx.GetTemplate().Render(file.Content, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to render content: %w", err)
	}

	content := []byte(renderedContent)

	// Check if file exists and content has changed
	// #nosec G304 -- File path from user config is intentional
	existingContent, err := os.ReadFile(renderedPath)
	fileExists := (err == nil)
	contentChanged := !fileExists || !bytes.Equal(existingContent, content)

	if contentChanged {
		result.Changed = true
	}

	// Create backup if requested and file exists
	if file.Backup && fileExists {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, existingContent, 0600); err != nil {
			ctx.GetLogger().Debugf("  Warning: failed to create backup: %v", err)
		} else {
			ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
		}
	}

	// Write file
	ctx.GetLogger().Debugf("  Writing file: %s (%d bytes)", renderedPath, len(content))
	if err := h.createFileWithBecome(renderedPath, content, mode, step, ec); err != nil {
		result.Failed = true
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Set ownership if specified
	if file.Owner != "" || file.Group != "" {
		if err := h.setOwnership(renderedPath, file.Owner, file.Group, false, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		eventType := events.EventFileCreated
		if fileExists {
			eventType = events.EventFileUpdated
		}
		publisher.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:      renderedPath,
				Mode:      h.formatMode(mode),
				SizeBytes: int64(len(content)),
				Changed:   result.Changed,
				DryRun:    ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

func (h *Handler) removeFileOrDirectory(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	// Check if path exists
	fileInfo, err := os.Stat(renderedPath)
	if os.IsNotExist(err) {
		// Already absent
		result.Changed = false
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to stat path: %w", err)
	}

	result.Changed = true
	isDir := fileInfo.IsDir()

	ctx.GetLogger().Debugf("  Removing: %s", renderedPath)
	if err := h.removeWithBecome(renderedPath, isDir, file.Force, step, ec); err != nil {
		result.Failed = true
		return fmt.Errorf("failed to remove: %w", err)
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		eventType := events.EventFileRemoved
		if isDir {
			eventType = events.EventDirRemoved
		}
		publisher.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:    renderedPath,
				Changed: result.Changed,
				DryRun:  ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

func (h *Handler) touchFile(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	mode := h.parseFileMode(file.Mode, defaultModeFor("touch"))

	// Check if file exists
	_, err := os.Stat(renderedPath)
	fileExists := (err == nil)

	if !fileExists {
		// Create empty file
		result.Changed = true
		ctx.GetLogger().Debugf("  Creating empty file: %s", renderedPath)
		if err := h.createFileWithBecome(renderedPath, []byte{}, mode, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to create file: %w", err)
		}
	} else {
		// Update timestamp
		result.Changed = true
		now := time.Now()
		ctx.GetLogger().Debugf("  Touching file: %s", renderedPath)
		if step.ShouldBecome() {
			// Use touch command with sudo
			if err := h.executeSudoCommand(fmt.Sprintf("touch %s", renderedPath), step, ec); err != nil {
				result.Failed = true
				return fmt.Errorf("failed to touch file: %w", err)
			}
		} else {
			if err := os.Chtimes(renderedPath, now, now); err != nil {
				result.Failed = true
				return fmt.Errorf("failed to touch file: %w", err)
			}
		}
	}

	// Set ownership if specified
	if file.Owner != "" || file.Group != "" {
		if err := h.setOwnership(renderedPath, file.Owner, file.Group, false, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	return nil
}

func (h *Handler) createSymlink(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	// Render src path
	renderedSrc, err := ctx.GetTemplate().Render(file.Src, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to render src: %w", err)
	}

	// Expand src path
	expandedSrc, err := ec.PathUtil.ExpandPath(renderedSrc, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to expand src path: %w", err)
	}

	// Check if link already exists with correct target
	if linkTarget, err := os.Readlink(renderedPath); err == nil {
		if linkTarget == expandedSrc {
			// Already correct
			result.Changed = false
			return nil
		}
		// Wrong target, remove it
		if file.Force {
			if err := os.Remove(renderedPath); err != nil {
				return fmt.Errorf("failed to remove existing link: %w", err)
			}
		} else {
			return fmt.Errorf("link exists with different target (use force: true to overwrite)")
		}
	}

	result.Changed = true
	ctx.GetLogger().Debugf("  Creating symlink: %s -> %s", renderedPath, expandedSrc)

	if step.ShouldBecome() {
		cmd := fmt.Sprintf("ln -s %s %s", expandedSrc, renderedPath)
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	} else {
		if err := os.Symlink(expandedSrc, renderedPath); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to create symlink: %w", err)
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventLinkCreated,
			Data: events.LinkCreatedData{
				Src:    expandedSrc,
				Dest:   renderedPath,
				Type:   "symlink",
				DryRun: ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

func (h *Handler) createHardlink(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	// Render src path
	renderedSrc, err := ctx.GetTemplate().Render(file.Src, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to render src: %w", err)
	}

	// Expand src path
	expandedSrc, err := ec.PathUtil.ExpandPath(renderedSrc, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("failed to expand src path: %w", err)
	}

	// Check if hardlink already exists
	if _, err := os.Stat(renderedPath); err == nil {
		// File exists - check if it's the same inode
		srcInfo, err1 := os.Stat(expandedSrc)
		dstInfo, err2 := os.Stat(renderedPath)
		if err1 == nil && err2 == nil && os.SameFile(srcInfo, dstInfo) {
			// Already hard linked
			result.Changed = false
			return nil
		}
		// Different file, remove if force
		if file.Force {
			if err := os.Remove(renderedPath); err != nil {
				return fmt.Errorf("failed to remove existing file: %w", err)
			}
		} else {
			return fmt.Errorf("file exists (use force: true to overwrite)")
		}
	}

	result.Changed = true
	ctx.GetLogger().Debugf("  Creating hardlink: %s -> %s", renderedPath, expandedSrc)

	if step.ShouldBecome() {
		cmd := fmt.Sprintf("ln %s %s", expandedSrc, renderedPath)
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to create hardlink: %w", err)
		}
	} else {
		if err := os.Link(expandedSrc, renderedPath); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to create hardlink: %w", err)
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventLinkCreated,
			Data: events.LinkCreatedData{
				Src:    expandedSrc,
				Dest:   renderedPath,
				Type:   stateHardlink,
				DryRun: ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

func (h *Handler) setPermissions(ctx actions.Context, ec *executor.ExecutionContext, file *config.File, renderedPath string, result *executor.Result, step *config.Step) error {
	mode := h.parseFileMode(file.Mode, defaultModeFor("perms"))

	// Get current permissions
	fileInfo, err := os.Stat(renderedPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	currentMode := fileInfo.Mode() & os.ModePerm
	if currentMode != mode {
		result.Changed = true
	}

	ctx.GetLogger().Debugf("  Setting permissions: %s (mode: %s)", renderedPath, h.formatMode(mode))

	if step.ShouldBecome() {
		cmd := fmt.Sprintf("chmod %s %s", file.Mode, renderedPath)
		if file.Recurse {
			cmd = fmt.Sprintf("chmod -R %s %s", file.Mode, renderedPath)
		}
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	} else {
		if file.Recurse {
			// Recursive not implemented without become for now
			return fmt.Errorf("recursive permission changes require become: true")
		}
		if err := os.Chmod(renderedPath, mode); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set permissions: %w", err)
		}
	}

	// Set ownership if specified
	if file.Owner != "" || file.Group != "" {
		if err := h.setOwnership(renderedPath, file.Owner, file.Group, file.Recurse, step, ec); err != nil {
			result.Failed = true
			return fmt.Errorf("failed to set ownership: %w", err)
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventPermissionsChanged,
			Data: events.PermissionsChangedData{
				Path:      renderedPath,
				Mode:      h.formatMode(mode),
				Owner:     file.Owner,
				Group:     file.Group,
				Recursive: file.Recurse,
				DryRun:    ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return nil
}

// Become support helpers

func (h *Handler) createFileWithBecome(path string, content []byte, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.ShouldBecome() {
		// #nosec G306 -- Mode is user-configurable for provisioning
		return os.WriteFile(path, content, mode)
	}

	if !security.IsBecomeSupported() {
		return fmt.Errorf("become not supported on %s", runtime.GOOS)
	}

	// Create temp file
	tmpFile, err := os.CreateTemp("", "mooncake-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	// Write content to temp file
	if err := os.WriteFile(tmpPath, content, 0600); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Move with sudo
	return h.executeSudoFileOperation(tmpPath, path, mode, step, ec)
}

func (h *Handler) createDirectoryWithBecome(path string, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.ShouldBecome() {
		return os.MkdirAll(path, mode)
	}

	if !security.IsBecomeSupported() {
		return fmt.Errorf("become not supported on %s", runtime.GOOS)
	}

	cmd := fmt.Sprintf("mkdir -p -m %s %s", h.formatMode(mode), path)
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) removeWithBecome(path string, isDir bool, _ bool, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.ShouldBecome() {
		if isDir {
			return os.RemoveAll(path)
		}
		return os.Remove(path)
	}

	if !security.IsBecomeSupported() {
		return fmt.Errorf("become not supported on %s", runtime.GOOS)
	}

	cmd := fmt.Sprintf("rm -f %s", path)
	if isDir {
		cmd = fmt.Sprintf("rm -rf %s", path)
	}
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) setOwnership(path, owner, group string, recurse bool, step *config.Step, ec *executor.ExecutionContext) error {
	if owner == "" && group == "" {
		return nil
	}

	if step.ShouldBecome() || runtime.GOOS != "linux" {
		return h.chownWithBecome(path, owner, group, recurse, step, ec)
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

func (h *Handler) chownWithBecome(path, owner, group string, recurse bool, step *config.Step, ec *executor.ExecutionContext) error {
	if !step.ShouldBecome() {
		return fmt.Errorf("chown requires become: true")
	}

	if !security.IsBecomeSupported() {
		return fmt.Errorf("become not supported on %s", runtime.GOOS)
	}

	ownerGroup := ""
	if owner != "" && group != "" {
		ownerGroup = owner + ":" + group
	} else if owner != "" {
		ownerGroup = owner
	} else if group != "" {
		ownerGroup = ":" + group
	}

	cmd := fmt.Sprintf("chown %s %s", ownerGroup, path)
	if recurse {
		cmd = fmt.Sprintf("chown -R %s %s", ownerGroup, path)
	}

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

func (h *Handler) executeSudoFileOperation(tmpPath, destPath string, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext) error {
	// Move file and set permissions with sudo
	cmd := fmt.Sprintf("mv %s %s && chmod %s %s", tmpPath, destPath, h.formatMode(mode), destPath)
	return h.executeSudoCommand(cmd, step, ec)
}

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
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

// Run is the Spec 16 unified entry point. It replaces the legacy Execute,
// DryRun, and Check methods. The handler inspects state once, then either
// performs side effects (ModeApply) or returns a prediction (ModePlan).
//
// All filesystem mutations route through ctx.Effects() so the same
// defaulting and predicate logic decides both the preview and the real
// run. This is the structural fix for the drift class of bugs that
// motivated Spec 16 (see docs-working/spec-16-unify-dryrun-execute.md).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	file := step.FileWrite

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	renderedPath, err := ec.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	state := file.State
	if state == "" {
		state = actionTypeFile
	}

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))
	p := ctx.Effects()
	opts := actions.PerformerOpts{Become: step.ShouldBecome()}

	primary, err := h.runState(ctx, ec, file, step, state, renderedPath, mode, p, opts)
	if err != nil {
		result.Failed = true
		return result, err
	}
	if primary.Err != nil {
		result.Failed = true
		return result, primary.Err
	}

	// Plan mode: surface prediction; skip ownership/event side effects.
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = primary.WouldChange
		result.Reason = primary.Reason
		// AlreadyOk in plan mode is still a "no-change" outcome; surface it
		// as a non-changing successful result.
		return result, nil
	}

	// Execute mode: real side effects ran.
	if primary.Performed {
		result.Changed = true
	}

	// Ownership: run after primary so chown applies to the just-written target.
	if file.Owner != "" || file.Group != "" {
		ownEffect := p.Chown(renderedPath, file.Owner, file.Group, opts)
		if ownEffect.Err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to set ownership: %w", ownEffect.Err)
		}
		if ownEffect.Performed {
			result.Changed = true
		}
	}

	emitFileEvent(ctx, state, primary, renderedPath, mode, h.formatMode)
	return result, nil
}

// runState dispatches to the per-state Effect call, returning the
// primary Effect. Returns (Effect, error) where error covers pre-Effect
// validation failures (e.g. template render errors); Effect.Err covers
// failures from the Performer itself.
func (h *Handler) runState(
	ctx actions.Context,
	ec *executor.ExecutionContext,
	file *config.File,
	_ *config.Step,
	state, renderedPath string,
	mode os.FileMode,
	p actions.Performer,
	opts actions.PerformerOpts,
) (actions.Effect, error) {
	switch state {
	case "directory":
		return p.Mkdir(renderedPath, mode, opts), nil

	case actionTypeFile:
		rendered, err := ctx.GetTemplate().Render(file.Content, ctx.GetVariables())
		if err != nil {
			return actions.Effect{}, fmt.Errorf("failed to render content: %w", err)
		}
		content := []byte(rendered)

		// Backup before overwriting — only in execute mode and only if
		// the file currently exists.
		if ctx.Mode() == actions.ModeApply && file.Backup {
			if existing, readErr := os.ReadFile(renderedPath); readErr == nil {
				backupPath := renderedPath + ".bak"
				if writeErr := os.WriteFile(backupPath, existing, 0o600); writeErr != nil {
					ctx.GetLogger().Debugf("  Warning: failed to create backup: %v", writeErr)
				} else {
					ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
				}
			}
		}
		return p.WriteFile(renderedPath, content, mode, opts), nil

	case "absent":
		return p.Remove(renderedPath, true, opts), nil

	case "touch":
		return p.Touch(renderedPath, mode, opts), nil

	case stateLink:
		expandedSrc, err := h.resolveLinkSrc(ctx, ec, file)
		if err != nil {
			return actions.Effect{}, err
		}
		if err := h.checkLinkForce(renderedPath, expandedSrc, file.Force); err != nil {
			return actions.Effect{}, err
		}
		return p.Symlink(expandedSrc, renderedPath, opts), nil

	case stateHardlink:
		expandedSrc, err := h.resolveLinkSrc(ctx, ec, file)
		if err != nil {
			return actions.Effect{}, err
		}
		if err := h.checkHardlinkForce(renderedPath, expandedSrc, file.Force); err != nil {
			return actions.Effect{}, err
		}
		return p.Hardlink(expandedSrc, renderedPath, opts), nil

	case "perms":
		return p.Chmod(renderedPath, mode, opts), nil

	default:
		return actions.Effect{}, fmt.Errorf("unknown file state: %s", state)
	}
}

// resolveLinkSrc renders and path-expands the link source. Returns the
// fully-resolved path or an error if rendering/expansion fails.
func (h *Handler) resolveLinkSrc(ctx actions.Context, ec *executor.ExecutionContext, file *config.File) (string, error) {
	rendered, err := ctx.GetTemplate().Render(file.Src, ctx.GetVariables())
	if err != nil {
		return "", fmt.Errorf("failed to render src: %w", err)
	}
	expanded, err := ec.PathUtil.ExpandPath(rendered, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return "", fmt.Errorf("failed to expand src path: %w", err)
	}
	return expanded, nil
}

// checkLinkForce errors out when a symlink exists with a different
// target and force is not set. This matches the legacy contract.
func (h *Handler) checkLinkForce(path, desiredTarget string, force bool) error {
	if linkTarget, err := os.Readlink(path); err == nil {
		if linkTarget != desiredTarget && !force {
			return errors.New("link exists with different target (use force: true to overwrite)")
		}
	}
	return nil
}

// checkHardlinkForce errors out when a regular file or wrong-inode
// hardlink exists and force is not set.
func (h *Handler) checkHardlinkForce(path, desiredTarget string, force bool) error {
	dstInfo, err := os.Stat(path)
	if err != nil {
		return nil // doesn't exist; safe to create
	}
	srcInfo, srcErr := os.Stat(desiredTarget)
	if srcErr == nil && os.SameFile(srcInfo, dstInfo) {
		return nil // already correct
	}
	if !force {
		return errors.New("file exists (use force: true to overwrite)")
	}
	return nil
}

// emitFileEvent publishes the appropriate event for the state and
// effect outcome. Only called in ModeApply. Matches the events the
// legacy Execute path emitted.
func emitFileEvent(ctx actions.Context, state string, eff actions.Effect, path string, mode os.FileMode, formatMode func(os.FileMode) string) {
	pub := ctx.GetEventPublisher()
	if pub == nil {
		return
	}

	switch state {
	case "directory":
		pub.Publish(events.Event{
			Type: events.EventDirCreated,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case actionTypeFile:
		eventType := events.EventFileCreated
		// Heuristic: if the Effect's Reason indicates an existing file
		// (contains "differs" or "chmod" or "match"), it's an update.
		// Cleaner long-term would be for Effect to carry a "was-existing"
		// bit, but this matches today's observable behavior.
		if eff.Reason != "" && !contains(eff.Reason, "would create") {
			eventType = events.EventFileUpdated
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case "absent":
		// Distinguish file vs dir for the event type.
		eventType := events.EventFileRemoved
		if eff.Reason != "" && contains(eff.Reason, "directory") {
			eventType = events.EventDirRemoved
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:    path,
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case "touch":
		pub.Publish(events.Event{
			Type: events.EventFileCreated,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case stateLink, stateHardlink:
		linkType := "symlink"
		if state == stateHardlink {
			linkType = stateHardlink
		}
		pub.Publish(events.Event{
			Type: events.EventLinkCreated,
			Data: events.LinkCreatedData{
				Src:    "", // path expansion happens earlier; Effect doesn't carry src today
				Dest:   path,
				Type:   linkType,
				DryRun: false,
			},
		})
	case "perms":
		pub.Publish(events.Event{
			Type: events.EventPermissionsChanged,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	}
}

// contains is a tiny strings.Contains replacement to avoid a dependency
// import for one call.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure path is treated; keep filepath import live for future use
// (parent-dir resolution may move into runState).
var _ = filepath.Dir
