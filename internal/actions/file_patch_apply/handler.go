// Package file_patch_apply implements the file_patch_apply action handler.
//
// The file_patch_apply action applies unified diff patches to files with support for:
// - Inline patch content or external patch files
// - Context line validation
// - Strict mode (fail on any hunk failure)
// - Atomic writes (temp file + rename)
// - Backup creation before modification
// - Idempotency (no change if patch already applied)
//
//nolint:revive,staticcheck // Package name matches action name convention (file_patch_apply)
package file_patch_apply

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	actionName = "file_patch_apply"
)

// Handler implements the Handler interface for file_patch_apply actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the file_patch_apply action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        actionName,
		Description: "Apply unified diff patches to files",
		Category:    actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileUpdated),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on file permissions
		ImplementsCheck:    true,       // Checks if patch already applied
	}
}

// Validate checks if the file_patch_apply configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FilePatchApply == nil {
		return fmt.Errorf("file_patch_apply configuration is nil")
	}

	fpa := step.FilePatchApply

	if fpa.Path == "" {
		hint := actions.GetActionHint(actionName, "path")
		return fmt.Errorf("path is required%s", hint)
	}

	// Either patch or patch_file must be specified
	if fpa.Patch == "" && fpa.PatchFile == "" {
		hint := actions.GetActionHint(actionName, "patch")
		return fmt.Errorf("either patch or patch_file is required%s", hint)
	}

	// Both cannot be specified
	if fpa.Patch != "" && fpa.PatchFile != "" {
		return fmt.Errorf("cannot specify both patch and patch_file")
	}

	// Validate context_lines if specified
	if fpa.ContextLines != nil && *fpa.ContextLines < 0 {
		return fmt.Errorf("context_lines must be >= 0, got %d", *fpa.ContextLines)
	}

	return nil
}

// Execute runs the file_patch_apply action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fpa := step.FilePatchApply

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
	renderedPath, err := ec.PathUtil.ExpandPath(fpa.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path safety
	if pathErr := pathutil.ValidateNoPathTraversal(renderedPath); pathErr != nil {
		ctx.GetLogger().Debugf("  Path validation warning: %v", pathErr)
	}

	// Get patch content
	patchContent := ""
	if fpa.Patch != "" {
		// Render inline patch
		renderedPatch, patchErr := ctx.GetTemplate().Render(fpa.Patch, ctx.GetVariables())
		if patchErr != nil {
			return result, fmt.Errorf("failed to render patch: %w", patchErr)
		}
		patchContent = renderedPatch
	} else {
		// Read patch from file
		renderedPatchFile, pathErr := ec.PathUtil.ExpandPath(fpa.PatchFile, ec.CurrentDir, ctx.GetVariables())
		if pathErr != nil {
			return result, fmt.Errorf("failed to expand patch_file path: %w", pathErr)
		}

		// #nosec G304 -- File path from user config is intentional for configuration management
		patchBytes, readErr := os.ReadFile(renderedPatchFile)
		if readErr != nil {
			return result, fmt.Errorf("failed to read patch file %s: %w", renderedPatchFile, readErr)
		}
		patchContent = string(patchBytes)
	}

	// Read target file content
	// #nosec G304 -- File path from user config is intentional for configuration management
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	// Parse patch
	patch, err := h.parsePatch(patchContent)
	if err != nil {
		return result, fmt.Errorf("failed to parse patch: %w", err)
	}

	// Get context lines requirement
	contextLines := 3 // default
	if fpa.ContextLines != nil {
		contextLines = *fpa.ContextLines
	}

	// Apply patch
	newContent, appliedHunks, failedHunks := h.applyPatch(
		string(originalContent),
		patch,
		contextLines,
	)

	// Check strict mode
	if fpa.Strict && failedHunks > 0 {
		return result, fmt.Errorf("patch application failed: %d hunk(s) failed in strict mode", failedHunks)
	}

	// Check if changes were made
	if string(originalContent) == newContent {
		result.Changed = false
		ctx.GetLogger().Debugf("  No changes needed (patch already applied)")
		return result, nil
	}

	// Create backup if requested
	if fpa.Backup {
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
	ctx.GetLogger().Infof("  Applied patch to %s (%d hunks succeeded, %d failed)", renderedPath, appliedHunks, failedHunks)

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
		"applied_hunks": appliedHunks,
		"failed_hunks":  failedHunks,
		"total_hunks":   len(patch.Hunks),
	})

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	fpa := step.FilePatchApply

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	renderedPath, err := ec.PathUtil.ExpandPath(fpa.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = fpa.Path
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would apply patch to %s", renderedPath)

	if fpa.Patch != "" {
		lines := strings.Count(fpa.Patch, "\n")
		ctx.GetLogger().Infof("            Patch: inline (%d lines)", lines)
	} else {
		ctx.GetLogger().Infof("            Patch file: %s", fpa.PatchFile)
	}

	contextLines := 3
	if fpa.ContextLines != nil {
		contextLines = *fpa.ContextLines
	}
	ctx.GetLogger().Infof("            Context lines: %d", contextLines)

	if fpa.Strict {
		ctx.GetLogger().Infof("            Mode: strict (fail if any hunk fails)")
	} else {
		ctx.GetLogger().Infof("            Mode: lenient (continue on hunk failures)")
	}

	if fpa.Backup {
		ctx.GetLogger().Infof("            Backup: %s.bak", renderedPath)
	}

	return nil
}

