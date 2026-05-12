// Package template — list resolution.
//
// Shared resolution path for inputs that need to be a []string but may arrive
// as: a Go slice already in the variable map (e.g. `[]string` loaded by
// include_vars), a template expression that renders to a YAML/JSON list
// literal, a Pongo2-stringified Go slice ("[a b c]"), or a whitespace/comma
// separated scalar. Used by `with_items` (planner) and `package.names`.
package template

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/expression"
)

// ResolveList resolves a string expression to a list of items.
//
// Order of attempts (first match wins; later steps only run if earlier steps
// could not establish a binding for `expr`):
//  1. If `expr` is the exact name of a variable, require its value to be a
//     list (returns an error otherwise — strict, preserves with_items
//     semantics).
//  2. Otherwise evaluate via the expression evaluator (supports dot notation).
//     If evaluation succeeds, require the result to be a list.
//  3. If neither path bound `expr` to a value, parse the expression itself as
//     a literal list/scalar (handles rendered strings like "[a, b]" or
//     "a b c").
//
// The returned slice is []interface{} for compatibility with `with_items`,
// which uses interface{} per iteration item. Callers needing []string can use
// ResolveStringList.
func ResolveList(expr string, vars map[string]interface{}, evaluator expression.Evaluator) ([]interface{}, error) {
	if val, ok := vars[expr]; ok {
		return toSlice(val, expr)
	}

	// Only try the expression evaluator for identifier-like inputs
	// (e.g. "parameters.items"). Bracketed list literals look like
	// expression array syntax to expr-lang and would evaluate to a slice
	// of nils for undefined identifiers.
	if evaluator != nil && looksLikeIdentifier(expr) {
		if result, err := evaluator.Evaluate(expr, vars); err == nil {
			return toSlice(result, expr)
		}
	}

	return parseListLiteral(expr)
}

// looksLikeIdentifier reports whether the expression is a chain of
// identifiers separated by dots — the only shape the planner uses for
// `with_items` dot-notation lookups.
func looksLikeIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '.' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// ResolveStringList resolves a string expression to []string. Non-string items
// are formatted with %v.
func ResolveStringList(expr string, vars map[string]interface{}, evaluator expression.Evaluator) ([]string, error) {
	items, err := ResolveList(expr, vars, evaluator)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(items))
	for i, item := range items {
		switch v := item.(type) {
		case string:
			out[i] = v
		default:
			out[i] = fmt.Sprintf("%v", v)
		}
	}
	return out, nil
}

// toSlice converts a value into []interface{}. The expr argument is used in
// the error message to identify which input produced the type mismatch.
func toSlice(val interface{}, expr string) ([]interface{}, error) {
	switch v := val.(type) {
	case []interface{}:
		return v, nil
	case []string:
		items := make([]interface{}, len(v))
		for i, s := range v {
			items[i] = s
		}
		return items, nil
	default:
		return nil, fmt.Errorf("with_items expression %q is not a list (got %T)", expr, val)
	}
}

// parseListLiteral parses a rendered string into a list. Accepts:
//   - JSON array: `["a","b"]`
//   - YAML flow sequence with bare scalars: `[a, b, c]`
//   - Pongo2-stringified slice: `[a b c]` (space-separated, no quotes)
//   - Whitespace- or comma-separated scalar: `a b c` or `a,b,c`
//
// Order of attempts is JSON-first (preserves quoted strings), then a
// bracket-strip-and-split fallback that handles both Pongo2 and YAML flow
// forms with bare scalars uniformly.
func parseListLiteral(s string) ([]interface{}, error) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil, fmt.Errorf("empty list expression")
	}

	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		var jsonItems []interface{}
		if err := json.Unmarshal([]byte(trimmed), &jsonItems); err == nil {
			return jsonItems, nil
		}

		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		return splitScalar(inner), nil
	}

	return splitScalar(trimmed), nil
}

// splitScalar splits a string on whitespace or commas, dropping empties.
func splitScalar(s string) []interface{} {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
	items := make([]interface{}, 0, len(fields))
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if f != "" {
			items = append(items, f)
		}
	}
	return items
}
