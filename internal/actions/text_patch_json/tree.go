// Package text_patch_json — ordered tree representation used to keep
// key order and indentation stable across parse → mutate → emit.
//
//nolint:revive // package name follows action convention
package text_patch_json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// nodeKind tags the variant of a node.
type nodeKind int

const (
	nScalar nodeKind = iota // scalars: string, number, bool, null — kept as raw bytes
	nObject
	nArray
)

// node is an order-preserving representation of a JSON value. Objects
// carry an explicit key order so emit() reproduces the input layout
// for unchanged subtrees; scalars are kept as their original bytes so
// number formatting (e.g. 1.0 vs 1) round-trips verbatim.
type node struct {
	kind   nodeKind
	raw    json.RawMessage  // nScalar
	keys   []string         // nObject — source order
	fields map[string]*node // nObject
	items  []*node          // nArray
}

func newObject() *node {
	return &node{kind: nObject, fields: map[string]*node{}}
}

func newArray() *node {
	return &node{kind: nArray}
}

func newScalar(b []byte) *node {
	// Copy so the source byte slice can be released by the decoder.
	cp := make(json.RawMessage, len(b))
	copy(cp, b)
	return &node{kind: nScalar, raw: cp}
}

// parseTree decodes the source bytes into a node tree. UseNumber()
// is enabled so integer / float fidelity is preserved.
func parseTree(data []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return parseValue(dec, tok)
}

func parseValue(dec *json.Decoder, tok json.Token) (*node, error) {
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			return parseObject(dec)
		case '[':
			return parseArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delim %q", t)
		}
	case string:
		return newScalar(encodeScalar(t)), nil
	case json.Number:
		return newScalar([]byte(t.String())), nil
	case bool:
		if t {
			return newScalar([]byte("true")), nil
		}
		return newScalar([]byte("false")), nil
	case nil:
		return newScalar([]byte("null")), nil
	default:
		return nil, fmt.Errorf("unsupported token %T", tok)
	}
}

func parseObject(dec *json.Decoder) (*node, error) {
	obj := newObject()
	for dec.More() {
		ktok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := ktok.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key, got %T", ktok)
		}
		vtok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		val, err := parseValue(dec, vtok)
		if err != nil {
			return nil, err
		}
		obj.keys = append(obj.keys, key)
		obj.fields[key] = val
	}
	// Consume the closing brace.
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return obj, nil
}

func parseArray(dec *json.Decoder) (*node, error) {
	arr := newArray()
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		val, err := parseValue(dec, tok)
		if err != nil {
			return nil, err
		}
		arr.items = append(arr.items, val)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	return arr, nil
}

// encodeScalar produces a canonical JSON encoding of a scalar value.
// Used when set/merge supplies a raw Go value that needs marshalling
// to embed in the tree.
func encodeScalar(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// nodeFromValue converts a Go value (from YAML decode — string, bool,
// int, float64, []interface{}, map[string]interface{}, map[interface{}]interface{}, nil)
// into a node tree. Returns an error for unsupported types.
func nodeFromValue(v interface{}) (*node, error) {
	switch t := v.(type) {
	case nil:
		return newScalar([]byte("null")), nil
	case bool, string, float64, int, int64:
		b, err := json.Marshal(t)
		if err != nil {
			return nil, err
		}
		return newScalar(b), nil
	case json.Number:
		return newScalar([]byte(t.String())), nil
	case []interface{}:
		arr := newArray()
		for _, item := range t {
			child, err := nodeFromValue(item)
			if err != nil {
				return nil, err
			}
			arr.items = append(arr.items, child)
		}
		return arr, nil
	case map[string]interface{}:
		obj := newObject()
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys) // stable ordering for synthesized objects
		for _, k := range keys {
			child, err := nodeFromValue(t[k])
			if err != nil {
				return nil, err
			}
			obj.keys = append(obj.keys, k)
			obj.fields[k] = child
		}
		return obj, nil
	case map[interface{}]interface{}:
		// YAML decoder may emit this shape for nested maps.
		conv := make(map[string]interface{}, len(t))
		for k, val := range t {
			ks, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("non-string map key %v", k)
			}
			conv[ks] = val
		}
		return nodeFromValue(conv)
	default:
		// Last resort: round-trip via json.Marshal so any concrete type
		// with json tags still works. Loses ordering for objects.
		b, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("unsupported value type %T: %w", t, err)
		}
		return parseTree(b)
	}
}

