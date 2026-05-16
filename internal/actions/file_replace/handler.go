// Package file_replace implements the file_replace action handler.
//
// The file_replace action performs in-place text replacement in files with support for:
// - Literal and regex-based pattern matching
// - Limited or unlimited replacements (count parameter)
// - Multiline and case-insensitive modes
// - Atomic writes (temp file + rename)
// - Backup creation before modification
// - Idempotency (no change if pattern doesn't match or already replaced)
//
//nolint:revive // Package name matches action name convention (file_replace)
package file_replace

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
	actionName = "text.replace"
)

// Handler implements the Handler interface for file_replace actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the file_replace action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           actionName,
		Description:    "Replace text in files using literal or regex patterns",
		Category:       actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileUpdated),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on file permissions
		ImplementsCheck:    true,       // Checks if replacement needed before modifying
	}
}

// Permissions implements actions.Permitter (spec-22). text.replace
// edits a file in place; Sudo when Path falls under a known system
// root. FilesystemWrite=[Path]. No Network; no RequiredBinaries
// (regex replacement is in-process).
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.TextReplace == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.TextReplace.Path) {
		ps.Sudo = true
	}
	if step.TextReplace.Path != "" {
		ps.FilesystemWrite = []string{step.TextReplace.Path}
	}
	return ps
}

// Validate checks if the file_replace configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.TextReplace == nil {
		return fmt.Errorf("file_replace configuration is nil")
	}

	fr := step.TextReplace

	if fr.Path == "" {
		hint := actions.GetActionHint(actionName, "path")
		return fmt.Errorf("path is required%s", hint)
	}

	if fr.Pattern == "" {
		hint := actions.GetActionHint(actionName, "pattern")
		return fmt.Errorf("pattern is required%s", hint)
	}

	// Note: Replace can be empty string (delete pattern)

	// Validate regex if regex mode enabled
	if fr.Flags != nil && fr.Flags.Regex {
		_, err := regexp.Compile(fr.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", fr.Pattern, err)
		}
	}

	// Validate count is positive if specified
	if fr.Count != nil && *fr.Count <= 0 {
		return fmt.Errorf("count must be positive, got %d", *fr.Count)
	}

	return nil
}

// Execute runs the file_replace action.
func (h *Handler) performReplace(content, pattern, replacement string, fr *config.FileReplace) (newContent string, count int, err error) {
	// Determine replacement count limit (-1 = all)
	countLimit := -1
	if fr.Count != nil {
		countLimit = *fr.Count
	}

	// Determine if regex mode is enabled (default: true)
	useRegex := true
	if fr.Flags != nil {
		useRegex = fr.Flags.Regex
	}

	// Literal replacement mode
	if !useRegex {
		if countLimit == -1 {
			// Replace all
			newContent = strings.ReplaceAll(content, pattern, replacement)
			count = strings.Count(content, pattern)
		} else {
			// Replace up to count
			newContent = strings.Replace(content, pattern, replacement, countLimit)
			before := strings.Count(content, pattern)
			after := strings.Count(newContent, pattern)
			count = before - after
		}
		return newContent, count, nil
	}

	// Regex replacement mode
	regexFlags := ""
	if fr.Flags != nil {
		if fr.Flags.CaseInsensitive {
			regexFlags += "(?i)"
		}
		if fr.Flags.Multiline {
			regexFlags += "(?m)"
		}
	}

	// Compile regex with flags
	re, err := regexp.Compile(regexFlags + pattern)
	if err != nil {
		return "", 0, fmt.Errorf("invalid regex pattern: %w", err)
	}

	// Perform replacement
	if countLimit == -1 {
		// Replace all matches
		newContent = re.ReplaceAllStringFunc(content, func(_ string) string {
			count++
			return replacement
		})
	} else {
		// Replace up to countLimit matches
		newContent = re.ReplaceAllStringFunc(content, func(match string) string {
			if count < countLimit {
				count++
				return replacement
			}
			return match
		})
	}

	return newContent, count, nil
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

// Run is the Spec 16 unified entry point. Performs the regex replacement
// in memory once and either reports the predicted change (ModePlan) or
// commits it atomically (ModeApply). Drift between plan preview and
// real execution is impossible because both modes compute the same
// `newContent` and compare against the same on-disk content.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fr := step.TextReplace

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	renderedPath, err := ec.Svc.PathUtil.ExpandPath(fr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}
	// F033: dead-code traversal check removed (see text_patch_ini).

	// #nosec G304 -- file path from user config
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	renderedPattern, err := ctx.GetTemplate().Render(fr.Pattern, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render pattern: %w", err)
	}
	renderedReplace, err := ctx.GetTemplate().Render(fr.Replace, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render replacement: %w", err)
	}

	newContent, replacementCount, err := h.performReplace(string(originalContent), renderedPattern, renderedReplace, fr)
	if err != nil {
		return result, err
	}

	// No-change short-circuit shared by both modes. No-match is
	// idempotent success — see MT-47.
	if string(originalContent) == newContent {
		_ = replacementCount
		result.Reason = "pattern not present or already replaced"
		return result, nil
	}

	// Change detected. Plan mode reports the prediction; execute mode
	// performs the atomic write and event emission.
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would replace %d occurrence(s)", replacementCount)
		return result, nil
	}

	// Capture pre-state for Reverse() (spec-22 phase 5 slice E).
	result.ReverseData = filehandler.CaptureReverseInfo(renderedPath, "")

	if fr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Replaced %d occurrence(s) in %s", replacementCount, renderedPath)

	if pub := ctx.GetEventPublisher(); pub != nil {
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
		"path":         renderedPath,
		"replacements": replacementCount,
		"pattern":      renderedPattern,
	})
	return result, nil
}
