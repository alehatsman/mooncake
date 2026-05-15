package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestMT75_StepYAMLOmitsEmptyUnionMembers is a regression test for
// manual-test #75 (2026-05-15): `plan -o plan.yaml` used to serialize
// every nil action union member as `<action>: null`, every empty
// string as `field: ""`, blowing a 1-step plan into ~580 lines. The
// YAML tags on Step now carry `,omitempty` so the encoder skips them.
func TestMT75_StepYAMLOmitsEmptyUnionMembers(t *testing.T) {
	s := Step{
		Name: "hello",
		Shell: &ShellAction{
			Cmd: "echo hi",
		},
	}

	out, err := yaml.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)

	// Sanity: the fields that ARE set must serialize.
	for _, want := range []string{"name: hello", "shell:", "cmd: echo hi"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}

	// MT-75: nil union members and empty scalar conditionals must NOT
	// appear. Sample across the union to catch regressions from any
	// future field added without omitempty.
	forbidden := []string{
		"file.write: null",
		"file.template: null",
		"file.copy: null",
		"file.download: null",
		"text.patch: null",
		"pkg: null",
		"git.clone: null",
		"os.service: null",
		"assert: null",
		"wait.http: null",
		"wait.command: null",
		"observe.process: null",
		"unless_exists: null",
		"unless_command: null",
		"creates: null",
		"unless: null",
		"when: \"\"",
		"as_user: \"\"",
		"changed_when: \"\"",
		"failed_when: \"\"",
		"timeout: \"\"",
		"as: \"\"",
		"cwd: \"\"",
	}
	for _, bad := range forbidden {
		if strings.Contains(got, bad) {
			t.Errorf("unexpected %q in YAML output (MT-75 regression):\n%s", bad, got)
		}
	}
}
