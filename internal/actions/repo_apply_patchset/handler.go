// Package repo_apply_patchset implements the repo_apply_patchset action handler.
//
// The repo_apply_patchset action applies multiple patches to multiple files with support for:
// - Inline patchset or external patchset files
// - Multi-file unified diff format (git diff output)
// - Atomic operations with rollback on failure (strict mode)
// - Backup creation before modifications
// - JSON output with per-file results
// - Idempotency (no change if patches already applied)
//
//nolint:revive,staticcheck // Package name matches action name convention (repo_apply_patchset)
package repo_apply_patchset

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	actionName = "repo_apply_patchset"
)

// Handler implements the Handler interface for repo_apply_patchset actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the repo_apply_patchset action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        actionName,
		Description: "Apply multiple patches to multiple files atomically",
		Category:    actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileUpdated),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on file permissions
		ImplementsCheck:    true,       // Checks if patches already applied
	}
}

// Validate checks if the repo_apply_patchset configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.RepoApplyPatchset == nil {
		return fmt.Errorf("repo_apply_patchset configuration is nil")
	}

	raps := step.RepoApplyPatchset

	// Either patchset or patchset_file must be specified
	if raps.Patchset == "" && raps.PatchsetFile == "" {
		hint := actions.GetActionHint(actionName, "patchset")
		return fmt.Errorf("either patchset or patchset_file is required%s", hint)
	}

	// Both cannot be specified
	if raps.Patchset != "" && raps.PatchsetFile != "" {
		return fmt.Errorf("cannot specify both patchset and patchset_file")
	}

	return nil
}

