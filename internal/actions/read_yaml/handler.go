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

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/read_common"
	"github.com/alehatsman/mooncake/internal/config"
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
	return read_common.RunRead(ctx, step.ReadYAML, parseYAML, "YAML")
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
