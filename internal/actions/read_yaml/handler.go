// Package read_yaml implements the `read.yaml` tier-1 action (spec-38).
//
// Sibling of `read.json`; only the parser differs. Multi-document YAML is
// rejected at parse-time per spec-38 Open Q3 — multi-doc support is a
// separate feature if real demand appears.
package read_yaml

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/read_common"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const actionName = "read.yaml"

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Read a YAML file and optionally extract a value by path",
		Category:           actions.CategoryData,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        nil,
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (Handler) Validate(step *config.Step) error {
	return read_common.Validate(step.ReadYAML, actionName)
}

func (h Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rf := step.ReadYAML
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
		Parse:      parseYAML,
		FormatName: "YAML",
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

// parseYAML decodes the first document, then refuses to continue if a
// second document follows (spec-38 Open Q3).
func parseYAML(data []byte, dst *any) error {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(dst); err != nil {
		return err
	}
	var second any
	switch err := dec.Decode(&second); {
	case errors.Is(err, io.EOF):
		return nil
	case err == nil:
		return fmt.Errorf("multi-document YAML not supported (use a single document; multi-doc support is a separate feature)")
	default:
		// A parse error reading the trailing document — surface it.
		return fmt.Errorf("trailing-document parse: %w", err)
	}
}
