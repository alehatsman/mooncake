// Package text_patch_json implements the text.patch.json action:
// structural edits to a JSON file via a tiny dotted + indexed path
// subset, preserving key order, indentation, and trailing newline.
// Idempotent: a second run with byte-identical desired state writes
// nothing.
//
//nolint:revive // package name follows action convention
package text_patch_json

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName       = "text.patch.json"
	atomicTempSuffix = ".tmp"
)

// Handler implements text.patch.json.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Apply structural set/delete/merge edits to a JSON file with order + indent preservation",
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

var validMergeStrategies = map[string]bool{
	"":              true, // default → append_unique
	"append_unique": true,
	"append":        true,
	"replace":       true,
}

func (h *Handler) Validate(step *config.Step) error {
	p := step.TextPatchJSON
	if p == nil {
		return fmt.Errorf("text.patch.json requires configuration")
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("text.patch.json: path is required")
	}
	if len(p.Set) == 0 && len(p.Delete) == 0 && len(p.Merge) == 0 {
		// MT-32: users who reach for RFC 6902 JSON Patch
		// (operations: [{op, path, value}]) silently lose the field
		// (it's not in the schema) and land here with a generic
		// error. Spell out the supported shape so they don't waste
		// time wondering why the standard form failed.
		return fmt.Errorf(
			"text.patch.json: at least one of set, delete, or merge is required " +
				"(note: RFC 6902 JSON Patch operations: [{op, path, value}] is NOT supported; " +
				"use the higher-level set:/delete:/merge: keys instead)",
		)
	}
	if !validMergeStrategies[p.MergeStrategy] {
		return fmt.Errorf("text.patch.json: invalid merge_strategy %q (valid: append_unique|append|replace)", p.MergeStrategy)
	}
	// Validate path syntax for every supplied path. Reject collisions
	// between set and delete (they're contradictory).
	deletePaths := make(map[string]struct{}, len(p.Delete))
	for i, path := range p.Delete {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.json: delete[%d]: %w", i, err)
		}
		deletePaths[path] = struct{}{}
	}
	for path := range p.Set {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.json: set[%q]: %w", path, err)
		}
		if _, conflict := deletePaths[path]; conflict {
			return fmt.Errorf("text.patch.json: path %q appears in both set and delete", path)
		}
	}
	for path := range p.Merge {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.json: merge[%q]: %w", path, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.TextPatchJSON
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("text.patch.json: context is not an ExecutionContext")
	}
	result := executor.NewResult()
	result.Checkable = true

	path, err := ec.Svc.PathUtil.ExpandPath(p.Path, ec.CurrentDir, ctx.Variables())
	if err != nil {
		return result, fmt.Errorf("text.patch.json: expand path: %w", err)
	}
	// F033: dead-code traversal check removed (see text_patch_ini).

	original, exists, mode, err := readOriginal(path)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("text.patch.json: file not found: %s", path)
	}

	tree, err := parseTree(original)
	if err != nil {
		return result, fmt.Errorf("text.patch.json: parse %s: %w", path, err)
	}

	indent := detectIndent(original)
	keepNewline := detectTrailingNewline(original)

	mutated, err := applyEdits(tree, p)
	if err != nil {
		return result, fmt.Errorf("text.patch.json: %w", err)
	}

	result.Operation = executor.OpUpdate
	result.Target = path
	result.Data = map[string]interface{}{"path": path}

	// If no edit actually changed the tree (e.g. delete of a missing
	// path, or set to the value already there), skip emit + write.
	// This sidesteps the formatting round-trip problem entirely: a
	// no-op edit must produce a no-op result regardless of how the
	// source happens to be whitespaced.
	if !mutated {
		result.Operation = executor.OpNoop
		result.Reason = "JSON file already matches desired state"
		return result, nil
	}

	newBytes := tree.emit(indent)
	if keepNewline {
		newBytes = append(newBytes, '\n')
	}

	if bytesEqual(newBytes, original) {
		result.Operation = executor.OpNoop
		result.Reason = "JSON file already matches desired state"
		return result, nil
	}

	reason := "would update " + path
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = reason
		return result, nil
	}

	// Capture pre-state for Reverse() (spec-22 phase 5 slice E).
	result.ReverseData = filehandler.CaptureReverseInfo(path, "")

	if p.Backup {
		// #nosec G306 -- mirrors backup style used by sibling text actions.
		if err := os.WriteFile(path+".bak", original, 0o600); err != nil {
			return result, fmt.Errorf("text.patch.json: backup: %w", err)
		}
	}

	if err := writeAtomic(path, newBytes, mode); err != nil {
		return result, fmt.Errorf("text.patch.json: write: %w", err)
	}

	result.Changed = true
	result.Reason = reason
	ctx.Logger().Infof("  text.patch.json: %s", path)

	if pub := ctx.EventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: path, Changed: true, DryRun: false},
		})
	}
	return result, nil
}

// applyEdits walks the configured edits in a deterministic order
// (delete → set → merge, alphabetic within each) so two runs with the
// same input produce identical output. Returns true when at least
// one edit produced an observable mutation; false when every edit
// was a no-op (e.g. delete of a missing key, set of an already-equal
// value).
func applyEdits(tree *node, p *config.TextPatchJSON) (bool, error) {
	mutated := false
	for _, path := range p.Delete {
		ok, err := deleteAt(tree, path)
		if err != nil {
			return false, fmt.Errorf("delete %q: %w", path, err)
		}
		mutated = mutated || ok
	}
	setPaths := make([]string, 0, len(p.Set))
	for k := range p.Set {
		setPaths = append(setPaths, k)
	}
	sort.Strings(setPaths)
	for _, path := range setPaths {
		value, err := nodeFromValue(p.Set[path])
		if err != nil {
			return false, fmt.Errorf("set %q: %w", path, err)
		}
		ok, err := setAt(tree, path, value)
		if err != nil {
			return false, fmt.Errorf("set %q: %w", path, err)
		}
		mutated = mutated || ok
	}
	mergePaths := make([]string, 0, len(p.Merge))
	for k := range p.Merge {
		mergePaths = append(mergePaths, k)
	}
	sort.Strings(mergePaths)
	strategy := p.MergeStrategy
	if strategy == "" {
		strategy = "append_unique"
	}
	for _, path := range mergePaths {
		value, err := nodeFromValue(p.Merge[path])
		if err != nil {
			return false, fmt.Errorf("merge %q: %w", path, err)
		}
		ok, err := mergeAt(tree, path, value, strategy)
		if err != nil {
			return false, fmt.Errorf("merge %q: %w", path, err)
		}
		mutated = mutated || ok
	}
	return mutated, nil
}

// readOriginal returns (content, exists, mode, err). A non-existent
// file is not an error; the caller decides whether that's valid.
func readOriginal(path string) ([]byte, bool, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, 0o644, nil
		}
		return nil, false, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
	}
	return data, true, info.Mode().Perm(), nil
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	tmp := path + atomicTempSuffix
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
