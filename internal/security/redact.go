package security

import (
	"sort"
	"strings"
	"sync"
)

// Redactor provides thread-safe string redaction for sensitive values
type Redactor struct {
	sensitiveValues []string
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

	return result
}
