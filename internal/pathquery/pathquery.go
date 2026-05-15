// Package pathquery extracts a value from a parsed JSON/YAML tree by a
// small dotted/indexed path subset shared by spec-38 (read.json / read.yaml)
// and reserved for future spec-25 patch actions.
//
// Path grammar:
//
//	segment   := identifier ( "[" digit+ "]" )*
//	path      := segment ( "." segment )*
//
// Supported: dotted segments and integer indices (e.g. "service.port",
// "tools[0].name", "a.b[3].c"). Identifier characters: ASCII letter, digit
// (not leading), underscore, hyphen.
//
// Not supported (rejected at Validate-time): wildcards (`*`), root anchors
// (`$`), filters (`[?...]`), negative indices, empty segments, leading or
// trailing dots, unmatched brackets. The error message points the caller at
// the supported subset.
package pathquery

import (
	"fmt"
	"strconv"
	"strings"
)

// segment is one parsed step of a path: a key plus zero or more numeric
// indices that follow it (`a[0][1]` → key="a", indices=[0,1]).
type segment struct {
	key     string
	indices []int
}

// Validate parses path and returns an error for unsupported syntax. Used
// at YAML validate-time so misuse is caught before any IO.
func Validate(path string) error {
	if path == "" {
		return nil
	}
	_, err := parse(path)
	return err
}

// Extract walks v along path and returns the addressed subtree.
//
//   - Empty path returns (v, true, nil): the whole document.
//   - Missing key or out-of-range index returns (nil, false, nil): path-miss
//     is not an error; callers decide what to do.
//   - Type mismatch — e.g. indexing a non-array, dotting into a non-map —
//     returns (nil, false, err) with a message pointing at the offending
//     segment.
//   - Malformed syntax returns (nil, false, err) up front (same check as
//     Validate).
//
// Supported value shapes for v: any combination of map[string]any,
// []any, scalars (string/number/bool/nil). yaml.v3 and encoding/json's
// default Unmarshal-into-any both produce values of this shape.
func Extract(v any, path string) (any, bool, error) {
	if path == "" {
		return v, true, nil
	}
	segs, err := parse(path)
	if err != nil {
		return nil, false, err
	}
	cur := v
	for _, s := range segs {
		// Step into the map for the key.
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, fmt.Errorf("pathquery: cannot index non-object with key %q", s.key)
		}
		next, present := m[s.key]
		if !present {
			return nil, false, nil
		}
		cur = next
		// Apply any chained indices.
		for _, idx := range s.indices {
			arr, ok := cur.([]any)
			if !ok {
				return nil, false, fmt.Errorf("pathquery: cannot index non-array at %q[%d]", s.key, idx)
			}
			if idx >= len(arr) {
				return nil, false, nil
			}
			cur = arr[idx]
		}
	}
	return cur, true, nil
}

// parse tokenizes path into segments, rejecting unsupported syntax with a
// message that points at the supported subset.
func parse(path string) ([]segment, error) {
	if strings.HasPrefix(path, ".") {
		return nil, fmt.Errorf("pathquery: leading dot in %q (supported: dotted keys + integer indices)", path)
	}
	if strings.HasSuffix(path, ".") {
		return nil, fmt.Errorf("pathquery: trailing dot in %q (supported: dotted keys + integer indices)", path)
	}
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("pathquery: empty segment in %q (supported: dotted keys + integer indices)", path)
	}
	// Catch filter-shaped brackets (`[?...]`, `[*]`) up front so the
	// dot-split below can't mask them as "unmatched [".
	if i := strings.IndexByte(path, '['); i >= 0 {
		if j := strings.IndexByte(path[i:], ']'); j > 0 {
			body := path[i+1 : i+j]
			if strings.ContainsAny(body, "?*$") {
				return nil, fmt.Errorf("pathquery: unsupported filter %q (supported: integer indices only)", body)
			}
		}
	}
	parts := strings.Split(path, ".")
	out := make([]segment, 0, len(parts))
	for _, p := range parts {
		seg, err := parseSegment(p)
		if err != nil {
			return nil, err
		}
		out = append(out, seg)
	}
	return out, nil
}

func parseSegment(s string) (segment, error) {
	if s == "" {
		return segment{}, fmt.Errorf("pathquery: empty segment (supported: dotted keys + integer indices)")
	}
	// A segment is: identifier ( "[" int "]" )*
	keyEnd := strings.IndexByte(s, '[')
	var key, tail string
	if keyEnd < 0 {
		key, tail = s, ""
	} else {
		key, tail = s[:keyEnd], s[keyEnd:]
	}
	if err := validateKey(key, s); err != nil {
		return segment{}, err
	}
	indices, err := parseIndices(tail, s)
	if err != nil {
		return segment{}, err
	}
	return segment{key: key, indices: indices}, nil
}

func validateKey(key, segLabel string) error {
	if key == "" {
		return fmt.Errorf("pathquery: missing key before bracket in %q (supported: dotted keys + integer indices)", segLabel)
	}
	for i, r := range key {
		switch {
		case r == '*' || r == '$' || r == '?':
			return fmt.Errorf("pathquery: unsupported character %q in key (supported: dotted keys + integer indices)", string(r))
		case r == '_' || r == '-':
			// allowed
		case r >= '0' && r <= '9':
			if i == 0 {
				return fmt.Errorf("pathquery: key %q starts with a digit (use brackets for array indices)", key)
			}
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			// allowed
		default:
			return fmt.Errorf("pathquery: unsupported character %q in key %q", string(r), key)
		}
	}
	return nil
}

func parseIndices(tail, segLabel string) ([]int, error) {
	if tail == "" {
		return nil, nil
	}
	var out []int
	for len(tail) > 0 {
		if tail[0] != '[' {
			return nil, fmt.Errorf("pathquery: expected '[' in %q, got %q", segLabel, string(tail[0]))
		}
		end := strings.IndexByte(tail, ']')
		if end < 0 {
			return nil, fmt.Errorf("pathquery: unmatched '[' in %q", segLabel)
		}
		body := tail[1:end]
		if body == "" {
			return nil, fmt.Errorf("pathquery: empty index in %q", segLabel)
		}
		if strings.ContainsAny(body, "?*$") {
			return nil, fmt.Errorf("pathquery: unsupported filter %q (supported: integer indices only)", body)
		}
		if strings.HasPrefix(body, "-") {
			return nil, fmt.Errorf("pathquery: negative index %q (supported: non-negative integers only)", body)
		}
		idx, err := strconv.Atoi(body)
		if err != nil {
			return nil, fmt.Errorf("pathquery: invalid index %q (supported: non-negative integers only): %w", body, err)
		}
		out = append(out, idx)
		tail = tail[end+1:]
	}
	return out, nil
}
