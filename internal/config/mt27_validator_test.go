package config

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// TestMT27_ValidatorAcceptsRegisteredActions is a regression test for
// manual-test #27 (2026-05-15): the embedded schema's oneOf action list
// was out of sync with the registry. read.json/read.yaml in particular
// had empty schema definitions (no properties + additionalProperties:false)
// which rejected every valid playbook using them.
func TestMT27_ValidatorAcceptsRegisteredActions(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{
			name: "read.json with path",
			yaml: `- name: read tag
  read.json:
    path: /tmp/test.json
`,
		},
		{
			name: "read.yaml with path + query",
			yaml: `- name: read manifest
  read.yaml:
    path: /tmp/manifest.yaml
    query: services.web.image
`,
		},
		{
			name: "text.patch.json with set",
			yaml: `- name: patch json
  text.patch.json:
    path: /tmp/in.json
    set:
      foo: bar
`,
		},
		{
			name: "git.clone with repo + dest",
			yaml: `- name: clone
  git.clone:
    repo: https://github.com/example/repo
    dest: /tmp/repo
`,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			tmp, err := os.CreateTemp("", "mt27-*.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmp.Name())
			if _, err := tmp.WriteString(c.yaml); err != nil {
				t.Fatal(err)
			}
			tmp.Close()

			_, diags, _ := ReadConfigWithValidation(tmp.Name())
			for _, d := range diags {
				if strings.Contains(d.Message, "Step must have exactly one action") ||
					strings.Contains(d.Message, "Step has no action") {
					t.Errorf("got vocabulary-rejection diagnostic for %s: %s", c.name, d.Message)
				}
			}
		})
	}
}

// TestMT27_ExtractRequiredNames covers the parser that lifts action names
// out of jsonschema's "missing properties: 'x', 'y'" / "required property
// 'z'" messages.
func TestMT27_ExtractRequiredNames(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"missing properties: 'read.json'", []string{"read.json"}},
		{"missing properties: 'a', 'b', 'c'", []string{"a", "b", "c"}},
		{"required property 'x'", []string{"x"}},
		{"some unrelated text", nil},
	}
	for _, c := range cases {
		got := extractRequiredNames(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("extractRequiredNames(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