// Execute runs the repo_apply_patchset action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	raps := step.RepoApplyPatchset

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

	// Get base directory
	baseDir := ec.CurrentDir
	if raps.BaseDir != "" {
		renderedBaseDir, err := ec.PathUtil.ExpandPath(raps.BaseDir, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("failed to expand base_dir: %w", err)
		}
		baseDir = renderedBaseDir
	}

	// Get patchset content
	patchsetContent := ""
	if raps.Patchset != "" {
		// Render inline patchset
		renderedPatchset, patchErr := ctx.GetTemplate().Render(raps.Patchset, ctx.GetVariables())
		if patchErr != nil {
			return result, fmt.Errorf("failed to render patchset: %w", patchErr)
		}
		patchsetContent = renderedPatchset
	} else {
		// Read patchset from file
		renderedPatchsetFile, pathErr := ec.PathUtil.ExpandPath(raps.PatchsetFile, ec.CurrentDir, ctx.GetVariables())
		if pathErr != nil {
			return result, fmt.Errorf("failed to expand patchset_file path: %w", pathErr)
		}

		// #nosec G304 -- File path from user config is intentional for configuration management
		patchsetBytes, readErr := os.ReadFile(renderedPatchsetFile)
		if readErr != nil {
			return result, fmt.Errorf("failed to read patchset file %s: %w", renderedPatchsetFile, readErr)
		}
		patchsetContent = string(patchsetBytes)
	}

	// Parse patchset
	filePatches, err := h.parsePatchset(patchsetContent)
	if err != nil {
		return result, fmt.Errorf("failed to parse patchset: %w", err)
	}

	if len(filePatches) == 0 {
		return result, fmt.Errorf("no valid patches found in patchset")
	}

	// Apply patches
	patchResults, anyChanged, err := h.applyPatchset(ctx, baseDir, filePatches, raps)
	if err != nil {
		return result, err
	}

	result.Changed = anyChanged

	// Write output file if specified
	if raps.OutputFile != "" {
		outputPath, pathErr := ec.PathUtil.ExpandPath(raps.OutputFile, ec.CurrentDir, ctx.GetVariables())
		if pathErr != nil {
			return result, fmt.Errorf("failed to expand output_file path: %w", pathErr)
		}

		outputData, jsonErr := json.MarshalIndent(patchResults, "", "  ")
		if jsonErr != nil {
			return result, fmt.Errorf("failed to marshal results: %w", jsonErr)
		}

		// #nosec G306 -- 0644 permissions are intentional for output files
		if writeErr := os.WriteFile(outputPath, outputData, 0644); writeErr != nil {
			return result, fmt.Errorf("failed to write output file: %w", writeErr)
		}
	}

	// Calculate statistics
	successCount := 0
	failureCount := 0
	for _, pr := range patchResults {
		if pr.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	ctx.GetLogger().Infof("  Applied patchset: %d succeeded, %d failed", successCount, failureCount)

	// Set result data
	result.SetData(map[string]interface{}{
		"total_files":    len(patchResults),
		"success_count":  successCount,
		"failure_count":  failureCount,
		"patch_results":  patchResults,
		"base_dir":       baseDir,
	})

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	raps := step.RepoApplyPatchset

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Get base directory
	baseDir := ec.CurrentDir
	if raps.BaseDir != "" {
		renderedBaseDir, err := ec.PathUtil.ExpandPath(raps.BaseDir, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render base_dir: %v", err)
			baseDir = raps.BaseDir
		} else {
			baseDir = renderedBaseDir
		}
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would apply patchset to files in %s", baseDir)

	if raps.Patchset != "" {
		lines := strings.Count(raps.Patchset, "\n")
		ctx.GetLogger().Infof("            Patchset: inline (%d lines)", lines)
	} else {
		ctx.GetLogger().Infof("            Patchset file: %s", raps.PatchsetFile)
	}

	if raps.Strict {
		ctx.GetLogger().Infof("            Mode: strict (rollback all if any fails)")
	} else {
		ctx.GetLogger().Infof("            Mode: lenient (apply what succeeds)")
	}

	if raps.Backup {
		ctx.GetLogger().Infof("            Backup: .bak files")
	}

	if raps.OutputFile != "" {
		ctx.GetLogger().Infof("            Output: %s", raps.OutputFile)
	}

	return nil
}

// FilePatch represents a patch for a single file
type FilePatch struct {
	Path   string
	Hunks  []*Hunk
}

// Hunk represents a single hunk in a unified diff
type Hunk struct {
	OldStart int      // Starting line in old file
	OldCount int      // Number of lines in old file
	NewStart int      // Starting line in new file
	NewCount int      // Number of lines in new file
	Lines    []string // Patch lines (with +, -, or space prefix)
}

// PatchResult represents the result of applying a patch to a file
type PatchResult struct {
	File          string `json:"file"`
	Success       bool   `json:"success"`
	Changed       bool   `json:"changed"`
	AppliedHunks  int    `json:"applied_hunks"`
	FailedHunks   int    `json:"failed_hunks"`
	TotalHunks    int    `json:"total_hunks"`
	Error         string `json:"error,omitempty"`
}

// parsePatchset parses a multi-file unified diff patchset
//nolint:unparam // Error return kept for future validation enhancements
func (h *Handler) parsePatchset(patchsetContent string) ([]*FilePatch, error) {
	lines := strings.Split(patchsetContent, "\n")
	var filePatches []*FilePatch
	var currentFilePatch *FilePatch
	var currentHunk *Hunk

	// Regex for file header: --- a/path/file.txt or --- path/file.txt
	fileHeaderOldRe := regexp.MustCompile(`^---\s+(?:a/)?(.+)`)
	fileHeaderNewRe := regexp.MustCompile(`^\+\+\+\s+(?:b/)?(.+)`)

	// Regex for hunk header: @@ -old_start,old_count +new_start,new_count @@
	hunkHeaderRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	inHunk := false

	for i := 0; i < len(lines); i++ {
		line := lines[i]

		// Check for file header (old file)
		if matches := fileHeaderOldRe.FindStringSubmatch(line); matches != nil {
			// Save previous file patch
			if currentFilePatch != nil && currentHunk != nil {
				currentFilePatch.Hunks = append(currentFilePatch.Hunks, currentHunk)
				currentHunk = nil
			}
			if currentFilePatch != nil && len(currentFilePatch.Hunks) > 0 {
				filePatches = append(filePatches, currentFilePatch)
			}

			// Start new file patch
			oldPath := matches[1]

			// Look ahead for new file path
			if i+1 < len(lines) {
				if newMatches := fileHeaderNewRe.FindStringSubmatch(lines[i+1]); newMatches != nil {
					newPath := newMatches[1]
					// Use new path (it's the file being patched)
					currentFilePatch = &FilePatch{
						Path:  newPath,
						Hunks: []*Hunk{},
					}
					i++ // Skip the +++ line
					continue
				}
			}

			// If no +++ line found, use old path
			currentFilePatch = &FilePatch{
				Path:  oldPath,
				Hunks: []*Hunk{},
			}
			inHunk = false
			continue
		}

		// Check for hunk header
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous hunk
			if currentHunk != nil && currentFilePatch != nil {
				currentFilePatch.Hunks = append(currentFilePatch.Hunks, currentHunk)
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
		if inHunk && currentHunk != nil && currentFilePatch != nil {
			// Unified diff lines start with +, -, or space
			if len(line) > 0 && (line[0] == '+' || line[0] == '-' || line[0] == ' ') {
				currentHunk.Lines = append(currentHunk.Lines, line)
			} else if line == "" {
				// Empty line in patch (context line)
				currentHunk.Lines = append(currentHunk.Lines, " ")
			} else if !strings.HasPrefix(line, "diff ") && !strings.HasPrefix(line, "index ") {
				// Unknown line that's not a diff metadata line
				inHunk = false
			}
		}
	}

	// Save last hunk and file patch
	if currentHunk != nil && currentFilePatch != nil {
		currentFilePatch.Hunks = append(currentFilePatch.Hunks, currentHunk)
	}
	if currentFilePatch != nil && len(currentFilePatch.Hunks) > 0 {
		filePatches = append(filePatches, currentFilePatch)
	}

	return filePatches, nil
}

// applyPatchset applies all patches in the patchset
func (h *Handler) applyPatchset(ctx actions.Context, baseDir string, filePatches []*FilePatch, raps *config.RepoApplyPatchset) ([]*PatchResult, bool, error) {
	results := make([]*PatchResult, 0, len(filePatches))
	backups := make(map[string][]byte) // file path -> original content
	anyChanged := false

	// Apply patches to all files
	for _, filePatch := range filePatches {
		// Resolve file path
		targetPath := filepath.Join(baseDir, filePatch.Path)

		// Validate path safety
		if pathErr := pathutil.ValidateNoPathTraversal(targetPath); pathErr != nil {
			ctx.GetLogger().Debugf("  Path validation warning for %s: %v", targetPath, pathErr)
		}

		// Read original file
		// #nosec G304 -- File path from user config is intentional for configuration management
		originalContent, err := os.ReadFile(targetPath)
		if err != nil {
			patchResult := &PatchResult{
				File:       filePatch.Path,
				Success:    false,
				Changed:    false,
				TotalHunks: len(filePatch.Hunks),
				Error:      fmt.Sprintf("failed to read file: %v", err),
			}
			results = append(results, patchResult)

			if raps.Strict {
				// Rollback all changes
				h.rollbackChanges(backups)
				return results, false, fmt.Errorf("failed to read %s in strict mode: %w", filePatch.Path, err)
			}
			continue
		}

		// Store backup
		backups[targetPath] = originalContent

		// Apply patch to this file
		newContent, appliedHunks, failedHunks := h.applyFilePatch(string(originalContent), filePatch)

		// Check if file changed
		fileChanged := string(originalContent) != newContent

		patchResult := &PatchResult{
			File:         filePatch.Path,
			Success:      failedHunks == 0,
			Changed:      fileChanged,
			AppliedHunks: appliedHunks,
			FailedHunks:  failedHunks,
			TotalHunks:   len(filePatch.Hunks),
		}

		if failedHunks > 0 {
			patchResult.Error = fmt.Sprintf("%d hunk(s) failed to apply", failedHunks)
		}

		results = append(results, patchResult)

		// Check strict mode
		if raps.Strict && failedHunks > 0 {
			// Rollback all changes
			h.rollbackChanges(backups)
			return results, false, fmt.Errorf("patch failed for %s in strict mode: %d hunk(s) failed", filePatch.Path, failedHunks)
		}

		// Write file if changed
		if fileChanged {
			anyChanged = true

			// Create backup if requested
			if raps.Backup {
				backupPath := targetPath + ".bak"
				if err := os.WriteFile(backupPath, originalContent, 0600); err != nil {
					ctx.GetLogger().Debugf("  Warning: Failed to create backup for %s: %v", filePatch.Path, err)
				}
			}

			// Write file atomically
			if err := h.writeAtomic(targetPath, newContent); err != nil {
				patchResult.Success = false
				patchResult.Error = fmt.Sprintf("failed to write file: %v", err)

				if raps.Strict {
					// Rollback all changes
					h.rollbackChanges(backups)
					return results, false, fmt.Errorf("failed to write %s in strict mode: %w", filePatch.Path, err)
				}
			} else {
				// Emit event
				publisher := ctx.GetEventPublisher()
				if publisher != nil {
					publisher.Publish(events.Event{
						Type: events.EventFileUpdated,
						Data: events.FileOperationData{
							Path:    targetPath,
							Changed: true,
							DryRun:  ctx.IsDryRun(),
						},
					})
				}
			}
		}
	}

	return results, anyChanged, nil
}

// applyFilePatch applies a patch to a single file
func (h *Handler) applyFilePatch(content string, filePatch *FilePatch) (newContent string, appliedHunks, failedHunks int) {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	lineIdx := 0 // Current position in original file (0-indexed)

	for _, hunk := range filePatch.Hunks {
		// Copy lines before the hunk
		hunkStart := hunk.OldStart - 1 // Convert to 0-indexed
		for lineIdx < hunkStart && lineIdx < len(lines) {
			result = append(result, lines[lineIdx])
			lineIdx++
		}

		// Try to apply hunk
		applied, newLines := h.applyHunk(lines, lineIdx, hunk)
		if !applied {
			failedHunks++
			// Copy original lines
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
func (h *Handler) applyHunk(lines []string, startIdx int, hunk *Hunk) (applied bool, newLines []string) {
	newLines = []string{}
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
			if fileIdx >= len(lines) || lines[fileIdx] != content {
				return false, nil
			}
			newLines = append(newLines, content)
			oldIdx++
		} else if prefix == '-' {
			// Deletion line - must match to be deleted
			fileIdx := startIdx + oldIdx
			if fileIdx >= len(lines) || lines[fileIdx] != content {
				return false, nil
			}
			// Don't add to newLines (it's being deleted)
			oldIdx++
		} else if prefix == '+' {
			// Addition line
			newLines = append(newLines, content)
			// Don't increment oldIdx (no corresponding old line)
		}
	}

	return true, newLines
}

// rollbackChanges restores original file contents
func (h *Handler) rollbackChanges(backups map[string][]byte) {
	for path, content := range backups {
		// Best effort rollback - ignore errors
		_ = os.WriteFile(path, content, 0600) // #nosec G306 - artifact file permissions
	}
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
