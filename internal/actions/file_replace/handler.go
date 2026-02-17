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
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	actionName = "file_replace"
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
		Name:        actionName,
		Description: "Replace text in files using literal or regex patterns",
		Category:    actions.CategoryFile,
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

// Validate checks if the file_replace configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileReplace == nil {
		return fmt.Errorf("file_replace configuration is nil")
	}

	fr := step.FileReplace

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
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fr := step.FileReplace

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
	renderedPath, err := ec.PathUtil.ExpandPath(fr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path safety (no traversal outside working dir)
	if pathErr := pathutil.ValidateNoPathTraversal(renderedPath); pathErr != nil {
		ctx.GetLogger().Debugf("  Path validation warning: %v", pathErr)
	}

	// Read file content
	// #nosec G304 -- File path from user config is intentional for configuration management
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	// Render pattern and replacement (support template variables)
	renderedPattern, err := ctx.GetTemplate().Render(fr.Pattern, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render pattern: %w", err)
	}

	renderedReplace, err := ctx.GetTemplate().Render(fr.Replace, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render replacement: %w", err)
	}

	// Perform replacement
	newContent, replacementCount, err := h.performReplace(string(originalContent), renderedPattern, renderedReplace, fr)
	if err != nil {
		return result, err
	}

	// Check if changes were made
	if originalContent := string(originalContent); originalContent == newContent {
		if !fr.AllowNoMatch && replacementCount == 0 {
			return result, fmt.Errorf("no matches found for pattern: %s", renderedPattern)
		}
		result.Changed = false
		ctx.GetLogger().Debugf("  No changes needed (pattern not found or already replaced)")
		return result, nil
	}

	// Create backup if requested
	if fr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
		ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
	}

	// Write file atomically (temp file + rename)
	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Replaced %d occurrence(s) in %s", replacementCount, renderedPath)

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
		"path":         renderedPath,
		"replacements": replacementCount,
		"pattern":      renderedPattern,
	})

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	fr := step.FileReplace

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	renderedPath, err := ec.PathUtil.ExpandPath(fr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = fr.Path
	}

	// Render pattern and replacement (best effort)
	renderedPattern, err := ctx.GetTemplate().Render(fr.Pattern, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render pattern: %v", err)
		renderedPattern = fr.Pattern
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would replace pattern '%s' in %s", renderedPattern, renderedPath)

	if fr.Count != nil {
		ctx.GetLogger().Infof("            Max replacements: %d", *fr.Count)
	} else {
		ctx.GetLogger().Infof("            Replacements: all occurrences")
	}

	if fr.Backup {
		ctx.GetLogger().Infof("            Backup: %s.bak", renderedPath)
	}

	// Show flags if set
	if fr.Flags != nil {
		var flagsDesc []string
		if !fr.Flags.Regex {
			flagsDesc = append(flagsDesc, "literal")
		} else {
			flagsDesc = append(flagsDesc, "regex")
		}
		if fr.Flags.Multiline {
			flagsDesc = append(flagsDesc, "multiline")
		}
		if fr.Flags.CaseInsensitive {
			flagsDesc = append(flagsDesc, "case-insensitive")
		}
		if len(flagsDesc) > 0 {
			ctx.GetLogger().Infof("            Flags: %s", strings.Join(flagsDesc, ", "))
		}
	}

	return nil
}

// performReplace performs the actual text replacement
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
