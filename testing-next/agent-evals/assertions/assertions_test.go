package assertions

import (
	"strings"
	"testing"
)

func TestParse_ValidForms(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"schema_valid", "schema_valid"},
		{"contains_step shell", "contains_step shell"},
		{"no_step shell", "no_step shell"},
		{"contains_step_with file_replace path=/etc/hosts", "contains_step_with file_replace path=/etc/hosts"},
		{"step_count <= 10", "step_count <= 10"},
		{"step_count >= 2", "step_count >= 2"},
		{"step_count == 5", "step_count == 5"},
	}
	for _, tc := range cases {
		a, err := Parse(tc.line)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", tc.line, err)
			continue
		}
		if got := a.String(); got != tc.want {
			t.Errorf("Parse(%q).String() = %q, want %q", tc.line, got, tc.want)
		}
	}
}

func TestParse_InvalidForms(t *testing.T) {
	// Unknown forms and shape mismatches must fail at parse time —
	// the harness intentionally does not silently pass typos.
	cases := []string{
		"",
		"bogus",
		"contains_step",                  // missing action
		"contains_step shell file",       // too many args
		"contains_step_with shell",       // missing field=value
		"contains_step_with shell field", // field without =value
		"step_count 10",                  // missing operator
		"step_count != 10",               // unsupported operator
		"step_count <= ten",              // non-numeric N
	}
	for _, line := range cases {
		if _, err := Parse(line); err == nil {
			t.Errorf("Parse(%q) returned nil error; want parse failure", line)
		}
	}
}

func TestContainsStep(t *testing.T) {
	plan := `
- name: hi
  shell: echo hello
- file_replace:
    path: /etc/hosts
    old_string: foo
    new_string: bar
`
	steps, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	a, _ := Parse("contains_step file_replace")
	if err := a.Check(plan, steps); err != nil {
		t.Errorf("contains_step file_replace should pass, got: %v", err)
	}
	miss, _ := Parse("contains_step assert")
	if err := miss.Check(plan, steps); err == nil {
		t.Error("contains_step assert should fail on a plan with no assert step")
	}
}

func TestContainsStepWith(t *testing.T) {
	plan := `
- file_replace:
    path: /etc/ssh/sshd_config
    old_string: PermitRootLogin prohibit-password
    new_string: PermitRootLogin no
- shell: systemctl reload ssh
`
	steps, _ := ParsePlan(plan)

	hit, _ := Parse("contains_step_with file_replace path=sshd_config")
	if err := hit.Check(plan, steps); err != nil {
		t.Errorf("substring match on path should pass, got: %v", err)
	}

	miss, _ := Parse("contains_step_with file_replace path=/etc/hosts")
	if err := miss.Check(plan, steps); err == nil {
		t.Error("non-matching path should fail")
	}

	// shell as scalar (not a map) — needle should match the scalar value.
	scalar, _ := Parse("contains_step_with shell cmd=systemctl")
	// Note: 'cmd' field is irrelevant when value is a scalar; we fall back to
	// matching the needle against the stringified scalar. Verify that path.
	if err := scalar.Check(plan, steps); err != nil {
		t.Errorf("scalar-shell fallback match should pass, got: %v", err)
	}
}

func TestStepCount(t *testing.T) {
	plan := "- shell: a\n- shell: b\n- shell: c\n"
	steps, _ := ParsePlan(plan)

	for _, tc := range []struct {
		line     string
		wantPass bool
	}{
		{"step_count <= 5", true},
		{"step_count <= 3", true},
		{"step_count <= 2", false},
		{"step_count >= 3", true},
		{"step_count >= 4", false},
		{"step_count == 3", true},
		{"step_count == 4", false},
	} {
		a, _ := Parse(tc.line)
		err := a.Check(plan, steps)
		if (err == nil) != tc.wantPass {
			t.Errorf("%q on 3-step plan: got err=%v, wantPass=%v", tc.line, err, tc.wantPass)
		}
	}
}

func TestParsePlan_StripsFences(t *testing.T) {
	plan := "```yaml\n- shell: echo hi\n```\n"
	steps, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step after fence strip, got %d", len(steps))
	}
}

func TestParsePlan_RunConfigShape(t *testing.T) {
	plan := `
version: "1"
steps:
  - shell: echo hi
  - file_replace:
      path: /tmp/x
      old_string: a
      new_string: b
`
	steps, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("ParsePlan: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps from RunConfig shape, got %d", len(steps))
	}
}

// TestParsePlan_CompactJSON pins proposal-08's assertion-level
// requirement: a model emitting compact JSON (the new prompt format)
// must parse into the same step-shape map as the legacy YAML path.
// All downstream assertions (contains_step, step_count, etc.) operate
// on the parsed Step slice, so JSON-emitting models are exercised by
// the same goal files with no fixture changes.
func TestParsePlan_CompactJSON(t *testing.T) {
	plan := `[{"name":"hi","cmd":{"argv":["echo","hi"]}},{"file_replace":{"path":"/etc/hosts","old_string":"a","new_string":"b"}}]`
	steps, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("ParsePlan(compact JSON): %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps from JSON array, got %d", len(steps))
	}
	// Spot-check that the assertions grammar works against the parsed
	// JSON-sourced steps without modification.
	a, _ := Parse("contains_step cmd")
	if err := a.Check(plan, steps); err != nil {
		t.Errorf("contains_step cmd should pass on JSON plan, got: %v", err)
	}
	b, _ := Parse("contains_step_with file_replace path=/etc/hosts")
	if err := b.Check(plan, steps); err != nil {
		t.Errorf("contains_step_with should match JSON-sourced step body, got: %v", err)
	}
}

// TestParsePlan_JSONFenced pins the fence-stripping path against
// ```json fences too — small models love to wrap output in fences
// even when the prompt forbids it.
func TestParsePlan_JSONFenced(t *testing.T) {
	plan := "```json\n[{\"name\":\"hi\",\"shell\":\"echo hi\"}]\n```\n"
	steps, err := ParsePlan(plan)
	if err != nil {
		t.Fatalf("ParsePlan(```json fence): %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step after ```json fence strip, got %d", len(steps))
	}
}

func TestSchemaValid_RejectsGarbage(t *testing.T) {
	// schema_valid should fail on something that isn't a valid plan.
	a, _ := Parse("schema_valid")
	err := a.Check("this is not yaml at all : : :", nil)
	if err == nil {
		t.Error("schema_valid should fail on garbage input")
	}
	// And should mention something useful in the message.
	if err != nil && !strings.Contains(err.Error(), "") {
		t.Logf("schema_valid error (informational): %v", err)
	}
}
