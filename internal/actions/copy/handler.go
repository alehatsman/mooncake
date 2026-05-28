// Package copy implements the copy action handler.
//
// The copy action copies files from source to destination with:
// - Checksum verification (before and after copy)
// - Atomic write pattern (temp file + rename)
// - Backup support
// - Idempotency based on size/modtime
package copy

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
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
		Examples: []string{
			`# Copy a config file with explicit mode + owner
- name: Drop the systemd unit
  file.copy:
    src: ./files/myapp.service
    dest: /etc/systemd/system/myapp.service
    mode: "0644"
    owner: root
    group: root
  become: true`,
		},
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
//
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cp := step.FileCopy

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	src, err := ec.Svc.PathUtil.ExpandPath(cp.Src, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}
	dest, err := ec.Svc.PathUtil.ExpandPath(cp.Dest, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.Operation = executor.OpUpdate
	result.Target = dest
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		if !result.Changed && !result.WouldChange && !result.Failed {
			result.Operation = executor.OpNoop
		}
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
		eff := ctx.Effects().Symlink(target, dest, actions.PerformerOpts{})
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

	// F026: source bytes are no longer read into a single []byte. The
	// Performer.CopyFile primitive streams src → dest via io.Copy, so a
	// 10 GB copy on a 256 MB-RAM container no longer OOMs the daemon.
	// Pre-fix this site did os.ReadFile(src) + Effects.WriteFile(dest,
	// content), holding the entire source in memory for the duration of
	// the write (~2× the file size between handler and WriteFile's own
	// existing-content check).

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
				ctx.Logger().Debugf("  Warning: failed to create backup: %v", berr)
			} else {
				ctx.Logger().Debugf("  Created backup: %s", backupPath)
			}
		}
	}

	// Force overrides idempotency — always counts as a change.
	if cp.Force {
		// Touch the dest to force CopyFile to overwrite even if
		// content matches. Cheapest: remove and let CopyFile create.
		if ctx.Mode() == actions.ModeApply {
			_ = os.Remove(dest)
		}
	}

	eff := ctx.Effects().CopyFile(src, dest, mode, actions.PerformerOpts{
		ExplicitMode: cp.Mode != "",
	})
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
		own := ctx.Effects().Chown(dest, cp.Owner, cp.Group, actions.PerformerOpts{})
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

	if pub := ctx.EventPublisher(); pub != nil {
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
