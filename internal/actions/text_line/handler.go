// Package text_line implements the text.line action: ensure a single
// line is present or absent in a file, optionally anchored by regex
// and positioned via insert_after / insert_before. Closes the
// lineinfile-equivalent gap in the text.* surface.
//
//nolint:revive // Package name matches action name convention (text_line)
package text_line

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	actionName        = "text.line"
	statePresent      = "present"
	stateAbsent       = "absent"
	defaultFileMode   = 0o644
	atomicTempSuffix  = ".tmp"
)

// Handler implements text.line.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Ensure a line is present or absent in a file (lineinfile-equivalent)",
		Category:           actions.CategoryFile,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	tl := step.TextLine
	if tl == nil {
		return fmt.Errorf("text.line requires configuration")
	}
	if strings.TrimSpace(tl.Path) == "" {
		return fmt.Errorf("text.line: path is required")
	}

	state := normalizeState(tl.State)
	switch state {
	case statePresent:
		if tl.Line == "" {
			return fmt.Errorf("text.line: line is required when state=present")
		}
		if strings.ContainsRune(tl.Line, '\n') {
			return fmt.Errorf("text.line: line must not contain newlines (use multiple steps or a different action)")
		}
	case stateAbsent:
		if tl.Line == "" && tl.Regexp == "" {
			return fmt.Errorf("text.line: state=absent requires either line or regexp")
		}
	default:
		return fmt.Errorf("text.line: state must be present or absent, got %q", tl.State)
	}

	if tl.Regexp != "" {
		if _, err := regexp.Compile(tl.Regexp); err != nil {
			return fmt.Errorf("text.line: invalid regexp: %w", err)
		}
	}
	if tl.InsertAfter != "" {
		if _, err := regexp.Compile(tl.InsertAfter); err != nil {
			return fmt.Errorf("text.line: invalid insert_after regex: %w", err)
		}
	}
	if tl.InsertBefore != "" {
		if _, err := regexp.Compile(tl.InsertBefore); err != nil {
			return fmt.Errorf("text.line: invalid insert_before regex: %w", err)
		}
	}
	if tl.InsertAfter != "" && tl.InsertBefore != "" {
		return fmt.Errorf("text.line: insert_after and insert_before are mutually exclusive")
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	tl := step.TextLine
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("text.line: context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	path, err := ec.Svc.PathUtil.ExpandPath(tl.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("text.line: expand path: %w", err)
	}
	if pathErr := pathutil.ValidateNoPathTraversal(path); pathErr != nil {
		ctx.GetLogger().Debugf("text.line: path validation warning: %v", pathErr)
	}

	rendered, err := renderFields(ctx, tl)
	if err != nil {
		return result, err
	}

	original, fileExists, mode, err := readOriginal(path)
	if err != nil {
		return result, err
	}

	plan, err := computePlan(rendered, original, fileExists)
	if err != nil {
		return result, err
	}

	result.Data = map[string]interface{}{
		"path":      path,
		"operation": plan.operation,
	}

	if !plan.changed {
		result.Reason = plan.reason
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = plan.reason
		return result, nil
	}

	if tl.Backup && fileExists {
		if err := os.WriteFile(path+".bak", []byte(original), 0o600); err != nil {
			return result, fmt.Errorf("text.line: backup: %w", err)
		}
	}

	if err := writeAtomic(path, plan.newContent, mode); err != nil {
		return result, fmt.Errorf("text.line: write: %w", err)
	}

	result.Changed = true
	result.Reason = plan.reason
	ctx.GetLogger().Infof("  text.line: %s (%s)", path, plan.operation)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{
				Path:    path,
				Changed: true,
				DryRun:  false,
			},
		})
	}
	return result, nil
}

// resolved is the post-template view of TextLine fields.
type resolved struct {
	state        string
	line         string
	regexp       *regexp.Regexp
	insertAfter  *regexp.Regexp
	insertBefore *regexp.Regexp
}

func renderFields(ctx actions.Context, tl *config.TextLine) (resolved, error) {
	r := resolved{state: normalizeState(tl.State)}

	tr := ctx.GetTemplate()
	vars := ctx.GetVariables()
	var err error
	r.line, err = tr.Render(tl.Line, vars)
	if err != nil {
		return r, fmt.Errorf("text.line: render line: %w", err)
	}
	rendRegexp, err := tr.Render(tl.Regexp, vars)
	if err != nil {
		return r, fmt.Errorf("text.line: render regexp: %w", err)
	}
	rendIA, err := tr.Render(tl.InsertAfter, vars)
	if err != nil {
		return r, fmt.Errorf("text.line: render insert_after: %w", err)
	}
	rendIB, err := tr.Render(tl.InsertBefore, vars)
	if err != nil {
		return r, fmt.Errorf("text.line: render insert_before: %w", err)
	}

	if rendRegexp != "" {
		r.regexp, err = regexp.Compile(rendRegexp)
		if err != nil {
			return r, fmt.Errorf("text.line: regexp: %w", err)
		}
	}
	if rendIA != "" {
		r.insertAfter, err = regexp.Compile(rendIA)
		if err != nil {
			return r, fmt.Errorf("text.line: insert_after: %w", err)
		}
	}
	if rendIB != "" {
		r.insertBefore, err = regexp.Compile(rendIB)
		if err != nil {
			return r, fmt.Errorf("text.line: insert_before: %w", err)
		}
	}
	return r, nil
}

func normalizeState(s string) string {
	if s == "" {
		return statePresent
	}
	return strings.ToLower(s)
}

