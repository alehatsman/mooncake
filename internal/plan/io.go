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

	// F056: 0o600 perms (was bare os.Create → 0644 under typical
	// umask). Plans carry secret refs + full playbook structure;
	// on a multi-user host the contents leak. F037 family —
	// pilot.RunLoop's saved plans were already fixed; this path
	// was missed.
	//
	// Atomic write (write to <path>.tmp, then rename): a failure
	// mid-write previously left a partial/empty plan at the
	// destination, which `apply --from-plan` would refuse with a
	// confusing decode error. With rename, the destination is
	// either the pre-existing file (untouched by this call) or
	// the fully-written new bytes — never partial.
	tmpPath := filePath + ".tmp"
	file, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- filePath is user-provided CLI argument
	if err != nil {
		return fmt.Errorf("failed to create plan file: %w", err)
	}
	// Cleanup the temp file on any error path. Successful rename
	// moves it out so the deferred Remove no-ops (file already
	// gone); on a write/close failure the temp file would otherwise
	// be orphaned next to the destination.
	renamed := false
	defer func() {
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()
	if _, writeErr := file.Write(out); writeErr != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write plan file: %w", writeErr)
	}
	if closeErr := file.Close(); closeErr != nil {
		return fmt.Errorf("failed to close plan file: %w", closeErr)
	}
	if renameErr := os.Rename(tmpPath, filePath); renameErr != nil {
		return fmt.Errorf("failed to rename plan file into place: %w", renameErr)
	}
	renamed = true
	return nil
}

// redactSecretMarkers rewrites the sentinel-marker form used in-memory
// (security.SentinelPrefix + "<ref>") to a human-readable form in
// serialized plan output. Works on raw bytes after marshal so it
// covers JSON, YAML, and any future serialization format without
// per-format hooks.
//
// Default output matches spec-23 §247 — bare `!secret` with the ref
// stripped. Operators debugging "which secret was supposed to be
// there?" can set MOONCAKE_SHOW_SECRET_REFS=1 to keep the ref in the
// output as `!secret <ref>`. Refs are not values and don't leak
// credentials, but keeping them out of plan output by default avoids
// any surprise when plans get shared in bug reports or attached to PRs.
func redactSecretMarkers(in []byte) []byte {
	if !bytes.Contains(in, []byte(security.SentinelPrefix)) {
		return in
	}
	showRef := os.Getenv("MOONCAKE_SHOW_SECRET_REFS") == "1"
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
		if showRef {
			out.WriteString("!secret ")
			out.WriteString(ref)
		} else {
			out.WriteString("!secret")
		}
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
		// F048 family: strict decode so a typo'd plan field
		// surfaces as an error instead of silently dropping.
		// Stage a decoder with DisallowUnknownFields so the
		// behavior matches the YAML branch below.
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(plan); err != nil {
			return nil, fmt.Errorf("failed to decode JSON plan: %w", err)
		}
	case ".yaml", ".yml":
		// F048 family: strict YAML decode. yaml.v3's KnownFields(true)
		// rejects unknown top-level keys instead of silently
		// dropping. A user-edited plan with a typo'd field (e.g.
		// `step:` instead of `steps:`) now errors at read time
		// rather than producing an apply that silently does the
		// wrong thing.
		dec := yaml.NewDecoder(bytes.NewReader(data))
		dec.KnownFields(true)
		if err := dec.Decode(plan); err != nil {
			return nil, fmt.Errorf("failed to decode YAML plan: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported file format: %s (use .json, .yaml, or .yml)", ext)
	}

	return plan, nil
}
