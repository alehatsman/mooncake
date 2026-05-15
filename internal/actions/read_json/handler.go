// Package read_json implements the `read.json` tier-1 action (spec-38).
//
// Reads a JSON file, optionally extracts a value by pathquery path, and
// publishes the result under the step's `as:` name. Read-only by
// contract: no Changed=true, no system mutation; CaptureInPlan=true so
// plan-mode runs publish the value too.
package read_json

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/read_common"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "read.json"

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Read a JSON file and optionally extract a value by path",
		Category:           actions.CategoryData,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        nil,
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true, // spec-37: side-effect-free read; bind in plan mode
	}
}

func (Handler) Validate(step *config.Step) error {
	return read_common.Validate(step.ReadJSON, actionName)
}

func (h Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rf := step.ReadJSON
	patterns, err := read_common.CompileRedactPatterns(rf.Redact)
	if err != nil {
		return nil, err
	}
	maxBytes := int64(0)
	if rf.MaxBytes != nil {
		maxBytes = *rf.MaxBytes
	}

	result := executor.NewResult()
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	out, err := read_common.Read(ctx, read_common.Opts{
		Path:       rf.Path,
		Query:      rf.Query,
		MaxBytes:   maxBytes,
		Redact:     patterns,
		Parse:      parseJSON,
		FormatName: "JSON",
	})
	if err != nil {
		return result, err
	}

	result.SetData(map[string]any{
		"path":       out.Path,
		"query":      out.Query,
		"found":      out.Found,
		"value":      out.Value,
		"bytes_read": out.BytesRead,
	})
	result.Changed = false
	if ctx.Mode() == actions.ModePlan {
		result.Checkable = true
		if out.Found {
			result.Reason = fmt.Sprintf("would read %d bytes from %s", out.BytesRead, out.Path)
		} else {
			result.Reason = fmt.Sprintf("would read %d bytes from %s; query path missed", out.BytesRead, out.Path)
		}
	}
	return result, nil
}

// parseJSON decodes JSON into dst, preserving the integer/float
// distinction Go's default decoder loses (MT-79). Without this,
// `{"port": 8080}` round-trips as `8080.000000` via float64 — ugly
// in templates and inconsistent with read.yaml, which preserves int
// types natively.
//
// Strategy: decode with json.Decoder.UseNumber() so every numeric
// becomes a json.Number, then walk the tree converting each
// json.Number to int64 when it has no fractional/exponent part,
// float64 otherwise. Existing string/bool/map/slice/null values pass
// through unchanged.
func parseJSON(data []byte, dst *any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw any
	if err := dec.Decode(&raw); err != nil {
		return err
	}
	*dst = normalizeJSONNumbers(raw)
	return nil
}

// normalizeJSONNumbers converts json.Number values inside v to int64
// or float64 depending on whether the source literal had a decimal
// point or exponent. Walks maps and slices recursively.
func normalizeJSONNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		s := x.String()
		// Integer when no '.' and no exponent. Use strconv to confirm
		// the value fits — JSON integers larger than int64 fall back
		// to float64 to avoid wraparound surprises.
		hasFractional := false
		for i := 0; i < len(s); i++ {
			c := s[i]
			if c == '.' || c == 'e' || c == 'E' {
				hasFractional = true
				break
			}
		}
		if !hasFractional {
			if n, err := strconv.ParseInt(s, 10, 64); err == nil {
				return n
			}
		}
		// Fall back to float64 (default JSON decode shape).
		if f, err := x.Float64(); err == nil {
			return f
		}
		// Last resort: keep the raw string so consumers can branch
		// rather than seeing a zero value.
		return s
	case map[string]any:
		for k, vv := range x {
			x[k] = normalizeJSONNumbers(vv)
		}
		return x
	case []any:
		for i, vv := range x {
			x[i] = normalizeJSONNumbers(vv)
		}
		return x
	default:
		return v
	}
}
