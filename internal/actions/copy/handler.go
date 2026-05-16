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
	"syscall"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
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
		Name:               "file.copy",
		Description:        "Copy files with checksum verification and atomic writes",
		Category:           actions.CategoryFile,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventFileCopied)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on dest path/ownership
		ImplementsCheck:    true,       // Uses checksums for idempotency
	}
}

// Permissions implements actions.Permitter (spec-22). Declares Sudo
// when Dest is under a known system root; populates FilesystemWrite=[Dest].
// No Network: file.copy reads a local source. RequiredBinaries unset.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.FileCopy == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.FileCopy.Dest) {
		ps.Sudo = true
	}
	if step.FileCopy.Dest != "" {
		ps.FilesystemWrite = []string{step.FileCopy.Dest}
	}
	return ps
}

// Validate checks if the copy configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileCopy == nil {
		return fmt.Errorf("copy configuration is nil")
	}

	copyAction := step.FileCopy
	if copyAction.Src == "" {
		hint := actions.GetActionHint("copy", "src")
		return fmt.Errorf("src is required%s", hint)
	}

	if copyAction.Dest == "" {
		hint := actions.GetActionHint("copy", "dest")
		return fmt.Errorf("dest is required%s", hint)
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
	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return fmt.Errorf("become not supported on %s", runtime.GOOS)
		}
		// Use sudo for final move
		cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			return fmt.Errorf("failed to move file with sudo: %w", err)
		}
	} else {
		if renameErr := os.Rename(tmpPath, dest); renameErr != nil {
			// Fallback for cross-device moves (e.g. btrfs subvolumes, tmpfs → home)
			if linkErr, ok := renameErr.(*os.LinkError); ok && linkErr.Err == syscall.EXDEV {
				if copyErr := crossDeviceCopy(tmpPath, dest, mode); copyErr != nil {
					return fmt.Errorf("failed to move file: %w", copyErr)
				}
			} else {
				return fmt.Errorf("failed to move file: %w", renameErr)
			}
		}
	}

	return nil
}

