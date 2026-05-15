// Package read_json implements the `read.json` tier-1 action (spec-38).
//
// Reads a JSON file, optionally extracts a value by pathquery path, and
// publishes the result under the step's `as:` name. Read-only by
// contract: no Changed=true, no system mutation; CaptureInPlan=true so
// plan-mode runs publish the value too.
package read_json

import (
	"encoding/json"
	"fmt"
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

func parseJSON(data []byte, dst *any) error {
	return json.Unmarshal(data, dst)
}
