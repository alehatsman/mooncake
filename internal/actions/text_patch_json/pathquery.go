// Package text_patch_json — node walker / patch operations on top
// of the shared dotted+indexed path parser in internal/textpatchpath.
//
//nolint:revive // package name follows action convention
package text_patch_json

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/textpatchpath"
)

// segment is a package-local alias for textpatchpath.Segment so the
// walker/patch operations below read naturally without spelling out
// the import each time.
type segment = textpatchpath.Segment

func parsePath(path string) ([]segment, error) { return textpatchpath.Parse(path) }

func validatePath(path string) error { return textpatchpath.Validate(path) }

// walk traverses the tree along segs and returns the addressed node
// plus the parent and final-segment so callers can mutate. Returns
// (nil, false, nil) on path-miss (not an error).
//
// When create is true, intermediate missing keys / array slots are
// created as needed (objects for key segments, arrays for index
// segments). Array indices may extend the array by exactly +1 when
// idx == len(items); any other out-of-bounds index returns an error.
func walk(root *node, segs []segment, create bool) (parent *node, last segment, leaf *node, found bool, err error) {
	if root == nil {
		return nil, segment{}, nil, false, nil
	}
	cur := root
	for i, seg := range segs {
		isLast := i == len(segs)-1
		if seg.IsIndex {
			if cur.kind != nArray {
				if create {
					return nil, segment{}, nil, false, fmt.Errorf("path mismatch: cannot index non-array at segment %d", i)
				}
				return nil, segment{}, nil, false, nil
			}
			if seg.Index < 0 || seg.Index > len(cur.items) {
				return nil, segment{}, nil, false, fmt.Errorf("array index %d out of range (len=%d) at segment %d", seg.Index, len(cur.items), i)
			}
			if seg.Index == len(cur.items) {
				if !create {
					return nil, segment{}, nil, false, nil
				}
				cur.items = append(cur.items, nil)
			}
			if isLast {
				return cur, seg, cur.items[seg.Index], cur.items[seg.Index] != nil, nil
			}
			if cur.items[seg.Index] == nil {
				if !create {
					return nil, segment{}, nil, false, nil
				}
				// Allocate child as object by default; arrays only get created
				// when the next segment is an index.
				if i+1 < len(segs) && segs[i+1].IsIndex {
					cur.items[seg.Index] = newArray()
				} else {
					cur.items[seg.Index] = newObject()
				}
			}
			cur = cur.items[seg.Index]
			continue
		}
		// Key segment.
		if cur.kind != nObject {
			if create {
				return nil, segment{}, nil, false, fmt.Errorf("path mismatch: cannot key non-object at segment %d", i)
			}
			return nil, segment{}, nil, false, nil
		}
		child, ok := cur.fields[seg.Key]
		if isLast {
			return cur, seg, child, ok, nil
		}
		if !ok {
			if !create {
				return nil, segment{}, nil, false, nil
			}
			if i+1 < len(segs) && segs[i+1].IsIndex {
				child = newArray()
			} else {
				child = newObject()
			}
			cur.keys = append(cur.keys, seg.Key)
			cur.fields[seg.Key] = child
		}
		cur = child
	}
	return nil, segment{}, nil, false, fmt.Errorf("unreachable: empty segments slice")
}

// setAt sets value at the addressed location. The path is created if
// missing. Returns mutated=false when the existing value at path is
// already deep-equal to the supplied value (no write needed).
func setAt(root *node, path string, value *node) (bool, error) {
	segs, err := parsePath(path)
	if err != nil {
		return false, err
	}
	parent, last, existing, _, err := walk(root, segs, true)
	if err != nil {
		return false, err
	}
	if existing != nil && nodeEqual(existing, value) {
		return false, nil
	}
	if last.IsIndex {
		parent.items[last.Index] = value
		return true, nil
	}
	if _, ok := parent.fields[last.Key]; !ok {
		parent.keys = append(parent.keys, last.Key)
	}
	parent.fields[last.Key] = value
	return true, nil
}