func (h *Handler) setOwnership(path, owner, group string, step *config.Step, ec *executor.ExecutionContext) error {
	if owner == "" && group == "" {
		return nil
	}

	if step.ShouldBecome() || runtime.GOOS != "linux" {
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

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
	// #nosec G204 - This is a provisioning tool designed to execute commands
	cmd := exec.Command("sudo", "-S", "sh", "-c", command)
	cmd.Stdin = bytes.NewBufferString(ec.Svc.SudoPass + "\n")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}

// crossDeviceCopy copies src to dst when os.Rename fails with EXDEV
// (source and destination are on different filesystems/subvolumes).
func crossDeviceCopy(src, dst string, mode os.FileMode) error {
	// #nosec G304 -- source path comes from validated config
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	// #nosec G304 -- destination path comes from validated config
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Sync(); err != nil {
		return err
	}
	return os.Remove(src)
}

// Run is the Spec 16 unified entry point. Reads the source file once,
// optionally verifies its checksum, then routes the write through
// Performer.WriteFile so plan-mode and execute-mode share the same
// idempotency decision (byte-for-byte content + mode match).
//
// Compared to the legacy Execute path:
//   - Idempotency uses content equality rather than size+mtime. This
//     is slower for large files but always correct (the legacy heuristic
//     could miss content-identical files with mismatched mtimes and
//     re-copy needlessly).
//   - Source mtime is no longer preserved on the destination. Nothing
//     else in the codebase depends on this; the freshness check that
//     used it is replaced by Performer's content comparison.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cp := step.FileCopy

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	src, err := ec.Svc.PathUtil.ExpandPath(cp.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}
	dest, err := ec.Svc.PathUtil.ExpandPath(cp.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	followSymlinks := cp.FollowSymlinks == nil || *cp.FollowSymlinks
	var srcInfo os.FileInfo
	if followSymlinks {
		srcInfo, err = os.Stat(src)
	} else {
		srcInfo, err = os.Lstat(src)
	}
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to stat source: %w", err)
	}
	if srcInfo.IsDir() {
		result.Failed = true
		return result, fmt.Errorf("src %q is a directory; mooncake's file.copy is single-file only. Use `shell: cp -r ...` to recurse, or copy each file with a `for_each_file:` loop", src)
	}

	// Symlink-source path: when follow_symlinks=false and source is a
	// symlink, create a symlink at dest pointing to the same target.
	if !followSymlinks && srcInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to read symlink target: %w", err)
		}
		if ctx.Mode() == actions.ModeApply {
			result.ReverseData = filehandler.CaptureReverseInfo(dest, "")
		}
		eff := ctx.Effects().Symlink(target, dest, actions.PerformerOpts{Become: step.ShouldBecome()})
		if eff.Err != nil {
			result.Failed = true
			return result, eff.Err
		}
		if ctx.Mode() == actions.ModePlan {
			result.WouldChange = eff.WouldChange
			result.Reason = eff.Reason
		} else {
			result.Changed = eff.Performed
		}
		return result, nil
	}

	// Source checksum verification (pre-copy). Failing here is a hard
	// error in both modes — it surfaces config drift early.
	if cp.Checksum != "" {
		ok, cerr := utils.VerifyChecksum(src, cp.Checksum)
		if cerr != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify source checksum: %w", cerr)
		}
		if !ok {
			result.Failed = true
			return result, fmt.Errorf("source checksum mismatch")
		}
	}

	// #nosec G304 — src path is user-supplied by design
	content, err := os.ReadFile(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read source: %w", err)
	}

	// Default to the source file's mode if the user didn't specify one.
	mode := h.parseFileMode(cp.Mode, srcInfo.Mode()&os.ModePerm)

	// Capture pre-state for Reverse() (spec-22 phase 5 slice D).
	// Apply mode only; plan mode doesn't mutate. Must precede the
	// Force-driven os.Remove below — otherwise the snapshot would
	// observe an empty path.
	if ctx.Mode() == actions.ModeApply {
		result.ReverseData = filehandler.CaptureReverseInfo(dest, "")
	}

	// Backup before overwrite — only in execute mode.
	if ctx.Mode() == actions.ModeApply && cp.Backup {
		if _, statErr := os.Stat(dest); statErr == nil {
			if backupPath, berr := utils.CreateBackup(dest); berr != nil {
				ctx.GetLogger().Debugf("  Warning: failed to create backup: %v", berr)
			} else {
				ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
			}
		}
	}

	// Force overrides idempotency — always counts as a change.
	if cp.Force {
		// Touch the dest to force WriteFile to overwrite even if
		// content matches. Cheapest: remove and let WriteFile create.
		if ctx.Mode() == actions.ModeApply {
			_ = os.Remove(dest)
		}
	}

	eff := ctx.Effects().WriteFile(dest, content, mode, actions.PerformerOpts{Become: step.ShouldBecome()})
	if eff.Err != nil {
		result.Failed = true
		return result, eff.Err
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = eff.WouldChange
		result.Reason = eff.Reason
		return result, nil
	}

	result.Changed = eff.Performed

	// Ownership after content is in place.
	if cp.Owner != "" || cp.Group != "" {
		own := ctx.Effects().Chown(dest, cp.Owner, cp.Group, actions.PerformerOpts{Become: step.ShouldBecome()})
		if own.Err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to set ownership: %w", own.Err)
		}
		if own.Performed {
			result.Changed = true
		}
	}

	// Post-copy checksum verification.
	if cp.Checksum != "" {
		ok, verr := utils.VerifyChecksum(dest, cp.Checksum)
		if verr != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify destination checksum: %w", verr)
		}
		if !ok {
			result.Failed = true
			return result, fmt.Errorf("destination checksum mismatch after copy")
		}
	}

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileCopied,
			Data: events.FileCopiedData{
				Src:       src,
				Dest:      dest,
				SizeBytes: srcInfo.Size(),
				Mode:      mode.String(),
				Checksum:  cp.Checksum,
				DryRun:    false,
			},
		})
	}

	return result, nil
}
