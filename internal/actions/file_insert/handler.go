// Package file_insert implements the file_insert action handler.
//
// The file_insert action inserts text before or after an anchor pattern with support for:
// - Literal and regex-based anchor matching
// - Before/after insertion positioning
// - Single or multiple anchor matches
// - Atomic writes (temp file + rename)
// - Backup creation before modification
// - Idempotency (no change if already inserted)
//
//nolint:revive,staticcheck // Package name matches action name convention (file_insert)
package file_insert

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
	actionName     = "file_insert"
	positionBefore = "before"
	positionAfter  = "after"
)

// Handler implements the Handler interface for file_insert actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the file_insert action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        actionName,
		Description: "Insert text before or after anchor patterns in files",
		Category:    actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileUpdated),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on file permissions
		ImplementsCheck:    true,       // Checks if insertion needed before modifying
	}
}

// Validate checks if the file_insert configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileInsert == nil {
		return fmt.Errorf("file_insert configuration is nil")
	}

	fi := step.FileInsert

	if fi.Path == "" {
		hint := actions.GetActionHint(actionName, "path")
		return fmt.Errorf("path is required%s", hint)
	}

	if fi.Anchor == "" {
		hint := actions.GetActionHint(actionName, "anchor")
		return fmt.Errorf("anchor is required%s", hint)
	}

	if fi.Position == "" {
		hint := actions.GetActionHint(actionName, "position")
		return fmt.Errorf("position is required%s", hint)
	}

	if fi.Position != positionBefore && fi.Position != positionAfter {
		return fmt.Errorf("position must be 'before' or 'after', got '%s'", fi.Position)
	}

	if fi.Content == "" {
		hint := actions.GetActionHint(actionName, "content")
		return fmt.Errorf("content is required%s", hint)
	}

	// Validate regex if regex mode enabled
	if fi.Regex {
		_, err := regexp.Compile(fi.Anchor)
		if err != nil {
			return fmt.Errorf("invalid regex anchor '%s': %w", fi.Anchor, err)
		}
	}

	return nil
}

// Execute runs the file_insert action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fi := step.FileInsert

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
	renderedPath, err := ec.PathUtil.ExpandPath(fi.Path, ec.CurrentDir, ctx.GetVariables())
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

	// Render anchor and content (support template variables)
	renderedAnchor, err := ctx.GetTemplate().Render(fi.Anchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render anchor: %w", err)
	}

	renderedContent, err := ctx.GetTemplate().Render(fi.Content, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render content: %w", err)
	}

	// Perform insertion
	newContent, insertionCount, err := h.performInsertion(
		string(originalContent),
		renderedAnchor,
		renderedContent,
		fi.Position,
		fi.Regex,
		fi.AllowMultiple,
	)
	if err != nil {
		return result, err
	}

	// Check if changes were made
	if string(originalContent) == newContent {
		result.Changed = false
		ctx.GetLogger().Debugf("  No changes needed (anchor not found or content already present)")
		return result, nil
	}

	// Create backup if requested
	if fi.Backup {
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
	ctx.GetLogger().Infof("  Inserted content at %d location(s) in %s", insertionCount, renderedPath)

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
		"path":       renderedPath,
		"insertions": insertionCount,
		"anchor":     renderedAnchor,
		"position":   fi.Position,
	})

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	fi := step.FileInsert

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	renderedPath, err := ec.PathUtil.ExpandPath(fi.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = fi.Path
	}

	// Render anchor
	renderedAnchor, err := ctx.GetTemplate().Render(fi.Anchor, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render anchor: %v", err)
		renderedAnchor = fi.Anchor
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would insert content %s anchor '%s' in %s",
		fi.Position, renderedAnchor, renderedPath)

	if fi.Regex {
		ctx.GetLogger().Infof("            Mode: regex")
	} else {
		ctx.GetLogger().Infof("            Mode: literal")
	}

	if fi.AllowMultiple {
		ctx.GetLogger().Infof("            Insert at all matches: yes")
	} else {
		ctx.GetLogger().Infof("            Insert at first match only")
	}

	if fi.Backup {
		ctx.GetLogger().Infof("            Backup: %s.bak", renderedPath)
	}

	return nil
}

// performInsertion performs the actual text insertion
func (h *Handler) performInsertion(content, anchor, insertion, position string, useRegex, allowMultiple bool) (newContent string, count int, err error) {
	lines := strings.Split(content, "\n")
	var result []string
	inserted := false

	for _, line := range lines {
		matched := false

		if useRegex {
			// Regex matching
			re, compileErr := regexp.Compile(anchor)
			if compileErr != nil {
				return "", 0, fmt.Errorf("invalid regex: %w", compileErr)
			}
			matched = re.MatchString(line)
		} else {
			// Literal matching
			matched = strings.Contains(line, anchor)
		}

		if matched {
			// Check if we should insert (first match only or allow multiple)
			if !inserted || allowMultiple {
				count++
				inserted = true

				if position == positionBefore {
					result = append(result, insertion, line)
				} else { // after
					result = append(result, line, insertion)
				}
			} else {
				// Already inserted at first match and allowMultiple=false
				result = append(result, line)
			}
		} else {
			result = append(result, line)
		}
	}

	if count == 0 {
		return "", 0, fmt.Errorf("anchor not found: %s", anchor)
	}

	newContent = strings.Join(result, "\n")
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
