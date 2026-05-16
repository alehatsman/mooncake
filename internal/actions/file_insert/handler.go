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
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName     = "text.insert"
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
		Name:           actionName,
		Description:    "Insert text before or after anchor patterns in files",
		Category:       actions.CategoryFile,
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

// Permissions implements actions.Permitter (spec-22). text.insert
// modifies a file via anchor-based insertion; Sudo when Path lives
// under a known system root. FilesystemWrite=[Path]. No Network; no
// RequiredBinaries.
func (h *Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.TextInsert == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.TextInsert.Path) {
		ps.Sudo = true
	}
	if step.TextInsert.Path != "" {
		ps.FilesystemWrite = []string{step.TextInsert.Path}
	}
	return ps
}

// Validate checks if the file_insert configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.TextInsert == nil {
		return fmt.Errorf("file_insert configuration is nil")
	}

	fi := step.TextInsert

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

// performInsertion performs the actual text insertion.
//
// MT-84: idempotency. For position=after, peek at the line(s)
// immediately following the matched anchor; if they already equal
// insertion, skip this match. For position=before, peek at the
// line(s) preceding the anchor in the output buffer. Multi-line
// insertion content is supported — the comparison splits on "\n"
// and matches the same number of lines.
func (h *Handler) performInsertion(content, anchor, insertion, position string, useRegex, allowMultiple bool) (newContent string, count int, err error) {
	lines := strings.Split(content, "\n")
	insertionLines := strings.Split(insertion, "\n")

	var (
		result     []string
		inserted   bool
		compiledRE *regexp.Regexp
	)
	if useRegex {
		var compileErr error
		compiledRE, compileErr = regexp.Compile(anchor)
		if compileErr != nil {
			return "", 0, fmt.Errorf("invalid regex: %w", compileErr)
		}
	}

	matches := func(line string) bool {
		if useRegex {
			return compiledRE.MatchString(line)
		}
		return strings.Contains(line, anchor)
	}

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if !matches(line) {
			result = append(result, line)
			continue
		}
		if inserted && !allowMultiple {
			result = append(result, line)
			continue
		}

		// MT-84: check for the already-inserted shape before mutating.
		// For position=after, the next N lines in the source buffer
		// should equal insertionLines. For position=before, the last N
		// lines accumulated into the result buffer should equal them.
		if position == positionAfter {
			if sliceEqual(lines, i+1, insertionLines) {
				// Already inserted in a prior run; treat as no-op for
				// this match. Don't set `inserted=true` — that would
				// suppress inserts at later anchors when
				// allowMultiple=false, which is wrong. Just emit the
				// anchor unchanged.
				result = append(result, line)
				continue
			}
			result = append(result, line)
			result = append(result, insertionLines...)
		} else { // before
			if tailEqual(result, insertionLines) {
				result = append(result, line)
				continue
			}
			result = append(result, insertionLines...)
			result = append(result, line)
		}
		count++
		inserted = true
	}

	if count == 0 {
		// MT-47 / MT-84: anchor not found OR every match already had
		// the insertion in place. Either way: idempotent success.
		// Caller's content-equality check folds this into "no changes".
		return strings.Join(result, "\n"), 0, nil
	}

	newContent = strings.Join(result, "\n")
	return newContent, count, nil
}

// sliceEqual reports whether lines[start : start+len(want)] equals
// want. Returns false if start is out of range. Used by MT-84's
// position=after peek.
func sliceEqual(lines []string, start int, want []string) bool {
	if start < 0 || start+len(want) > len(lines) {
		return false
	}
	for i, w := range want {
		if lines[start+i] != w {
			return false
		}
	}
	return true
}

// tailEqual reports whether the last len(want) entries in result
// equal want. Used by MT-84's position=before peek.
func tailEqual(result, want []string) bool {
	if len(result) < len(want) {
		return false
	}
	off := len(result) - len(want)
	for i, w := range want {
		if result[off+i] != w {
			return false
		}
	}
	return true
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

// Run is the Spec 16 unified entry point. Computes the would-be content
// in memory once; plan mode reports the prediction, execute mode
// commits the change.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fi := step.TextInsert

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

	renderedPath, err := ec.Svc.PathUtil.ExpandPath(fi.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}
	// F033: dead-code traversal check removed (see text_patch_ini).

	// #nosec G304 -- file path from user config
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	renderedAnchor, err := ctx.GetTemplate().Render(fi.Anchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render anchor: %w", err)
	}
	renderedContent, err := ctx.GetTemplate().Render(fi.Content, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render content: %w", err)
	}

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

	if string(originalContent) == newContent {
		result.Reason = "anchor not found or content already present"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would insert content at %d location(s)", insertionCount)
		return result, nil
	}

	// Capture pre-state for Reverse() (spec-22 phase 5 slice E).
	result.ReverseData = filehandler.CaptureReverseInfo(renderedPath, "")

	if fi.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Inserted content at %d location(s) in %s", insertionCount, renderedPath)

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
		"path":       renderedPath,
		"insertions": insertionCount,
		"anchor":     renderedAnchor,
		"position":   fi.Position,
	})
	return result, nil
}
