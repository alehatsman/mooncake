// Package text_patch_yaml implements the text.patch.yaml action:
// structural edits to a YAML file via a tiny dotted + indexed path
// subset, preserving key order and comments adjacent to unchanged
// nodes (via yaml.v3's node API). Idempotent: a second run with
// byte-identical desired state writes nothing.
//
//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"bytes"
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
	"gopkg.in/yaml.v3"
)

const (
	actionName       = "text.patch.yaml"
	atomicTempSuffix = ".tmp"
)

// Handler implements text.patch.yaml.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Apply structural set/delete/merge edits to a YAML file, preserving order and adjacent comments",
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
	"":              true,
	"append_unique": true,
	"append":        true,
	"replace":       true,
}

func (h *Handler) Validate(step *config.Step) error {
	p := step.TextPatchYAML
	if p == nil {
		return fmt.Errorf("text.patch.yaml requires configuration")
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("text.patch.yaml: path is required")
	}
	if len(p.Set) == 0 && len(p.Delete) == 0 && len(p.Merge) == 0 {
		return fmt.Errorf("text.patch.yaml: at least one of set, delete, or merge is required")
	}
	if !validMergeStrategies[p.MergeStrategy] {
		return fmt.Errorf("text.patch.yaml: invalid merge_strategy %q (valid: append_unique|append|replace)", p.MergeStrategy)
	}
	deletePaths := make(map[string]struct{}, len(p.Delete))
	for i, path := range p.Delete {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.yaml: delete[%d]: %w", i, err)
		}
		deletePaths[path] = struct{}{}
	}
	for path := range p.Set {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.yaml: set[%q]: %w", path, err)
		}
		if _, conflict := deletePaths[path]; conflict {
			return fmt.Errorf("text.patch.yaml: path %q appears in both set and delete", path)
		}
	}
	for path := range p.Merge {
		if err := validatePath(path); err != nil {
			return fmt.Errorf("text.patch.yaml: merge[%q]: %w", path, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.TextPatchYAML
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("text.patch.yaml: context is not an ExecutionContext")
	}
	result := executor.NewResult()
	result.Checkable = true

	path, err := ec.Svc.PathUtil.ExpandPath(p.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("text.patch.yaml: expand path: %w", err)
	}
	// F033: dead-code traversal check removed (see text_patch_ini).

	original, exists, mode, err := readOriginal(path)
	if err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("text.patch.yaml: file not found: %s", path)
	}

	doc, err := parseYAML(original)
	if err != nil {
		return result, fmt.Errorf("text.patch.yaml: parse %s: %w", path, err)
	}

	indent := detectIndent(original)
	keepNewline := bytes.HasSuffix(original, []byte("\n"))

	mutated, err := applyEdits(doc, p)
	if err != nil {
		return result, fmt.Errorf("text.patch.yaml: %w", err)
	}

	result.Data = map[string]interface{}{"path": path}

	if !mutated {
		result.Reason = "YAML file already matches desired state"
		return result, nil
	}

	newBytes, err := marshalDoc(doc, indent)
	if err != nil {
		return result, fmt.Errorf("text.patch.yaml: encode: %w", err)
	}
	if keepNewline && !bytes.HasSuffix(newBytes, []byte("\n")) {
		newBytes = append(newBytes, '\n')
	}
	if !keepNewline && bytes.HasSuffix(newBytes, []byte("\n")) {
		newBytes = newBytes[:len(newBytes)-1]
	}

	if bytes.Equal(newBytes, original) {
		result.Reason = "YAML file already matches desired state"
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
			return result, fmt.Errorf("text.patch.yaml: backup: %w", err)
		}
	}
	if err := writeAtomic(path, newBytes, mode); err != nil {
		return result, fmt.Errorf("text.patch.yaml: write: %w", err)
	}

	result.Changed = true
	result.Reason = reason
	ctx.GetLogger().Infof("  text.patch.yaml: %s", path)
	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{Path: path, Changed: true, DryRun: false},
		})
	}
	return result, nil
}

// applyEdits applies delete → set → merge in a deterministic
// alphabetic order within each group. Returns true when at least one
// edit changed the tree.
func applyEdits(doc *yaml.Node, p *config.TextPatchYAML) (bool, error) {
	root := rootContent(doc)
	if root == nil {
		// File parsed to nothing; only `set` makes sense, and only when
		// at least one set path is supplied.
		if len(p.Set) == 0 {
			return false, nil
		}
		root = &yaml.Node{Kind: yaml.MappingNode}
		setDocumentContent(doc, root)
	}

	mutated := false
	for _, path := range p.Delete {
		ok, err := deleteAt(root, path)
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
		value, err := valueNode(p.Set[path])
		if err != nil {
			return false, fmt.Errorf("set %q: %w", path, err)
		}
		ok, err := setAt(root, path, value)
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
		value, err := valueNode(p.Merge[path])
		if err != nil {
			return false, fmt.Errorf("merge %q: %w", path, err)
		}
		ok, err := mergeAt(root, path, value, strategy)
		if err != nil {
			return false, fmt.Errorf("merge %q: %w", path, err)
		}
		mutated = mutated || ok
	}
	return mutated, nil
}

func parseYAML(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func marshalDoc(doc *yaml.Node, indent int) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	if indent > 0 {
		enc.SetIndent(indent)
	}
	// Encoding a DocumentNode emits the document body. Encoding any
	// other Kind emits it directly.
	target := doc
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		target = doc.Content[0]
	}
	if err := enc.Encode(target); err != nil {
		_ = enc.Close()
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// detectIndent returns the spaces-per-level used by the input or the
// yaml.v3 default (4) when no indentation can be detected. yaml.v3
// only supports space-based indentation (no tabs); the encoder
// SetIndent takes an int.
func detectIndent(data []byte) int {
	for i := 0; i < len(data); i++ {
		// Look for the first newline followed by leading spaces in
		// what we presume is the body of a mapping or sequence.
		if data[i] != '\n' {
			continue
		}
		j := i + 1
		count := 0
		for j < len(data) && data[j] == ' ' {
			count++
			j++
		}
		if count > 0 && j < len(data) && data[j] != '\n' {
			return count
		}
	}
	return 0 // 0 → encoder uses its default (4)
}

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
