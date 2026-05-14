// Package text_patch_yaml — path parser for the small dotted +
// indexed subset (a.b.c, a[0], a.b[3].c). No wildcards, filters, or
// recursion. Mirrors the same surface used by text.patch.json so the
// two actions feel identical to users.
//
//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"fmt"
	"strconv"
	"strings"
)

// segment is one element of a parsed path.
type segment struct {
	Key     string
	Index   int
	IsIndex bool
}

func parsePath(path string) ([]segment, error) {
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

	var out []segment
	i := 0
	for i < len(path) {
		j := i
		for j < len(path) && path[j] != '.' && path[j] != '[' {
			j++
		}
		if j > i {
			out = append(out, segment{Key: path[i:j]})
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
			out = append(out, segment{Index: idx, IsIndex: true})
			i = j + 1 + end + 1
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

func validatePath(path string) error {
	_, err := parsePath(path)
	return err
}
