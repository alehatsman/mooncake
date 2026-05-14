// Package text_patch_ini implements the text.patch.ini action:
// structural section/key edits for INI-style configuration files
// (php.ini, systemd unit files, ssh_config, ...). It preserves
// comments, blank lines, section ordering, indentation of untouched
// keys, and the file's native line ending (LF or CRLF). Idempotent
// by design: a second run with the same desired state produces a
// byte-identical file.
//
//nolint:revive // package name follows action convention
package text_patch_ini

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
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	actionName       = "text.patch.ini"
	defaultFileMode  = 0o644
	atomicTempSuffix = ".tmp"
)

// Handler implements text.patch.ini.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the text.patch.ini action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Apply structural section/key edits to an INI-style configuration file",
		Category:           actions.CategoryFile,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventFileUpdated)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // INI is portable across all platforms.
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Validate checks that the text.patch.ini step is well-formed: a path
// is present, at least one of set/delete is non-empty, no key is both
// set AND deleted, and no key string is empty. This runs before any
// I/O so misconfiguration fails fast.
func (h *Handler) Validate(step *config.Step) error {
	p := step.TextPatchINI
	if p == nil {
		return fmt.Errorf("text.patch.ini requires configuration")
	}
	if strings.TrimSpace(p.Path) == "" {
		return fmt.Errorf("text.patch.ini: path is required")
	}
	if len(p.Set) == 0 && len(p.Delete) == 0 {
		return fmt.Errorf("text.patch.ini: at least one of set or delete is required")
	}
	for k := range p.Set {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("text.patch.ini: set key must not be empty")
		}
	}
	delSet := make(map[string]struct{}, len(p.Delete))
	for _, k := range p.Delete {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("text.patch.ini: delete key must not be empty")
		}
		delSet[k] = struct{}{}
	}
	for k := range p.Set {
		if _, conflict := delSet[k]; conflict {
			return fmt.Errorf("text.patch.ini: key %q appears in both set and delete", k)
		}
	}
	return nil
}

// Run is the unified plan/apply entry point. Both modes compute the
// same desired bytes via the iniDoc parser/emitter and compare against
// the on-disk content; plan mode reports the prediction and applies
// nothing, apply mode writes atomically (and optionally backs up).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	p := step.TextPatchINI
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("text.patch.ini: context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	path, err := ec.Svc.PathUtil.ExpandPath(p.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("text.patch.ini: expand path: %w", err)
	}
	if pathErr := pathutil.ValidateNoPathTraversal(path); pathErr != nil {
		ctx.GetLogger().Debugf("text.patch.ini: path validation warning: %v", pathErr)
	}

	original, fileExists, mode, err := readOriginal(path)
	if err != nil {
		return result, err
	}

	// File missing + only delete keys: nothing to do.
	if !fileExists && len(p.Set) == 0 {
		result.Reason = "file does not exist; nothing to delete"
		return result, nil
	}

	doc := parseINI([]byte(original))
	changed := applyEdits(&doc, p)
	newBytes := doc.render()

	result.Data = map[string]interface{}{
		"path": path,
	}

	// Compare against the actual original bytes, not the boolean from
	// applyEdits: parsing + rendering an already-conformant file must
	// be byte-identical. If it isn't, treat the diff as the source of
	// truth.
	if bytesEqual(newBytes, []byte(original)) && fileExists {
		_ = changed
		result.Reason = "INI file already matches desired state"
		return result, nil
	}

	reason := "would update INI keys"
	if !fileExists {
		reason = "would create INI file"
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = reason
		return result, nil
	}

	// Capture pre-state for Reverse() (spec-22 phase 5 slice E).
	result.ReverseData = filehandler.CaptureReverseInfo(path, "")

	if p.Backup && fileExists {
		// #nosec G306 -- mirrors backup mode used by sibling text actions.
		if err := os.WriteFile(path+".bak", []byte(original), 0o600); err != nil {
			return result, fmt.Errorf("text.patch.ini: backup: %w", err)
		}
	}

	if err := writeAtomic(path, newBytes, mode); err != nil {
		return result, fmt.Errorf("text.patch.ini: write: %w", err)
	}

	result.Changed = true
	result.Reason = reason
	ctx.GetLogger().Infof("  text.patch.ini: %s", path)

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

// ---------------------------------------------------------------------
// INI parser / emitter
// ---------------------------------------------------------------------

// iniItemKind tags each parsed line so the renderer can reproduce the
// original layout: section headers, key=value pairs, comments, and
// blank lines are all retained.
type iniItemKind int

