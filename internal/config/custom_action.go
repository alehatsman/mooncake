package config

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// IsCustomAction reports whether name is a registered custom action — one
// admitted by the run's action registry that has no dedicated typed Step
// field (e.g. "notify.webhook"). It lets the parser fold a typed-key custom
// action into the generic #111 carrier so it validates and dispatches exactly
// like a built-in.
//
// A nil predicate means "no custom actions": every key that is not a known
// Step field stays unknown, preserving the strict typo diagnostics. This is
// the default for callers that have no registry (CLI validate, doctor).
//
// The registry is passed as this minimal predicate rather than as an
// *actions.Registry because config cannot import actions — that would be an
// import cycle (actions depends on config). The executor and agent, which
// hold the registry, pass its Has method.
type IsCustomAction func(name string) bool

// normalizeCustomActionSteps rewrites every typed-key custom action in the
// parsed YAML node tree into the generic carrier form, in place:
//
//   - notify.webhook:          - action: notify.webhook
//     url: …          →         with:
//     method: POST                url: …
//     method: POST
//
// A step key is folded only when it is NOT a known Step field AND isCustom
// reports it as a registered custom action. Running this before strict-field
// validation and decode means the rest of the pipeline (both validators,
// DetermineActionType, executor dispatch) only ever sees carrier form and is
// untouched. Keys that are neither known fields nor registered actions are
// left alone for the unknown-field validator to flag — typo-catching is
// preserved (a misspelled `notifyy.webhook:` still errors).
//
// Returns whether it changed the tree; no-op (false) when isCustom is nil.
func normalizeCustomActionSteps(root *yaml.Node, isCustom IsCustomAction) bool {
	if isCustom == nil || root == nil {
		return false
	}
	node := root
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return false
		}
		node = node.Content[0]
	}
	switch node.Kind {
	case yaml.SequenceNode:
		// Plain array of steps (old format).
		return normalizeStepSeq(node, isCustom)
	case yaml.MappingNode:
		// RunConfig: top-level `steps:` plus every named task's `steps:`.
		changed := normalizeStepSeq(mappingValue(node, "steps"), isCustom)
		if tasks := mappingValue(node, "tasks"); tasks != nil && tasks.Kind == yaml.MappingNode {
			for i := 1; i < len(tasks.Content); i += 2 {
				if normalizeStepSeq(mappingValue(tasks.Content[i], "steps"), isCustom) {
					changed = true
				}
			}
		}
		return changed
	}
	return false
}

// normalizeStepSeq folds every step in a step-list sequence node.
func normalizeStepSeq(seq *yaml.Node, isCustom IsCustomAction) bool {
	if seq == nil || seq.Kind != yaml.SequenceNode {
		return false
	}
	changed := false
	for _, step := range seq.Content {
		if normalizeStep(step, isCustom) {
			changed = true
		}
	}
	return changed
}

// normalizeStep folds a typed-key custom action at this step into the carrier
// and recurses into any nested step lists (transaction / try / catch /
// finally / on_change / on_rollback / heal — derived from the struct, so new
// compound types are covered automatically).
func normalizeStep(step *yaml.Node, isCustom IsCustomAction) bool {
	if step == nil || step.Kind != yaml.MappingNode {
		return false
	}
	changed := false
	out := make([]*yaml.Node, 0, len(step.Content))
	for i := 0; i+1 < len(step.Content); i += 2 {
		keyNode, valNode := step.Content[i], step.Content[i+1]
		key := keyNode.Value
		switch {
		case stepListFields[key]:
			if normalizeStepSeq(valNode, isCustom) {
				changed = true
			}
			out = append(out, keyNode, valNode)
		case !stepFieldNames[key] && isCustom(key):
			// Fold `name: {params}` → `action: name` + `with: {params}`.
			out = append(out, scalarNode("action"), scalarNode(key), scalarNode("with"), valNode)
			changed = true
		default:
			out = append(out, keyNode, valNode)
		}
	}
	step.Content = out
	return changed
}

// NormalizePlanBytes folds typed-key custom actions in raw plan bytes into the
// generic carrier and returns the re-encoded YAML. Callers that decode a plan
// into typed config.Step values before it reaches a reader (e.g. the agent's
// transaction wrap) must run this first: a typed decode silently drops any key
// without a dedicated Step field, destroying the typed-key form.
//
// The input is returned UNCHANGED — byte-for-byte — when isCustom is nil or no
// step needed folding, so a plan that uses only built-ins (the common case)
// never gets reflowed and its hash is stable.
func NormalizePlanBytes(planBytes []byte, isCustom IsCustomAction) ([]byte, error) {
	if isCustom == nil {
		return planBytes, nil
	}
	var root yaml.Node
	if err := yaml.Unmarshal(planBytes, &root); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	if !normalizeCustomActionSteps(&root, isCustom) {
		return planBytes, nil
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&root); err != nil {
		return nil, fmt.Errorf("normalize plan: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("normalize plan: %w", err)
	}
	return buf.Bytes(), nil
}

// mappingValue returns the value node for key in a mapping node, or nil.
func mappingValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// scalarNode builds a plain string scalar node for a synthesized carrier key.
func scalarNode(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

// stepFieldNames is the set of YAML keys recognized as Step fields — every
// built-in action plus the universal step fields (name/when/as/…). Derived
// once from the struct tags so it never drifts as fields are added. A key
// outside this set is a candidate typed-key custom action.
var stepFieldNames = func() map[string]bool {
	m := make(map[string]bool, stepType.NumField())
	for i := 0; i < stepType.NumField(); i++ {
		if name := yamlFieldName(stepType.Field(i)); name != "" {
			m[name] = true
		}
	}
	return m
}()

// stepListFields is the subset of Step fields whose value is a nested list of
// steps (transaction, try, catch, finally, on_change, on_rollback, heal).
// Custom actions can appear inside these, so normalization recurses into them.
//
// The boundary is deliberate: folding follows the control-flow compounds a
// Step holds DIRECTLY ([]Step fields). It does not descend into typed
// sub-structs that happen to carry steps (e.g. artifact.capture) — a
// typed-key custom there is not silently dropped but surfaces a clear
// unknown-field error, with the carrier available as the escape hatch. This
// keeps the rule simple and free of false positives inside opaque `with:`
// params, which a key-name-matching walk would risk.
var stepListFields = func() map[string]bool {
	sliceOfStep := reflect.TypeOf([]Step(nil))
	m := make(map[string]bool)
	for i := 0; i < stepType.NumField(); i++ {
		f := stepType.Field(i)
		if f.Type == sliceOfStep {
			if name := yamlFieldName(f); name != "" {
				m[name] = true
			}
		}
	}
	return m
}()

// yamlFieldName returns the YAML key for a struct field (the tag name with any
// ",omitempty"-style options stripped), or "" for an unexported or `yaml:"-"`
// field.
func yamlFieldName(f reflect.StructField) string {
	tag := f.Tag.Get("yaml")
	if tag == "" || tag == "-" {
		return ""
	}
	if i := strings.IndexByte(tag, ','); i >= 0 {
		tag = tag[:i]
	}
	return tag
}
