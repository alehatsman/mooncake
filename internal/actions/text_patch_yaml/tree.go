// Package text_patch_yaml — yaml.Node walker + mutators. The yaml.v3
// node API preserves key order and comments adjacent to unchanged
// nodes on round-trip, which is what we need to surface as the
// action's idempotency / comment-preservation contract.
//
//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"bytes"
	"fmt"
	"reflect"

	"gopkg.in/yaml.v3"
)

// rootContent returns the addressable root node of a parsed YAML
// document. yaml.Decode always wraps the top-level value in a
// DocumentNode whose Content[0] is the actual root. Callers operate on
// that inner node so mutations propagate back through Marshal.
func rootContent(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) > 0 {
		return doc.Content[0]
	}
	return doc
}

// setDocumentContent replaces the root of a document. When the
// document was empty (e.g. parsed from `null` or an empty string)
// we wrap and install the supplied node in place.
func setDocumentContent(doc, child *yaml.Node) {
	if doc.Kind != yaml.DocumentNode {
		*doc = *child
		return
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{child}
		return
	}
	doc.Content[0] = child
}

// mappingLookup scans a MappingNode's interleaved key/value pairs and
// returns the value node and its index in Content for the given string key.
func mappingLookup(m *yaml.Node, key string) (valNode *yaml.Node, idx int) {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, -1
	}
	for i := 0; i < len(m.Content); i += 2 {
		k := m.Content[i]
		if k.Kind == yaml.ScalarNode && k.Value == key {
			return m.Content[i+1], i
		}
	}
	return nil, -1
}

// walk descends segs from root, returning the parent container, the
// final segment, the addressed value (or nil when missing), and a
// found flag. When create is true, intermediate missing keys / array
// slots are created (mapping for key segments, sequence for index
// segments).
func walk(root *yaml.Node, segs []segment, create bool) (parent *yaml.Node, last segment, leaf *yaml.Node, found bool, err error) {
	if root == nil {
		return nil, segment{}, nil, false, nil
	}
	cur := root
	for i, seg := range segs {
		isLast := i == len(segs)-1
		if seg.IsIndex {
			if cur.Kind != yaml.SequenceNode {
				if create {
					return nil, segment{}, nil, false, fmt.Errorf("path mismatch: cannot index non-sequence at segment %d", i)
				}
				return nil, segment{}, nil, false, nil
			}
			if seg.Index < 0 || seg.Index > len(cur.Content) {
				return nil, segment{}, nil, false, fmt.Errorf("array index %d out of range (len=%d) at segment %d", seg.Index, len(cur.Content), i)
			}
			if seg.Index == len(cur.Content) {
				if !create {
					return nil, segment{}, nil, false, nil
				}
				cur.Content = append(cur.Content, nil)
			}
			if isLast {
				return cur, seg, cur.Content[seg.Index], cur.Content[seg.Index] != nil, nil
			}
			if cur.Content[seg.Index] == nil {
				if !create {
					return nil, segment{}, nil, false, nil
				}
				if i+1 < len(segs) && segs[i+1].IsIndex {
					cur.Content[seg.Index] = &yaml.Node{Kind: yaml.SequenceNode}
				} else {
					cur.Content[seg.Index] = &yaml.Node{Kind: yaml.MappingNode}
				}
			}
			cur = cur.Content[seg.Index]
			continue
		}
		// Key segment.
		if cur.Kind != yaml.MappingNode {
			if create {
				return nil, segment{}, nil, false, fmt.Errorf("path mismatch: cannot key non-mapping at segment %d", i)
			}
			return nil, segment{}, nil, false, nil
		}
		val, _ := mappingLookup(cur, seg.Key)
		if isLast {
			return cur, seg, val, val != nil, nil
		}
		if val == nil {
			if !create {
				return nil, segment{}, nil, false, nil
			}
			var newChild *yaml.Node
			if i+1 < len(segs) && segs[i+1].IsIndex {
				newChild = &yaml.Node{Kind: yaml.SequenceNode}
			} else {
				newChild = &yaml.Node{Kind: yaml.MappingNode}
			}
			cur.Content = append(cur.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: seg.Key, Tag: "!!str"},
				newChild,
			)
			val = newChild
		}
		cur = val
	}
	return nil, segment{}, nil, false, fmt.Errorf("unreachable: empty segments slice")
}

// valueNode converts a Go value (string / number / bool / []interface{} /
// map[string]interface{} / etc.) into a yaml.Node. Round-trips via
// yaml.Marshal so any concrete type encodable to YAML works; the
// resulting node is suitable for direct insertion into the document
// tree.
func valueNode(v interface{}) (*yaml.Node, error) {
	// Normalize map[interface{}]interface{} which yaml.v2 emitted and
	// might bleed through if a caller hand-builds a value.
	if m, ok := v.(map[interface{}]interface{}); ok {
		conv := make(map[string]interface{}, len(m))
		for k, val := range m {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string map key %v", k)
			}
			conv[ks] = val
		}
		v = conv
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return nil, err
	}
	var n yaml.Node
	if err := yaml.Unmarshal(b, &n); err != nil {
		return nil, err
	}
	if n.Kind == yaml.DocumentNode && len(n.Content) > 0 {
		return n.Content[0], nil
	}
	return &n, nil
}