const (
	itemBlank iniItemKind = iota
	itemComment
	itemSection
	itemKey
	itemRaw // anything unrecognized; preserved verbatim
)

// iniItem holds a single line plus enough context to mutate it without
// disturbing its formatting (indent + separator spacing for keys). All
// non-key items are reproduced verbatim from `raw`.
type iniItem struct {
	kind    iniItemKind
	section string // section this line belongs to ("" = top-level)
	key     string // for itemKey: the bare key name (trimmed)
	indent  string // leading whitespace before the key
	sep     string // chars between key and value: e.g. " = ", "=", " "
	value   string // raw value substring (no trailing newline)
	raw     string // verbatim line (no line ending); for keys, rebuilt on set()
}

// iniDoc is the parsed, mutable representation of an INI file.
type iniDoc struct {
	items []iniItem
	eol   string // detected line ending: "\n" or "\r\n"
}

// parseINI decodes the bytes into an iniDoc. Comments (`#`/`;`),
// blanks, and unrecognized lines are kept as-is. Line ending is
// detected from the first occurrence of \r\n; fallback "\n".
func parseINI(data []byte) iniDoc {
	doc := iniDoc{eol: detectEOL(data)}
	if len(data) == 0 {
		return doc
	}

	// Split preserving empty trailing element when content ends with EOL.
	text := string(data)
	hasFinalEOL := strings.HasSuffix(text, doc.eol)
	if hasFinalEOL {
		text = strings.TrimSuffix(text, doc.eol)
	}
	rawLines := strings.Split(text, doc.eol)

	currentSection := ""
	for _, line := range rawLines {
		item := classifyLine(line, currentSection)
		if item.kind == itemSection {
			currentSection = item.section
		}
		doc.items = append(doc.items, item)
	}

	// Preserve the "ended with EOL" property as an empty trailing
	// itemBlank so render() can re-emit the final newline. We avoid
	// adding it when the file was a single empty string.
	if hasFinalEOL {
		doc.items = append(doc.items, iniItem{kind: itemBlank, raw: ""})
	}
	return doc
}

// classifyLine inspects a single line (no trailing newline) and
// returns the matching iniItem. The parser is intentionally
// permissive: anything that doesn't look like a section header,
// comment, or key=value is preserved as raw so we don't corrupt
// hand-written quirks.
func classifyLine(line, currentSection string) iniItem {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return iniItem{kind: itemBlank, section: currentSection, raw: line}
	}
	if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
		return iniItem{kind: itemComment, section: currentSection, raw: line}
	}
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") && len(trimmed) >= 2 {
		name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		return iniItem{kind: itemSection, section: name, raw: line}
	}
	// Key=Value or Key Value (ssh_config style).
	// Split on FIRST '=' if any; otherwise on FIRST run of whitespace.
	indent := leadingWhitespace(line)
	body := line[len(indent):]
	if eq := strings.Index(body, "="); eq >= 0 {
		keyPart := body[:eq]
		rest := body[eq+1:]
		key := strings.TrimRight(keyPart, " \t")
		sepTail := keyPart[len(key):]
		valLead := leadingWhitespace(rest)
		value := rest[len(valLead):]
		return iniItem{
			kind:    itemKey,
			section: currentSection,
			key:     key,
			indent:  indent,
			sep:     sepTail + "=" + valLead,
			value:   value,
			raw:     line,
		}
	}
	// ssh_config style: "Port 22".
	if ws := firstWhitespace(body); ws >= 0 {
		key := body[:ws]
		rest := body[ws:]
		valLead := leadingWhitespace(rest)
		value := rest[len(valLead):]
		return iniItem{
			kind:    itemKey,
			section: currentSection,
			key:     key,
			indent:  indent,
			sep:     valLead,
			value:   value,
			raw:     line,
		}
	}
	// Bare token with no value (e.g. "BareFlag"). Treat as a key with
	// empty value and a single-space separator placeholder so we can
	// still re-emit if asked to update it.
	return iniItem{
		kind:    itemKey,
		section: currentSection,
		key:     body,
		indent:  indent,
		sep:     " ",
		value:   "",
		raw:     line,
	}
}