// Patch represents a parsed unified diff patch
type Patch struct {
	Hunks []*Hunk
}

// Hunk represents a single hunk in a unified diff
type Hunk struct {
	OldStart int      // Starting line in old file
	OldCount int      // Number of lines in old file
	NewStart int      // Starting line in new file
	NewCount int      // Number of lines in new file
	Lines    []string // Patch lines (with +, -, or space prefix)
}

// parsePatch parses a unified diff patch
func (h *Handler) parsePatch(patchContent string) (*Patch, error) {
	lines := strings.Split(patchContent, "\n")
	patch := &Patch{}

	// Regex for hunk header: @@ -old_start,old_count +new_start,new_count @@
	hunkHeaderRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	var currentHunk *Hunk
	inHunk := false

	for _, line := range lines {
		// Check for hunk header
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous hunk
			if currentHunk != nil {
				patch.Hunks = append(patch.Hunks, currentHunk)
			}

			// Parse hunk header
			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			currentHunk = &Hunk{
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
				Lines:    []string{},
			}
			inHunk = true
			continue
		}

		// Add lines to current hunk
		if inHunk && currentHunk != nil {
			// Unified diff lines start with +, -, or space
			if len(line) > 0 && (line[0] == '+' || line[0] == '-' || line[0] == ' ') {
				currentHunk.Lines = append(currentHunk.Lines, line)
			} else if line == "" {
				// Empty line in patch (context line)
				currentHunk.Lines = append(currentHunk.Lines, " ")
			}
		}
	}

	// Save last hunk
	if currentHunk != nil {
		patch.Hunks = append(patch.Hunks, currentHunk)
	}

	if len(patch.Hunks) == 0 {
		return nil, fmt.Errorf("no valid hunks found in patch")
	}

	return patch, nil
}

// applyPatch applies a patch to file content
func (h *Handler) applyPatch(content string, patch *Patch, minContextLines int) (newContent string, appliedHunks, failedHunks int) {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	lineIdx := 0 // Current position in original file (0-indexed)

	for _, hunk := range patch.Hunks {
		// Copy lines before the hunk
		hunkStart := hunk.OldStart - 1 // Convert to 0-indexed
		for lineIdx < hunkStart && lineIdx < len(lines) {
			result = append(result, lines[lineIdx])
			lineIdx++
		}

		// Try to apply hunk
		applied, newLines, err := h.applyHunk(lines, lineIdx, hunk, minContextLines)
		if err != nil || !applied {
			failedHunks++
			// In non-strict mode, copy original lines
			for i := 0; i < hunk.OldCount && lineIdx < len(lines); i++ {
				result = append(result, lines[lineIdx])
				lineIdx++
			}
			continue
		}

		// Hunk applied successfully
		appliedHunks++
		result = append(result, newLines...)
		lineIdx += hunk.OldCount
	}

	// Copy remaining lines
	for lineIdx < len(lines) {
		result = append(result, lines[lineIdx])
		lineIdx++
	}

	newContent = strings.Join(result, "\n")
	return newContent, appliedHunks, failedHunks
}

// applyHunk attempts to apply a single hunk
func (h *Handler) applyHunk(lines []string, startIdx int, hunk *Hunk, minContextLines int) (applied bool, newLines []string, err error) {
	newLines = []string{}

	// Validate context lines
	contextMatches := 0
	oldIdx := 0

	for _, patchLine := range hunk.Lines {
		if len(patchLine) == 0 {
			continue
		}

		prefix := patchLine[0]
		content := patchLine[1:]

		if prefix == ' ' {
			// Context line - must match
			fileIdx := startIdx + oldIdx
			if fileIdx >= len(lines) {
				return false, nil, fmt.Errorf("context line beyond file end")
			}

			if lines[fileIdx] == content {
				contextMatches++
				newLines = append(newLines, content)
				oldIdx++
		} else if contextMatches < minContextLines {
			// Context mismatch
			return false, nil, fmt.Errorf("insufficient context matches")
		}
		} else if prefix == '-' {
			// Deletion line - must match to be deleted
			fileIdx := startIdx + oldIdx
			if fileIdx >= len(lines) {
				return false, nil, fmt.Errorf("deletion line beyond file end")
			}

			if lines[fileIdx] != content {
				return false, nil, fmt.Errorf("deletion line mismatch")
			}
			// Don't add to newLines (it's being deleted)
			oldIdx++
		} else if prefix == '+' {
			// Addition line
			newLines = append(newLines, content)
			// Don't increment oldIdx (no corresponding old line)
		}
	}

	return true, newLines, nil
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
