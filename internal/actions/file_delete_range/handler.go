// Package file_delete_range implements the file_delete_range action handler.
//
// The file_delete_range action deletes text between two anchor patterns with support for:
// - Literal and regex-based anchor matching
// - Inclusive or exclusive deletion (include/exclude anchor lines)
// - Atomic writes (temp file + rename)
// - Backup creation before modification
// - Idempotency (no change if range not found)
//
//nolint:revive,staticcheck // Package name matches action name convention (file_delete_range)
package file_delete_range

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName = "text.delete_range"
)

// Handler implements the Handler interface for file_delete_range actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the file_delete_range action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           actionName,
		Description:    "Delete text between start and end anchor patterns in files",
		Category:       actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileUpdated),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on file permissions
		ImplementsCheck:    true,       // Checks if deletion needed before modifying
	}
}

// Permissions implements actions.Permitter (spec-22). text.delete_range
// deletes content between two anchors in a file; Sudo when Path is
// under a known system root. FilesystemWrite=[Path]. No Network; no
// RequiredBinaries.
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.TextDeleteRange == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.TextDeleteRange.Path) {
		ps.Sudo = true
	}
	if step.TextDeleteRange.Path != "" {
		ps.FilesystemWrite = []string{step.TextDeleteRange.Path}
	}
	return ps
}

// Validate checks if the file_delete_range configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.TextDeleteRange == nil {
		return fmt.Errorf("file_delete_range configuration is nil")
	}

	fdr := step.TextDeleteRange

	if fdr.Path == "" {
		hint := actions.GetActionHint(actionName, "path")
		return fmt.Errorf("path is required%s", hint)
	}

	if fdr.StartAnchor == "" {
		hint := actions.GetActionHint(actionName, "start_anchor")
		return fmt.Errorf("start_anchor is required%s", hint)
	}

	if fdr.EndAnchor == "" {
		hint := actions.GetActionHint(actionName, "end_anchor")
		return fmt.Errorf("end_anchor is required%s", hint)
	}

	// Validate regex if regex mode enabled
	if fdr.Regex {
		if _, err := regexp.Compile(fdr.StartAnchor); err != nil {
			return fmt.Errorf("invalid regex start_anchor '%s': %w", fdr.StartAnchor, err)
		}
		if _, err := regexp.Compile(fdr.EndAnchor); err != nil {
			return fmt.Errorf("invalid regex end_anchor '%s': %w", fdr.EndAnchor, err)
		}
	}

	return nil
}

// Execute runs the file_delete_range action.
func (h *Handler) performDeletion(content, startAnchor, endAnchor string, useRegex, inclusive bool) (newContent string, deletedLines int, err error) {
	lines := strings.Split(content, "\n")
	var result []string
	inRange := false
	startFound := false
	endFound := false

	for _, line := range lines {
		startMatched := false
		endMatched := false

		if useRegex {
			// Regex matching
			if startRe, compileErr := regexp.Compile(startAnchor); compileErr == nil {
				startMatched = startRe.MatchString(line)
			}
			if endRe, compileErr := regexp.Compile(endAnchor); compileErr == nil {
				endMatched = endRe.MatchString(line)
			}
		} else {
			// Literal matching
			startMatched = strings.Contains(line, startAnchor)
			endMatched = strings.Contains(line, endAnchor)
		}

		if startMatched && !inRange {
			// Found start anchor
			startFound = true
			inRange = true
			if !inclusive {
				// Keep start anchor line
				result = append(result, line)
			} else {
				// Delete start anchor line
				deletedLines++
			}
			continue
		}

		if endMatched && inRange {
			// Found end anchor
			endFound = true
			inRange = false
			if !inclusive {
				// Keep end anchor line
				result = append(result, line)
			} else {
				// Delete end anchor line
				deletedLines++
			}
			continue
		}

		if inRange {
			// Inside range, delete this line
			deletedLines++
		} else {
			// Outside range, keep this line
			result = append(result, line)
		}
	}

	// MT-47: missing anchors are idempotent success — a playbook that
	// deleted a range once won't find the anchors on re-run (the
	// content was removed). Return the *original* content unchanged
	// when either anchor is missing; the caller's content-equality
	// check turns this into the "no changes needed" branch. Note:
	// the partial `result` slice cannot be reused when start was
	// found but end wasn't — that path skipped lines that should be
	// preserved.
	if !startFound || !endFound {
		return content, 0, nil
	}

	newContent = strings.Join(result, "\n")
	return newContent, deletedLines, nil
}

// writeAtomic writes content to file using atomic write pattern (temp file + rename).
// mode is the permission set applied to the temp file so the rename preserves
// the original file's mode instead of clobbering it to 0644.
func (h *Handler) writeAtomic(path, content string, mode os.FileMode) error {
	// Write to temp file first
	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(content), mode); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpFile, path); err != nil {
		// Cleanup temp file on error
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// Run is the Spec 16 unified entry point. Computes the post-deletion
// content in memory; plan mode reports the prediction, execute mode
// commits the change atomically.
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fdr := step.TextDeleteRange

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true
	result.Operation = executor.OpUpdate
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		if !result.Changed && !result.WouldChange && !result.Failed {
			result.Operation = executor.OpNoop
		}
	}()

	renderedPath, err := ec.Svc.PathUtil.ExpandPath(fdr.Path, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}
	result.Target = renderedPath
	// F033: dead-code traversal check removed (see text_patch_ini).

	// #nosec G304 -- file path from user config
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}
	// Capture the original mode so the atomic write preserves it rather
	// than clobbering the file to 0644 (e.g. a 0600 secret).
	origInfo, err := os.Stat(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to stat file %s: %w", renderedPath, err)
	}
	origMode := origInfo.Mode().Perm()

	renderedStartAnchor, err := ctx.Template().Render(fdr.StartAnchor, ctx.Variables())
	if err != nil {
		return result, fmt.Errorf("failed to render start_anchor: %w", err)
	}
	renderedEndAnchor, err := ctx.Template().Render(fdr.EndAnchor, ctx.Variables())
	if err != nil {
		return result, fmt.Errorf("failed to render end_anchor: %w", err)
	}

	newContent, deletedLines, err := h.performDeletion(
		string(originalContent),
		renderedStartAnchor,
		renderedEndAnchor,
		fdr.Regex,
		fdr.Inclusive,
	)
	if err != nil {
		return result, err
	}

	if string(originalContent) == newContent {
		result.Reason = "range not found or already deleted"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would delete %d line(s)", deletedLines)
		return result, nil
	}

	// Capture pre-state for Reverse() (spec-22 phase 5 slice E).
	result.ReverseData = filehandler.CaptureReverseInfo(renderedPath, "")

	if fdr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent, origMode); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.Logger().Infof("  Deleted %d line(s) from %s", deletedLines, renderedPath)

	if pub := ctx.EventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{
				Path:    renderedPath,
				Changed: true,
				DryRun:  false,
			},
		})
	}

	result.SetData(map[string]interface{}{
		"path":          renderedPath,
		"deleted_lines": deletedLines,
		"start_anchor":  renderedStartAnchor,
		"end_anchor":    renderedEndAnchor,
	})
	return result, nil
}
