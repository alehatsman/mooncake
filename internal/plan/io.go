// Package plan provides plan generation and persistence for mooncake configurations.
package plan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/security"
)

// SavePlanToFile saves a plan to a file in JSON or YAML format.
//
// Before writing, the marshalled bytes go through redactSecretMarkers
// which rewrites any in-memory `\x00__MOONCAKE_SECRET__:env:FOO`
// sentinel back to a human-readable `!secret env:FOO` form. This is
// the spec-23 §3 plan-output redaction: the real secret value never
// makes it to disk (the marker carries only the *ref*, never the
// resolved value), but the marker itself looks like a control
// character so we rewrite it before serialization for readability.
func SavePlanToFile(p *Plan, filePath string) (err error) {
	ext := filepath.Ext(filePath)

	var buf bytes.Buffer
	switch ext {
	case ".json":
		encoder := json.NewEncoder(&buf)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(p); err != nil {
			return fmt.Errorf("failed to encode plan as JSON: %w", err)
		}
	case ".yaml", ".yml":
		encoder := yaml.NewEncoder(&buf)
		encoder.SetIndent(2)
		if err := encoder.Encode(p); err != nil {
			return fmt.Errorf("failed to encode plan as YAML: %w", err)
		}
	default:
		return fmt.Errorf("unsupported file format: %s (use .json, .yaml, or .yml)", ext)
	}

	out := redactSecretMarkers(buf.Bytes())

	file, err := os.Create(filePath) // #nosec G304 -- filePath is user-provided CLI argument
	if err != nil {
		return fmt.Errorf("failed to create plan file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close plan file: %w", closeErr)
		}
	}()
	if _, err := file.Write(out); err != nil {
		return fmt.Errorf("failed to write plan file: %w", err)
	}
	return nil
}

// redactSecretMarkers rewrites the sentinel-marker form used in-memory
// (security.SentinelPrefix + "<ref>") to the human-readable
// `!secret <ref>` form used in serialized plan output. Works on raw
// bytes after marshal so it covers JSON, YAML, and any future
// serialization format without per-format hooks.
//
// Note (v1): the rendered output keeps the *ref* (e.g. `env:APP_TOKEN`)
// so the user can debug "which secret was that supposed to be?". The
// spec acceptance criteria §247 want bare `!secret`; keeping the ref
// is a deliberate v1 deviation that prioritizes debuggability. Refs are
// not values — they don't leak credentials.
func redactSecretMarkers(in []byte) []byte {
	if !bytes.Contains(in, []byte(security.SentinelPrefix)) {
		return in
	}
	// strings.Replace-style pass: find every marker, take chars until the
	// next stop char (quote or newline in JSON/YAML output), rewrite.
	s := string(in)
	var out strings.Builder
	out.Grow(len(s))
	for {
		idx := strings.Index(s, security.SentinelPrefix)
		if idx < 0 {
			out.WriteString(s)
			return []byte(out.String())
		}
		out.WriteString(s[:idx])
		s = s[idx+len(security.SentinelPrefix):]
		// Consume the ref until we hit a delimiter that the serializer
		// would have placed (JSON closing quote, YAML newline, etc.).
		end := len(s)
		for i, r := range s {
			if r == '"' || r == '\n' || r == '\\' {
				end = i
				break
			}
		}
		ref := s[:end]
		s = s[end:]
		out.WriteString("!secret ")
		out.WriteString(ref)
	}
}

// LoadPlanFromFile loads a plan from a JSON or YAML file
func LoadPlanFromFile(filePath string) (*Plan, error) {
	data, err := os.ReadFile(filePath) // #nosec G304 -- filePath is user-provided CLI argument
	if err != nil {
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	ext := filepath.Ext(filePath)
	plan := &Plan{}

	switch ext {
	case ".json":
		if err := json.Unmarshal(data, plan); err != nil {
			return nil, fmt.Errorf("failed to decode JSON plan: %w", err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, plan); err != nil {
			return nil, fmt.Errorf("failed to decode YAML plan: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s (use .json, .yaml, or .yml)", ext)
	}

	return plan, nil
}
