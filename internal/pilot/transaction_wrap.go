package pilot

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// transactionWrapName is the Name field set on pilot's implicit
// transaction wrapper. Surfaced in event streams and the audit trail so
// operators can tell which step boundary belongs to pilot vs. their
// own authored steps.
const transactionWrapName = "pilot apply"

// wrappedPlan is the on-the-wire shape pilot emits to the executor.
// Distinct from config.RunConfig because its Version and Vars fields
// carry the omitempty tag — without it, an empty pilot plan would
// marshal `version: ""` and `vars: {}` and dirty the YAML the operator
// inspects via the audit trail.
type wrappedPlan struct {
	Version string                 `yaml:"version,omitempty"`
	Vars    map[string]interface{} `yaml:"vars,omitempty"`
	Steps   []config.Step          `yaml:"steps"`
}

// WrapInTransaction takes sanitized plan YAML (the LLM's output, fences
// stripped) and returns YAML where the original top-level steps are
// folded into a single transaction step. allow_irreversible is set true
// so the plan-time check accepts steps whose handler doesn't implement
// Reverser — the operator was warned about those at the plan-confirm
// gate (spec-67 §11).
//
// Accepts both supported input shapes: a bare list-of-steps and a
// structured RunConfig with version/vars/steps. The output is always
// the structured shape. An empty step list is passed through unchanged.
func WrapInTransaction(planBytes []byte) ([]byte, error) {
	steps, version, vars, err := decodePlan(planBytes)
	if err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return planBytes, nil
	}

	wrapped := wrappedPlan{
		Version: version,
		Vars:    vars,
		Steps: []config.Step{
			{
				Name:              transactionWrapName,
				Transaction:       steps,
				AllowIrreversible: true,
			},
		},
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&wrapped); err != nil {
		return nil, fmt.Errorf("marshal wrapped plan: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("close encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// CountIrreversibleSteps returns the number of top-level steps in the
// plan whose handler does not implement actions.Reverser. The
// confirm-gate (spec-67 §10) consumes this for its recap line.
//
// An unknown action is conservatively counted as irreversible — the
// gate should warn the operator about anything the executor can't roll
// back, and unknown actions also fail plan validation downstream, so
// the count being non-zero is the right signal either way.
func CountIrreversibleSteps(planBytes []byte) (int, error) {
	steps, _, _, err := decodePlan(planBytes)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, s := range steps {
		actionType := s.DetermineActionType()
		if actionType == "" {
			continue
		}
		h, ok := actions.Get(actionType)
		if !ok {
			count++
			continue
		}
		if _, ok := h.(actions.Reverser); !ok {
			count++
		}
	}
	return count, nil
}

// decodePlan unmarshals plan YAML in either supported shape and returns
// the top-level steps plus (when present) the RunConfig version and
// vars. Mirrors the dispatch in internal/config/reader.go.
func decodePlan(planBytes []byte) ([]config.Step, string, map[string]interface{}, error) {
	var rootNode yaml.Node
	if err := yaml.Unmarshal(planBytes, &rootNode); err != nil {
		return nil, "", nil, fmt.Errorf("parse plan: %w", err)
	}
	if isSequenceRoot(&rootNode) {
		var steps []config.Step
		if err := rootNode.Decode(&steps); err != nil {
			return nil, "", nil, fmt.Errorf("decode plan as steps array: %w", err)
		}
		return steps, "", nil, nil
	}
	var rc config.RunConfig
	if err := rootNode.Decode(&rc); err != nil {
		return nil, "", nil, fmt.Errorf("decode plan as RunConfig: %w", err)
	}
	return rc.Steps, rc.Version, rc.Vars, nil
}

// isSequenceRoot returns true when the YAML doc's root content is a
// sequence (bare list of steps) vs. a mapping (structured RunConfig).
func isSequenceRoot(node *yaml.Node) bool {
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return node.Content[0].Kind == yaml.SequenceNode
	}
	return node.Kind == yaml.SequenceNode
}