// set ensures section.key has the given value. Returns true when the
// underlying document changed. If the key already exists in the
// target section, only the value substring is rewritten so spacing
// around `=` and the original indent are preserved. If the section
// doesn't exist yet, a `[Section]` block is appended at end-of-file.
func (d *iniDoc) set(section, key, value string) bool {
	// Update existing key in section.
	for i := range d.items {
		it := &d.items[i]
		if it.kind == itemKey && it.section == section && it.key == key {
			if it.value == value {
				return false
			}
			it.value = value
			// Rebuild the raw line so render() can rely solely on the
			// item fields if needed; we keep raw in sync.
			it.raw = it.indent + it.key + it.sep + it.value
			return true
		}
	}

	// Key not present. Need to add it. If section exists, insert at
	// end of that section block; otherwise append a new section.
	if section == "" {
		// Top-level insert: drop a key at the very top of the file
		// (before any section header). If the file already starts
		// with top-level keys this places it after the last one.
		insertAt := lastTopLevelInsertionPoint(d.items)
		newItem := iniItem{
			kind:    itemKey,
			section: "",
			key:     key,
			indent:  "",
			sep:     "=",
			value:   value,
			raw:     key + "=" + value,
		}
		d.items = insertAtIndex(d.items, insertAt, newItem)
		return true
	}

	// Section path.
	sectionStart := -1
	sectionEnd := len(d.items) // exclusive
	for i, it := range d.items {
		if it.kind == itemSection && it.section == section {
			sectionStart = i
			// Find end = next section header (or EOF).
			sectionEnd = len(d.items)
			for j := i + 1; j < len(d.items); j++ {
				if d.items[j].kind == itemSection {
					sectionEnd = j
					break
				}
			}
			break
		}
	}

	if sectionStart == -1 {
		// Create new section at end of file. Locate the insertion
		// point just before any trailing "ends-with-EOL" marker so
		// we don't double-up newlines, then ensure exactly one blank
		// line separates the new section from the prior content.
		insertAt := tailContentEnd(d.items)
		additions := []iniItem{}
		if needsLeadingBlank(d.items) {
			additions = append(additions, iniItem{kind: itemBlank, section: "", raw: ""})
		}
		additions = append(additions,
			iniItem{kind: itemSection, section: section, raw: "[" + section + "]"},
			iniItem{
				kind:    itemKey,
				section: section,
				key:     key,
				indent:  "",
				sep:     "=",
				value:   value,
				raw:     key + "=" + value,
			},
		)
		d.items = insertSliceAt(d.items, insertAt, additions)
		return true
	}

	// Insert at end of existing section, before any trailing blank
	// lines that belong to a gap before the next section.
	insertAt := sectionEnd
	for insertAt > sectionStart+1 && d.items[insertAt-1].kind == itemBlank {
		insertAt--
	}
	newItem := iniItem{
		kind:    itemKey,
		section: section,
		key:     key,
		indent:  "",
		sep:     "=",
		value:   value,
		raw:     key + "=" + value,
	}
	d.items = insertAtIndex(d.items, insertAt, newItem)
	return true
}

// del removes any key line matching section + key. Returns true when
// at least one line was deleted. Comments and blank lines around the
// deleted key are left untouched, mirroring how a human editor would
// delete just the affected line.
func (d *iniDoc) del(section, key string) bool {
	out := d.items[:0]
	changed := false
	for _, it := range d.items {
		if it.kind == itemKey && it.section == section && it.key == key {
			changed = true
			continue
		}
		out = append(out, it)
	}
	d.items = out
	return changed
}

// render serializes the iniDoc back to bytes, preserving the detected
// line ending. The parser stores raw text per item; for keys whose
// value changed, set() also updated raw, so we can emit raw uniformly.
func (d iniDoc) render() []byte {
	if len(d.items) == 0 {
		return nil
	}
	var sb strings.Builder
	// Note: parseINI appends a trailing itemBlank with raw="" when
	// the input ended with EOL; that empty raw plus the EOL we emit
	// below reproduces the trailing newline exactly.
	for i, it := range d.items {
		sb.WriteString(it.raw)
		if i < len(d.items)-1 {
			sb.WriteString(d.eol)
		}
	}
	return []byte(sb.String())
}

// ---------------------------------------------------------------------
// edit driver + helpers
// ---------------------------------------------------------------------

// applyEdits runs `set` then `delete` against the document, returning
// whether any item-level change was reported. The actual byte-level
// "did the file change?" question is answered separately by comparing
// the rendered output to the original.
func applyEdits(doc *iniDoc, p *config.TextPatchINI) bool {
	changed := false
	// Sort keys so output is deterministic when creating new sections.
	keys := make([]string, 0, len(p.Set))
	for k := range p.Set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		section, name := splitKey(k)
		if doc.set(section, name, p.Set[k]) {
			changed = true
		}
	}
	for _, k := range p.Delete {
		section, name := splitKey(k)
		if doc.del(section, name) {
			changed = true
		}
	}
	return changed
}