// setAt sets value at the addressed location. The path is created if
// missing. Returns mutated=false when the existing value is deep-equal
// to value (no write needed).
func setAt(root *yaml.Node, path string, value *yaml.Node) (bool, error) {
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
		parent.Content[last.Index] = value
		return true, nil
	}
	_, idx := mappingLookup(parent, last.Key)
	if idx >= 0 {
		parent.Content[idx+1] = value
		return true, nil
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: last.Key, Tag: "!!str"},
		value,
	)
	return true, nil
}

// deleteAt removes the leaf at path. Missing paths are no-ops.
func deleteAt(root *yaml.Node, path string) (bool, error) {
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
		parent.Content = append(parent.Content[:last.Index], parent.Content[last.Index+1:]...)
		return true, nil
	}
	_, idx := mappingLookup(parent, last.Key)
	if idx < 0 {
		return false, nil
	}
	parent.Content = append(parent.Content[:idx], parent.Content[idx+2:]...)
	return true, nil
}

// mergeAt deep-merges value into the node at path. Objects: non-
// destructive (add keys not already present, recurse on shared keys).
// Arrays: strategy-driven (append_unique | append | replace).
func mergeAt(root *yaml.Node, path string, value *yaml.Node, strategy string) (bool, error) {
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
			parent.Content[last.Index] = value
		} else {
			_, idx := mappingLookup(parent, last.Key)
			if idx >= 0 {
				parent.Content[idx+1] = value
			} else {
				parent.Content = append(parent.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: last.Key, Tag: "!!str"},
					value,
				)
			}
		}
		return true, nil
	}
	switch {
	case leaf.Kind == yaml.MappingNode && value.Kind == yaml.MappingNode:
		return mergeMapping(leaf, value), nil
	case leaf.Kind == yaml.SequenceNode && value.Kind == yaml.SequenceNode:
		merged, changed := mergeSequence(leaf, value, strategy)
		if !changed {
			return false, nil
		}
		if last.IsIndex {
			parent.Content[last.Index] = merged
		} else {
			_, idx := mappingLookup(parent, last.Key)
			parent.Content[idx+1] = merged
		}
		return true, nil
	default:
		return false, fmt.Errorf("merge at %q: cannot merge %s into %s", path, kindName(value.Kind), kindName(leaf.Kind))
	}
}

func mergeMapping(dst, src *yaml.Node) bool {
	added := false
	for i := 0; i < len(src.Content); i += 2 {
		srcKey := src.Content[i]
		srcVal := src.Content[i+1]
		dstVal, idx := mappingLookup(dst, srcKey.Value)
		if idx < 0 {
			dst.Content = append(dst.Content, srcKey, srcVal)
			added = true
			continue
		}
		if dstVal.Kind == yaml.MappingNode && srcVal.Kind == yaml.MappingNode {
			if mergeMapping(dstVal, srcVal) {
				added = true
			}
		}
	}
	return added
}

func mergeSequence(dst, src *yaml.Node, strategy string) (*yaml.Node, bool) {
	switch strategy {
	case "replace":
		if nodeEqual(dst, src) {
			return dst, false
		}
		return src, true
	case "append":
		if len(src.Content) == 0 {
			return dst, false
		}
		out := &yaml.Node{Kind: yaml.SequenceNode, Style: dst.Style}
		out.Content = append(out.Content, dst.Content...)
		out.Content = append(out.Content, src.Content...)
		return out, true
	case "", "append_unique":
		out := &yaml.Node{Kind: yaml.SequenceNode, Style: dst.Style}
		out.Content = append(out.Content, dst.Content...)
		added := false
		for _, item := range src.Content {
			dup := false
			for _, existing := range out.Content {
				if nodeEqual(existing, item) {
					dup = true
					break
				}
			}
			if !dup {
				out.Content = append(out.Content, item)
				added = true
			}
		}
		return out, added
	}
	return dst, false
}

// nodeEqual reports semantic equality of two yaml.Nodes via a
// decode-and-compare round-trip. Comments and node positions are
// intentionally ignored — equality is "do these encode to the same
// value", which is the relevant question for idempotency / dedup.
func nodeEqual(a, b *yaml.Node) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	var av, bv interface{}
	if err := a.Decode(&av); err != nil {
		// Fall back to raw byte comparison of the marshalled form.
		ab, _ := yaml.Marshal(a)
		bb, _ := yaml.Marshal(b)
		return bytes.Equal(ab, bb)
	}
	if err := b.Decode(&bv); err != nil {
		return false
	}
	return reflect.DeepEqual(av, bv)
}

func kindName(k yaml.Kind) string {
	switch k {
	case yaml.ScalarNode:
		return "scalar"
	case yaml.MappingNode:
		return "mapping"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.AliasNode:
		return "alias"
	case yaml.DocumentNode:
		return "document"
	}
	return "unknown"
}