// emit serializes the tree back to JSON, using `indent` (e.g. "  ") at
// each depth level. An empty indent emits compact JSON (no spaces /
// newlines).
func (n *node) emit(indent string) []byte {
	var buf bytes.Buffer
	emitNode(&buf, n, indent, 0)
	return buf.Bytes()
}

func emitNode(buf *bytes.Buffer, n *node, indent string, depth int) {
	switch n.kind {
	case nScalar:
		buf.Write(n.raw)
	case nObject:
		if len(n.keys) == 0 {
			buf.WriteString("{}")
			return
		}
		if indent == "" {
			buf.WriteByte('{')
			for i, k := range n.keys {
				if i > 0 {
					buf.WriteByte(',')
				}
				keyJSON, _ := json.Marshal(k)
				buf.Write(keyJSON)
				buf.WriteByte(':')
				emitNode(buf, n.fields[k], indent, depth+1)
			}
			buf.WriteByte('}')
			return
		}
		buf.WriteByte('{')
		buf.WriteByte('\n')
		for i, k := range n.keys {
			buf.WriteString(strings.Repeat(indent, depth+1))
			keyJSON, _ := json.Marshal(k)
			buf.Write(keyJSON)
			buf.WriteString(": ")
			emitNode(buf, n.fields[k], indent, depth+1)
			if i < len(n.keys)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(indent, depth))
		buf.WriteByte('}')
	case nArray:
		if len(n.items) == 0 {
			buf.WriteString("[]")
			return
		}
		if indent == "" {
			buf.WriteByte('[')
			for i, item := range n.items {
				if i > 0 {
					buf.WriteByte(',')
				}
				emitNode(buf, item, indent, depth+1)
			}
			buf.WriteByte(']')
			return
		}
		buf.WriteByte('[')
		buf.WriteByte('\n')
		for i, item := range n.items {
			buf.WriteString(strings.Repeat(indent, depth+1))
			emitNode(buf, item, indent, depth+1)
			if i < len(n.items)-1 {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString(strings.Repeat(indent, depth))
		buf.WriteByte(']')
	}
}

// detectIndent inspects the first nested element in the source bytes
// and returns the indent string (spaces or a tab). Returns "" for
// compact (single-line) JSON and "  " (two spaces) as a fallback when
// detection fails.
func detectIndent(data []byte) string {
	// Skip BOM if present.
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	for i := 0; i < len(data); i++ {
		c := data[i]
		if c == '{' || c == '[' {
			// Look for the first newline after the opening delim.
			rest := data[i+1:]
			nl := bytes.IndexByte(rest, '\n')
			if nl < 0 {
				return "" // compact
			}
			// Collect whitespace after the newline.
			j := nl + 1
			var ws []byte
			for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t') {
				ws = append(ws, rest[j])
				j++
			}
			if len(ws) == 0 {
				return ""
			}
			return string(ws)
		}
	}
	return "  "
}

// detectTrailingNewline returns true when the input ends with a newline
// byte. The emitter preserves this on round-trip.
func detectTrailingNewline(data []byte) bool {
	return len(data) > 0 && data[len(data)-1] == '\n'
}

// nodeEqual reports deep structural equality between two nodes. Used
// by the append_unique merge strategy to avoid duplicates.
func nodeEqual(a, b *node) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if a.kind != b.kind {
		return false
	}
	switch a.kind {
	case nScalar:
		return bytes.Equal(a.raw, b.raw)
	case nObject:
		if len(a.keys) != len(b.keys) {
			return false
		}
		for _, k := range a.keys {
			bv, ok := b.fields[k]
			if !ok {
				return false
			}
			if !nodeEqual(a.fields[k], bv) {
				return false
			}
		}
		return true
	case nArray:
		if len(a.items) != len(b.items) {
			return false
		}
		for i := range a.items {
			if !nodeEqual(a.items[i], b.items[i]) {
				return false
			}
		}
		return true
	}
	return false
}
