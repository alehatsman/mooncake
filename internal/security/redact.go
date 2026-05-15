package security

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Redactor provides thread-safe string redaction for sensitive values
type Redactor struct {
	sensitiveValues []string
	patterns        []*regexp.Regexp
	mu              sync.RWMutex
}

// NewRedactor creates a new Redactor instance
func NewRedactor() *Redactor {
	return &Redactor{
		sensitiveValues: make([]string, 0),
	}
}

// AddSensitive adds a sensitive value to be redacted
// Empty strings are ignored
func (r *Redactor) AddSensitive(value string) {
	if value == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sensitiveValues = append(r.sensitiveValues, value)
	// Sort by length (longest first) for proper substring matching
	sort.Slice(r.sensitiveValues, func(i, j int) bool {
		return len(r.sensitiveValues[i]) > len(r.sensitiveValues[j])
	})
}

// AddPattern compiles and registers a regex pattern. String leaves of
// values passed to RedactValue (and substrings of strings passed to
// Redact) that match the pattern are replaced with [REDACTED]. Returns
// the compilation error on invalid patterns. See spec-38.
func (r *Redactor) AddPattern(pattern string) error {
	if pattern == "" {
		return nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("redact: invalid pattern %q: %w", pattern, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.patterns = append(r.patterns, re)
	return nil
}

// Redact replaces all occurrences of sensitive values with [REDACTED]
func (r *Redactor) Redact(text string) string {
	if text == "" {
		return text
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := text
	for _, sensitive := range r.sensitiveValues {
		if sensitive != "" {
			result = strings.ReplaceAll(result, sensitive, "[REDACTED]")
		}
	}
	for _, re := range r.patterns {
		result = re.ReplaceAllString(result, "[REDACTED]")
	}

	return result
}

// RedactValue walks an arbitrary value (any combination of map, slice,
// string, number, bool, nil) and returns a deep-redacted copy. String
// leaves are passed through Redact; non-string leaves are returned
// unchanged. Map keys are NOT redacted — rewriting keys would break
// downstream shape contracts. Short-circuits to v when no sensitives or
// patterns are configured.
func (r *Redactor) RedactValue(v any) any {
	r.mu.RLock()
	noWork := len(r.sensitiveValues) == 0 && len(r.patterns) == 0
	r.mu.RUnlock()
	if noWork {
		return v
	}
	return r.redactValue(v)
}

func (r *Redactor) redactValue(v any) any {
	switch x := v.(type) {
	case string:
		return r.Redact(x)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = r.redactValue(vv)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = r.redactValue(vv)
		}
		return out
	default:
		return v
	}
}