func readOriginal(path string) (content string, exists bool, mode os.FileMode, err error) {
	info, statErr := os.Stat(path)
	if errors.Is(statErr, fs.ErrNotExist) {
		return "", false, defaultFileMode, nil
	}
	if statErr != nil {
		return "", false, 0, fmt.Errorf("text.line: stat %s: %w", path, statErr)
	}
	if info.IsDir() {
		return "", false, 0, fmt.Errorf("text.line: %s is a directory", path)
	}
	// #nosec G304 -- file path from user config.
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false, 0, fmt.Errorf("text.line: read %s: %w", path, readErr)
	}
	return string(data), true, info.Mode().Perm(), nil
}

// computedPlan is the predicted outcome shared by plan and apply modes.
type computedPlan struct {
	changed    bool
	operation  string // "insert" | "replace" | "delete" | "noop-present" | "noop-absent" | "create"
	reason     string
	newContent string
}

func computePlan(r resolved, original string, fileExists bool) (computedPlan, error) {
	if r.state == stateAbsent {
		return computeAbsent(r, original, fileExists)
	}
	return computePresent(r, original, fileExists)
}

func computePresent(r resolved, original string, fileExists bool) (computedPlan, error) {
	if !fileExists {
		return computedPlan{
			changed:    true,
			operation:  "create",
			reason:     fmt.Sprintf("would create file with single line %q", r.line),
			newContent: ensureTrailingNewline(r.line),
		}, nil
	}

	lines, trailingNL := splitLines(original)

	// 1. Exact match of the target line → noop.
	for _, l := range lines {
		if l == r.line {
			return computedPlan{operation: "noop-present", reason: "line already present"}, nil
		}
	}

	// 2. Regexp anchor matches → replace first match.
	if r.regexp != nil {
		for i, l := range lines {
			if r.regexp.MatchString(l) {
				if l == r.line {
					return computedPlan{operation: "noop-present", reason: "line already present"}, nil
				}
				lines[i] = r.line
				return computedPlan{
					changed:    true,
					operation:  "replace",
					reason:     fmt.Sprintf("would replace line matching %q", r.regexp.String()),
					newContent: joinLines(lines, trailingNL),
				}, nil
			}
		}
	}

	// 3. insert_after anchor.
	if r.insertAfter != nil {
		for i, l := range lines {
			if r.insertAfter.MatchString(l) {
				inserted := append([]string{}, lines[:i+1]...)
				inserted = append(inserted, r.line)
				inserted = append(inserted, lines[i+1:]...)
				return computedPlan{
					changed:    true,
					operation:  "insert",
					reason:     fmt.Sprintf("would insert after line matching %q", r.insertAfter.String()),
					newContent: joinLines(inserted, trailingNL),
				}, nil
			}
		}
	}

	// 4. insert_before anchor.
	if r.insertBefore != nil {
		for i, l := range lines {
			if r.insertBefore.MatchString(l) {
				inserted := append([]string{}, lines[:i]...)
				inserted = append(inserted, r.line)
				inserted = append(inserted, lines[i:]...)
				return computedPlan{
					changed:    true,
					operation:  "insert",
					reason:     fmt.Sprintf("would insert before line matching %q", r.insertBefore.String()),
					newContent: joinLines(inserted, trailingNL),
				}, nil
			}
		}
	}

	// 5. Default: append at end.
	lines = append(lines, r.line)
	return computedPlan{
		changed:    true,
		operation:  "append",
		reason:     "would append line at end of file",
		newContent: joinLines(lines, true),
	}, nil
}

func computeAbsent(r resolved, original string, fileExists bool) (computedPlan, error) {
	if !fileExists {
		return computedPlan{operation: "noop-absent", reason: "file does not exist"}, nil
	}
	lines, trailingNL := splitLines(original)
	kept := make([]string, 0, len(lines))
	removed := 0
	for _, l := range lines {
		match := false
		if r.line != "" && l == r.line {
			match = true
		}
		if !match && r.regexp != nil && r.regexp.MatchString(l) {
			match = true
		}
		if match {
			removed++
			continue
		}
		kept = append(kept, l)
	}
	if removed == 0 {
		return computedPlan{operation: "noop-absent", reason: "no matching lines to remove"}, nil
	}
	return computedPlan{
		changed:    true,
		operation:  "delete",
		reason:     fmt.Sprintf("would remove %d line(s)", removed),
		newContent: joinLines(kept, trailingNL),
	}, nil
}

// splitLines splits content on \n, returning the slice of lines and
// whether the original ended with a newline. The final empty element
// produced by strings.Split is dropped; trailingNL preserves the
// distinction so the file can be reassembled byte-identically.
func splitLines(s string) ([]string, bool) {
	if s == "" {
		return nil, false
	}
	trailing := strings.HasSuffix(s, "\n")
	trimmed := s
	if trailing {
		trimmed = strings.TrimSuffix(s, "\n")
	}
	if trimmed == "" {
		// Original was a single "\n".
		return []string{""}, true
	}
	return strings.Split(trimmed, "\n"), trailing
}

func joinLines(lines []string, trailingNL bool) string {
	if len(lines) == 0 {
		if trailingNL {
			return "\n"
		}
		return ""
	}
	out := strings.Join(lines, "\n")
	if trailingNL {
		out += "\n"
	}
	return out
}

func ensureTrailingNewline(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}

func writeAtomic(path, content string, mode os.FileMode) error {
	tmp := path + atomicTempSuffix
	if mode == 0 {
		mode = defaultFileMode
	}
	if err := os.WriteFile(tmp, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
