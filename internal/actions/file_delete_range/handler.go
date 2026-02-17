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
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	actionName = "file_delete_range"
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
		Name:        actionName,
		Description: "Delete text between start and end anchor patterns in files",
		Category:    actions.CategoryFile,
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

// Validate checks if the file_delete_range configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileDeleteRange == nil {
		return fmt.Errorf("file_delete_range configuration is nil")
	}

	fdr := step.FileDeleteRange

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
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fdr := step.FileDeleteRange

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Expand and render path
	renderedPath, err := ec.PathUtil.ExpandPath(fdr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path safety
	if pathErr := pathutil.ValidateNoPathTraversal(renderedPath); pathErr != nil {
		ctx.GetLogger().Debugf("  Path validation warning: %v", pathErr)
	}

	// Read file content
	// #nosec G304 -- File path from user config is intentional for configuration management
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	// Render anchors (support template variables)
	renderedStartAnchor, err := ctx.GetTemplate().Render(fdr.StartAnchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render start_anchor: %w", err)
	}

	renderedEndAnchor, err := ctx.GetTemplate().Render(fdr.EndAnchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render end_anchor: %w", err)
	}

	// Perform deletion
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

	// Check if changes were made
	if string(originalContent) == newContent {
		result.Changed = false
		ctx.GetLogger().Debugf("  No changes needed (range not found or already deleted)")
		return result, nil
	}

	// Create backup if requested
	if fdr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
		ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
	}

	// Write file atomically
	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Deleted %d line(s) from %s", deletedLines, renderedPath)

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{
				Path:    renderedPath,
				Changed: result.Changed,
				DryRun:  ctx.IsDryRun(),
			},
		})
	}

	// Set result data
	result.SetData(map[string]interface{}{
		"path":          renderedPath,
		"deleted_lines": deletedLines,
		"start_anchor":  renderedStartAnchor,
		"end_anchor":    renderedEndAnchor,
	})

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	fdr := step.FileDeleteRange

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	renderedPath, err := ec.PathUtil.ExpandPath(fdr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = fdr.Path
	}

	// Render anchors
	renderedStartAnchor, err := ctx.GetTemplate().Render(fdr.StartAnchor, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render start_anchor: %v", err)
		renderedStartAnchor = fdr.StartAnchor
	}

	renderedEndAnchor, err := ctx.GetTemplate().Render(fdr.EndAnchor, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render end_anchor: %v", err)
		renderedEndAnchor = fdr.EndAnchor
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would delete range between '%s' and '%s' in %s",
		renderedStartAnchor, renderedEndAnchor, renderedPath)

	if fdr.Regex {
		ctx.GetLogger().Infof("            Mode: regex")
	} else {
		ctx.GetLogger().Infof("            Mode: literal")
	}

	if fdr.Inclusive {
		ctx.GetLogger().Infof("            Include anchor lines: yes")
	} else {
		ctx.GetLogger().Infof("            Include anchor lines: no")
	}

	if fdr.Backup {
		ctx.GetLogger().Infof("            Backup: %s.bak", renderedPath)
	}

	return nil
}

// performDeletion performs the actual range deletion
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

	if !startFound {
		return "", 0, fmt.Errorf("start anchor not found: %s", startAnchor)
	}

	if !endFound {
		return "", 0, fmt.Errorf("end anchor not found: %s", endAnchor)
	}

	newContent = strings.Join(result, "\n")
	return newContent, deletedLines, nil
}

// writeAtomic writes content to file using atomic write pattern (temp file + rename)
func (h *Handler) writeAtomic(path, content string) error {
	// Write to temp file first
	tmpFile := path + ".tmp"
	// #nosec G306 -- 0644 permissions are intentional for user-editable config files
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
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
