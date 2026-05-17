// Package textpatchpath parses the dotted + indexed path syntax used
// by the in-place text patch actions (text.patch.json,
// text.patch.yaml). The supported shape is a.b.c and a[0].b[3].c —
// no wildcards, filters, or recursion.
//
// Distinct from internal/pathquery, which exposes Extract() for
// read-shaped queries (read.json, read.yaml, queryio). The two
// surfaces have different semantics around multi-index segments and
// error wording, and the patch path is intentionally narrower.
package textpatchpath

import (
	"fmt"
	"strconv"
	"strings"
)

// Segment is one element of a parsed path. Either Key is set (object
// field lookup) or IsIndex+Index (array index). The two are
// mutually exclusive by construction; the helpers below never emit a
// Segment with both populated.
type Segment struct {
	Key     string
	Index   int
	IsIndex bool
}

// Parse splits the supplied path into Segments. Returns an error for
// unsupported syntax: wildcards, filters, leading or trailing dots,
// empty segments, negative indices, unmatched brackets.
func Parse(path string) ([]Segment, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("path is empty")
	}
	if strings.ContainsAny(path, "*$?@") {
		return nil, fmt.Errorf("unsupported syntax in %q (only dotted + indexed paths are supported)", path)
	}
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("empty segment in %q", path)
	}
	if strings.HasPrefix(path, ".") || strings.HasSuffix(path, ".") {
		return nil, fmt.Errorf("leading or trailing dot in %q", path)
	}

	var out []Segment
	i := 0
	for i < len(path) {
		// Collect a key up to the next '.', '[', or end.
		j := i
		for j < len(path) && path[j] != '.' && path[j] != '[' {
			j++
		}
		if j > i {
			out = append(out, Segment{Key: path[i:j]})
		}
		if j >= len(path) {
			break
		}
		switch path[j] {
		case '.':
			i = j + 1
			if i >= len(path) {
				return nil, fmt.Errorf("trailing dot in %q", path)
			}
		case '[':
			end := strings.IndexByte(path[j+1:], ']')
			if end < 0 {
				return nil, fmt.Errorf("unmatched '[' in %q", path)
			}
			inside := path[j+1 : j+1+end]
			if inside == "" {
				return nil, fmt.Errorf("empty bracket in %q", path)
			}
			idx, err := strconv.Atoi(inside)
			if err != nil {
				return nil, fmt.Errorf("non-integer index %q in %q", inside, path)
			}
			if idx < 0 {
				return nil, fmt.Errorf("negative index %d in %q", idx, path)
			}
			out = append(out, Segment{Index: idx, IsIndex: true})
			i = j + 1 + end + 1
			// Allow optional '.' or '[' after the closing bracket.
			if i < len(path) && path[i] == '.' {
				i++
				if i >= len(path) {
					return nil, fmt.Errorf("trailing dot in %q", path)
				}
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("path %q has no segments", path)
	}
	return out, nil
}

// Validate is Parse-wrapped-to-discard-result so callers can reject
// unsupported syntax at validation time without building a slice.
func Validate(path string) error {
	_, err := Parse(path)
	return err
}
