package agent

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSanitizePlan(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "yaml fence",
			input:    "```yaml\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "yml fence",
			input:    "```yml\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "generic fence",
			input:    "```\n- shell:\n    cmd: echo hello\n```",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			name:     "no fence",
			input:    "- shell:\n    cmd: echo hello",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
		{
			// A model that ignores the new prompt and still emits YAML
			// inside a ```json fence (or vice versa) must keep working.
			// Re-encoding through DecodeAuto + yaml.Marshal canonicalizes
			// the output; simple scalar-shell stays byte-identical.
			name:     "json fence around yaml body",
			input:    "```json\n- shell: echo hi\n```",
			expected: "- shell: echo hi",
			wantErr:  false,
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "whitespace only",
			input:    "   \n\n  ",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "fence with whitespace",
			input:    "  ```yaml\n- shell:\n    cmd: echo hello\n```  ",
			expected: "- shell:\n    cmd: echo hello",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizePlan(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if strings.TrimSpace(string(result)) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

// TestSanitizePlan_JSONStepsWrapperUnwraps mirrors the legacy YAML
// "steps wrapper" behavior for the new JSON path. A model that emits
// a RunConfig-shape JSON object should have its steps unwrapped, just
// like the YAML case in sanitize_debug_test.go.
func TestSanitizePlan_JSONStepsWrapperUnwraps(t *testing.T) {
	input := `{"name":"demo","steps":[{"name":"hi","cmd":{"argv":["echo","hi"]}}]}`
	result, err := SanitizePlan(input)
	if err != nil {
		t.Fatalf("SanitizePlan: %v", err)
	}
	s := string(result)
	if !strings.Contains(s, "argv") || !strings.Contains(s, "echo") {
		t.Errorf("expected unwrapped steps to contain argv+echo, got: %s", s)
	}
	// The unwrapped output must not still contain the wrapper "steps:"
	// key — that would mean we kept the RunConfig shape.
	if strings.Contains(s, "steps:") {
		t.Errorf("expected steps wrapper to be unwrapped, still present in: %s", s)
	}
}

// TestSanitizePlan_FormatEquivalence is the proposal-08 contract test:
// JSON input and the equivalent YAML input must produce semantically
// identical sanitized output. We don't pin formatted bytes (yaml.Marshal
// formatting is incidental) — we decode both outputs and compare via
// reflect.DeepEqual so this test survives yaml.v3 cosmetic changes.
//
// Pinning the contract this way is what lets us claim "JSON-emitting
// models are exercised by the same downstream code as YAML models with
// no flag day".
func TestSanitizePlan_FormatEquivalence(t *testing.T) {
	cases := []struct {
		name string
		json string
		yaml string
	}{
		{
			name: "single cmd step",
			json: `[{"name":"hi","cmd":{"argv":["echo","hi"]}}]`,
			yaml: "- name: hi\n  cmd:\n    argv:\n      - echo\n      - hi\n",
		},
		{
			name: "two-step plan with shell and file_replace",
			json: `[{"shell":"echo a"},{"file_replace":{"path":"/etc/x","old_string":"a","new_string":"b"}}]`,
			yaml: "- shell: echo a\n- file_replace:\n    path: /etc/x\n    old_string: a\n    new_string: b\n",
		},
		{
			name: "empty plan literal",
			json: `[]`,
			yaml: `[]`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonOut, err := SanitizePlan(tc.json)
			if err != nil {
				t.Fatalf("SanitizePlan(json): %v", err)
			}
			yamlOut, err := SanitizePlan(tc.yaml)
			if err != nil {
				t.Fatalf("SanitizePlan(yaml): %v", err)
			}
			var fromJSON, fromYAML any
			if err := yaml.Unmarshal(jsonOut, &fromJSON); err != nil {
				t.Fatalf("decode jsonOut: %v\nraw:\n%s", err, jsonOut)
			}
			if err := yaml.Unmarshal(yamlOut, &fromYAML); err != nil {
				t.Fatalf("decode yamlOut: %v\nraw:\n%s", err, yamlOut)
			}
			if !reflect.DeepEqual(fromJSON, fromYAML) {
				t.Errorf("JSON and YAML sanitized outputs differ semantically:\nfromJSON: %#v\nfromYAML: %#v",
					fromJSON, fromYAML)
			}
		})
	}
}

// TestSanitizePlan_BareFenceWithJSON covers the small-model pattern
// of wrapping JSON in a bare ``` fence (no language tag). Strips the
// fence and re-encodes to canonical YAML.
func TestSanitizePlan_BareFenceWithJSON(t *testing.T) {
	input := "```\n[{\"name\":\"x\",\"cmd\":{\"argv\":[\"true\"]}}]\n```"
	result, err := SanitizePlan(input)
	if err != nil {
		t.Fatalf("SanitizePlan: %v", err)
	}
	var steps []map[string]any
	if err := yaml.Unmarshal(result, &steps); err != nil {
		t.Fatalf("decoded sanitized output: %v\nraw:\n%s", err, result)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d (raw: %s)", len(steps), result)
	}
	if steps[0]["name"] != "x" {
		t.Errorf("expected step name=x, got %v", steps[0]["name"])
	}
}