// deleteAt removes the leaf at path. Missing paths return mutated=false.
func deleteAt(root *node, path string) (bool, error) {
	segs, err := parsePath(path)
	if err != nil {
		return false, err
	}
	parent, last, _, found, err := walk(root, segs, false)
	if err != nil {
		return false, err
	}
	if !found || parent == nil {
		return false, nil
	}
	if last.IsIndex {
		parent.items = append(parent.items[:last.Index], parent.items[last.Index+1:]...)
		return true, nil
	}
	delete(parent.fields, last.Key)
	for i, k := range parent.keys {
		if k == last.Key {
			parent.keys = append(parent.keys[:i], parent.keys[i+1:]...)
			break
		}
	}
	return true, nil
}

// mergeAt applies a merge operation at path. For objects: deep-set
// missing keys, never overwrite existing ones. For arrays: dispatch
// on strategy. Missing path: create and treat the merge value as the
// new content. Returns mutated=false when the merge is a no-op (no
// new keys added, no new array elements).
func mergeAt(root *node, path string, value *node, strategy string) (bool, error) {
	segs, err := parsePath(path)
	if err != nil {
		return false, err
	}
	parent, last, leaf, _, err := walk(root, segs, true)
	if err != nil {
		return false, err
	}
	if leaf == nil {
		if last.IsIndex {
			parent.items[last.Index] = value
		} else {
			if _, ok := parent.fields[last.Key]; !ok {
				parent.keys = append(parent.keys, last.Key)
			}
			parent.fields[last.Key] = value
		}
		return true, nil
	}
	switch {
	case leaf.kind == nObject && value.kind == nObject:
		added := mergeObject(leaf, value)
		return added, nil
	case leaf.kind == nArray && value.kind == nArray:
		merged, changed := mergeArray(leaf, value, strategy)
		if !changed {
			return false, nil
		}
		if last.IsIndex {
			parent.items[last.Index] = merged
		} else {
			parent.fields[last.Key] = merged
		}
		return true, nil
	default:
		return false, fmt.Errorf("merge at %q: cannot merge %s into %s", path, kindName(value.kind), kindName(leaf.kind))
	}
}

// mergeObject adds keys from src into dst whose keys aren't already
// present (spec: "deep-set, no overwrite of present keys"). Nested
// objects recurse; nested arrays follow the default append_unique
// rule. Existing keys are untouched. Returns true when at least one
// new key or nested key was added.
func mergeObject(dst, src *node) bool {
	added := false
	for _, k := range src.keys {
		srcVal := src.fields[k]
		dstVal, exists := dst.fields[k]
		if !exists {
			dst.keys = append(dst.keys, k)
			dst.fields[k] = srcVal
			added = true
			continue
		}
		if dstVal.kind == nObject && srcVal.kind == nObject {
			if mergeObject(dstVal, srcVal) {
				added = true
			}
		}
	}
	return added
}

// mergeArray returns the merged array plus whether merging produced
// any change (i.e. at least one element differs from dst).
func mergeArray(dst, src *node, strategy string) (*node, bool) {
	switch strategy {
	case "replace":
		if nodeEqual(dst, src) {
			return dst, false
		}
		return src, true
	case "append":
		// WARNING (#98): "append" is intentionally NON-idempotent — it
		// concatenates src onto dst unconditionally, so every run grows
		// the array and re-running a playbook keeps appending duplicates.
		// This is the one merge_strategy that breaks the package-level
		// idempotency guarantee. Callers who want convergence (add the
		// element only if it isn't already present) must use
		// "append_unique" instead.
		if len(src.items) == 0 {
			return dst, false
		}
		out := newArray()
		out.items = append(out.items, dst.items...)
		out.items = append(out.items, src.items...)
		return out, true
	case "", "append_unique":
		out := newArray()
		out.items = append(out.items, dst.items...)
		added := false
		for _, item := range src.items {
			dup := false
			for _, existing := range out.items {
				if nodeEqual(existing, item) {
					dup = true
					break
				}
			}
			if !dup {
				out.items = append(out.items, item)
				added = true
			}
		}
		return out, added
	}
	return dst, false
}

func kindName(k nodeKind) string {
	switch k {
	case nScalar:
		return "scalar"
	case nObject:
		return "object"
	case nArray:
		return "array"
	}
	return "unknown"
}
