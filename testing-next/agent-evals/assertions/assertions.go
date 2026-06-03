// Package assertions implements the agent-eval assertion grammar.
//
// Each assertion is a single line in a goal file. The grammar is
// deliberately small: an unknown form fails to parse, surfacing
// typos immediately instead of silently passing. See README in the
// parent directory for the supported forms.
package assertions

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"gopkg.in/yaml.v3"
)

// Step is the LLM-emitted plan step shape. We decode into a free-form
// map because we want to inspect arbitrary fields without coupling
// the harness to the kernel's typed Step struct (which evolves and
// would tie the harness to whatever version of internal/config is
// in tree).
type Step = map[string]interface{}

// universalFields are step-level keys that are NOT the action verb.
// Anything in a step that is not in this set is treated as the
// action key. Kept in sync conceptually with config.Step's universal
// fields (~36 today, see arch soft-cap #2 in CLAUDE.md).
var universalFields = map[string]struct{}{
	"name":          {},
	"when":          {},
	"loop":          {},
	"loop_var":      {},
	"tags":          {},
	"register":      {},
	"changed_when":  {},
	"failed_when":   {},
	"vars":          {},
	"ignore_errors": {},
	"retry":         {},
	"timeout":       {},
	"transaction":   {},
	"check":         {},
	"check_mode":    {},
	"diff":          {},
	"become":        {},
	"become_user":   {},
	"environment":   {},
	"async":         {},
	"poll":          {},
	"no_log":        {},
	"delegate_to":   {},
	"run_once":      {},
	"notify":        {},
	"handler":       {},
	// #111 generic carrier: `action:` names the verb (its value), `with:`
	// carries its params — neither is itself the action key.
	"action": {},
	"with":   {},
}

// Assertion is one parsed check that can run against a plan YAML.
type Assertion interface {
	Check(planYAML string, steps []Step) error
	String() string
}

// Parse parses one assertion line into a typed Assertion.
//
// Unknown forms return an error rather than a stub-pass assertion —
// a typo in a goal file should fail loudly at parse time, not
// pretend to pass at run time.
func Parse(line string) (Assertion, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty assertion")
	}

	fields := strings.Fields(line)
	switch fields[0] {
	case "schema_valid":
		if len(fields) != 1 {
			return nil, fmt.Errorf("schema_valid takes no arguments, got %d", len(fields)-1)
		}
		return schemaValid{}, nil

	case "contains_step":
		if len(fields) != 2 {
			return nil, fmt.Errorf("contains_step requires exactly one action name (got %q)", line)
		}
		return containsStep{action: fields[1]}, nil

	case "no_step":
		if len(fields) != 2 {
			return nil, fmt.Errorf("no_step requires exactly one action name (got %q)", line)
		}
		return noStep{action: fields[1]}, nil

	case "contains_step_with":
		if len(fields) < 3 {
			return nil, fmt.Errorf("contains_step_with requires <action> <field>=<substring> (got %q)", line)
		}
		action := fields[1]
		kv := strings.Join(fields[2:], " ")
		eq := strings.Index(kv, "=")
		if eq <= 0 {
			return nil, fmt.Errorf("contains_step_with: expected <field>=<substring>, got %q", kv)
		}
		return containsStepWith{
			action: action,
			field:  strings.TrimSpace(kv[:eq]),
			needle: strings.TrimSpace(kv[eq+1:]),
		}, nil

	case "step_count":
		if len(fields) != 3 {
			return nil, fmt.Errorf("step_count requires <op> <N> (e.g. step_count <= 10), got %q", line)
		}
		n, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("step_count: cannot parse N=%q: %w", fields[2], err)
		}
		switch fields[1] {
		case "<=":
			return stepCountAtMost{n: n}, nil
		case ">=":
			return stepCountAtLeast{n: n}, nil
		case "==":
			return stepCountEqual{n: n}, nil
		default:
			return nil, fmt.Errorf("step_count: operator must be <=, >=, or ==, got %q", fields[1])
		}
	}

	return nil, fmt.Errorf("unknown assertion form: %q", line)
}

// ParseAll parses every line; returns the first parse error if any.
func ParseAll(lines []string) ([]Assertion, error) {
	out := make([]Assertion, 0, len(lines))
	for i, l := range lines {
		a, err := Parse(l)
		if err != nil {
			return nil, fmt.Errorf("assertion[%d] %q: %w", i, l, err)
		}
		out = append(out, a)
	}
	return out, nil
}

// ParsePlan decodes a plan YAML emitted by the LLM into a slice of
// steps. Tolerates both list-shape (legacy) and RunConfig-shape (new
// format with `steps:`). Strips markdown code fences if the model
// emitted them — the prompt forbids fences but real models sometimes
// disobey, and the harness should report assertion failures, not
// parse failures, when that happens.
func ParsePlan(planYAML string) ([]Step, error) {
	planYAML = stripCodeFences(planYAML)

	var asList []Step
	if err := yaml.Unmarshal([]byte(planYAML), &asList); err == nil && len(asList) > 0 {
		return asList, nil
	}

	var asConfig struct {
		Steps []Step `yaml:"steps"`
	}
	if err := yaml.Unmarshal([]byte(planYAML), &asConfig); err == nil && len(asConfig.Steps) > 0 {
		return asConfig.Steps, nil
	}

	return nil, fmt.Errorf("plan did not parse as a step list or a RunConfig with steps")
}

func stripCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) < 2 {
		return s
	}
	lines = lines[1:]
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// stepAction extracts the action verb from a step. Returns "" if the
// step has only universal fields (which would be a malformed step).
func stepAction(s Step) string {
	// #111 generic carrier: the verb is the *value* of `action:`, not a key.
	if v, ok := s["action"]; ok {
		if name, ok := v.(string); ok && name != "" {
			return name
		}
	}
	for k := range s {
		if _, universal := universalFields[k]; !universal {
			return k
		}
	}
	return ""
}

// --- individual assertion types -----------------------------------------

type schemaValid struct{}

func (schemaValid) String() string { return "schema_valid" }
func (schemaValid) Check(planYAML string, _ []Step) error {
	// Write to a tempfile and use the same reader the live executor
	// uses — that way we exercise the real validation path, including
	// the "forgot the leading dash" hint and secret-tag rewrites.
	f, err := os.CreateTemp("", "agent-eval-plan-*.yml")
	if err != nil {
		return fmt.Errorf("tempfile: %w", err)
	}
	path := f.Name()
	defer func() { _ = os.Remove(path) }()
	if _, err := f.WriteString(stripCodeFences(planYAML)); err != nil {
		_ = f.Close()
		return fmt.Errorf("write tempfile: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close tempfile: %w", err)
	}

	_, diags, err := config.ReadConfigWithValidation(path, nil)
	if err != nil {
		return fmt.Errorf("read+validate: %w", err)
	}
	if config.HasErrors(diags) {
		// Surface only the first error to keep the report readable.
		for _, d := range diags {
			if d.Severity == "error" {
				return fmt.Errorf("%s:%d: %s", filepath.Base(path), d.Line, d.Message)
			}
		}
	}
	return nil
}

type containsStep struct{ action string }

func (c containsStep) String() string { return "contains_step " + c.action }
func (c containsStep) Check(_ string, steps []Step) error {
	for _, s := range steps {
		if stepAction(s) == c.action {
			return nil
		}
	}
	return fmt.Errorf("no step with action %q in plan", c.action)
}

type noStep struct{ action string }

func (n noStep) String() string { return "no_step " + n.action }
func (n noStep) Check(_ string, steps []Step) error {
	for i, s := range steps {
		if stepAction(s) == n.action {
			return fmt.Errorf("found forbidden action %q at step %d", n.action, i)
		}
	}
	return nil
}

type containsStepWith struct {
	action string
	field  string
	needle string
}

func (c containsStepWith) String() string {
	return fmt.Sprintf("contains_step_with %s %s=%s", c.action, c.field, c.needle)
}
func (c containsStepWith) Check(_ string, steps []Step) error {
	for _, s := range steps {
		if stepAction(s) != c.action {
			continue
		}
		// Typed steps carry params under the action key; #111 carrier steps
		// carry them under with:.
		key := c.action
		if _, carrier := s["action"]; carrier {
			key = "with"
		}
		body, ok := s[key].(map[string]interface{})
		if !ok {
			// Action value isn't a map — e.g. `shell: ls -la`. Fall back
			// to stringifying and matching the needle there.
			if v, ok := s[key].(string); ok && strings.Contains(v, c.needle) {
				return nil
			}
			continue
		}
		v, ok := body[c.field]
		if !ok {
			continue
		}
		if strings.Contains(fmt.Sprintf("%v", v), c.needle) {
			return nil
		}
	}
	return fmt.Errorf("no %s step has %s containing %q", c.action, c.field, c.needle)
}

type stepCountAtMost struct{ n int }

func (s stepCountAtMost) String() string { return fmt.Sprintf("step_count <= %d", s.n) }
func (s stepCountAtMost) Check(_ string, steps []Step) error {
	if len(steps) > s.n {
		return fmt.Errorf("plan has %d steps, max allowed %d", len(steps), s.n)
	}
	return nil
}

type stepCountAtLeast struct{ n int }

func (s stepCountAtLeast) String() string { return fmt.Sprintf("step_count >= %d", s.n) }
func (s stepCountAtLeast) Check(_ string, steps []Step) error {
	if len(steps) < s.n {
		return fmt.Errorf("plan has %d steps, min required %d", len(steps), s.n)
	}
	return nil
}

type stepCountEqual struct{ n int }

func (s stepCountEqual) String() string { return fmt.Sprintf("step_count == %d", s.n) }
func (s stepCountEqual) Check(_ string, steps []Step) error {
	if len(steps) != s.n {
		return fmt.Errorf("plan has %d steps, expected exactly %d", len(steps), s.n)
	}
	return nil
}