// splitKey divides "Section.key" into ("Section", "key"). For keys
// without a dot, returns ("", key) — the sectionless form used by
// ssh_config-style files. The split is on the FIRST dot only, so a
// literal dot in a key name remains valid for sectionless files
// (where users provide bare keys without a section prefix).
func splitKey(s string) (section, key string) {
	if i := strings.IndexByte(s, '.'); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}

// detectEOL returns "\r\n" if the input contains CRLF, otherwise
// "\n". Mixed-ending files (extremely rare) are normalized to the
// first-seen CRLF on write, which is acceptable for config edits.
func detectEOL(data []byte) string {
	for i := 0; i+1 < len(data); i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return "\r\n"
		}
	}
	return "\n"
}

func leadingWhitespace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

func firstWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' || s[i] == '\t' {
			return i
		}
	}
	return -1
}

// lastTopLevelInsertionPoint returns the index at which a new
// sectionless key should be inserted. The position is after any
// existing top-level keys but before the first section header.
func lastTopLevelInsertionPoint(items []iniItem) int {
	lastKey := -1
	for i, it := range items {
		if it.kind == itemSection {
			if lastKey >= 0 {
				return lastKey + 1
			}
			return i
		}
		if it.kind == itemKey && it.section == "" {
			lastKey = i
		}
	}
	// No section header present at all → append at end (but before
	// the trailing-blank marker, if any).
	end := len(items)
	for end > 0 && items[end-1].kind == itemBlank && items[end-1].raw == "" {
		end--
	}
	return end
}

// needsLeadingBlank reports whether the doc should have a blank line
// inserted before a newly-appended section header. We add one when
// the existing tail is real content (i.e. not already a blank line).
func needsLeadingBlank(items []iniItem) bool {
	// Walk back past the trailing-EOL marker (an empty itemBlank).
	end := len(items)
	for end > 0 && items[end-1].kind == itemBlank && items[end-1].raw == "" {
		end--
	}
	if end == 0 {
		return false
	}
	return items[end-1].kind != itemBlank
}

func insertAtIndex(items []iniItem, idx int, in iniItem) []iniItem {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(items) {
		return append(items, in)
	}
	out := make([]iniItem, 0, len(items)+1)
	out = append(out, items[:idx]...)
	out = append(out, in)
	out = append(out, items[idx:]...)
	return out
}

// insertSliceAt inserts a slice of items at the given index. Used
// when appending a multi-item block (e.g. blank + section header +
// key) while preserving the trailing EOL marker.
func insertSliceAt(items []iniItem, idx int, ins []iniItem) []iniItem {
	if idx < 0 {
		idx = 0
	}
	if idx >= len(items) {
		return append(items, ins...)
	}
	out := make([]iniItem, 0, len(items)+len(ins))
	out = append(out, items[:idx]...)
	out = append(out, ins...)
	out = append(out, items[idx:]...)
	return out
}

// tailContentEnd returns the index just past the last real-content
// item, i.e. the position before any trailing empty-blank "ends with
// EOL" marker added by parseINI.
func tailContentEnd(items []iniItem) int {
	end := len(items)
	for end > 0 && items[end-1].kind == itemBlank && items[end-1].raw == "" {
		end--
	}
	return end
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

// ---------------------------------------------------------------------
// file I/O
// ---------------------------------------------------------------------

func readOriginal(path string) (content string, exists bool, mode os.FileMode, err error) {
	info, statErr := os.Stat(path)
	if errors.Is(statErr, fs.ErrNotExist) {
		return "", false, defaultFileMode, nil
	}
	if statErr != nil {
		return "", false, 0, fmt.Errorf("text.patch.ini: stat %s: %w", path, statErr)
	}
	if info.IsDir() {
		return "", false, 0, fmt.Errorf("text.patch.ini: %s is a directory", path)
	}
	// #nosec G304 -- file path from user config.
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false, 0, fmt.Errorf("text.patch.ini: read %s: %w", path, readErr)
	}
	return string(data), true, info.Mode().Perm(), nil
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	tmp := path + atomicTempSuffix
	if mode == 0 {
		mode = defaultFileMode
	}
	// #nosec G306 -- mode comes from the existing file (or default 0644 for new files), matches sibling text actions.
	if err := os.WriteFile(tmp, content, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
