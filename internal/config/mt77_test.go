package config

import (
	"os"
	"strings"
	"testing"
)

// TestMT77_UnknownStepFieldNotEmptyVocab is a regression test for
// manual-test #77 (2026-05-15): a step-level typo'd field next to a
// valid action used to produce "Step must have exactly one action ()"
// — an empty vocabulary parenthesis — because the MT-27 collector
// only walks /required causes and the underlying additionalProperties
// failure carries none. Surface the unknown-field diagnostic instead.
func TestMT77_UnknownStepFieldNotEmptyVocab(t *testing.T) {
	yaml := `- file.write:
    path: /tmp/guarded.txt
    content: "v1"
  creates: /tmp/guarded.txt
`
	tmp, err := os.CreateTemp("", "mt77-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(yaml); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	_, diags, _ := ReadConfigWithValidation(tmp.Name())
	for _, d := range diags {
		// The fix's user-visible commitment.
		if strings.Contains(d.Message, "exactly one action ()") {
			t.Errorf("regression: empty action vocabulary in diagnostic: %q", d.Message)
		}
		if strings.Contains(d.Message, "Unknown field 'creates'") {
			return // happy path
		}
	}
	t.Errorf("expected 'Unknown field creates' diagnostic, got %d diags: %+v", len(diags), diags)
}

// TestMT77_NoActionStillShowsVocab guards against an over-correction:
// the genuinely-missing-action case should still emit the full action
// vocabulary so users learn what's available.
func TestMT77_NoActionStillShowsVocab(t *testing.T) {
	yaml := `- name: no action
  when: "true"
`
	tmp, err := os.CreateTemp("", "mt77-noaction-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(yaml)
	tmp.Close()

	_, diags, _ := ReadConfigWithValidation(tmp.Name())
	gotVocab := false
	for _, d := range diags {
		if strings.Contains(d.Message, "exactly one action (") && !strings.Contains(d.Message, "exactly one action ()") {
			gotVocab = true
			// Spot-check a representative entry — keeps the test
			// resilient to future action additions / renames.
			if !strings.Contains(d.Message, "shell") || !strings.Contains(d.Message, "file.write") {
				t.Errorf("vocabulary missing canonical entries: %q", d.Message)
			}
		}
	}
	if !gotVocab {
		t.Errorf("expected populated action vocabulary, got %d diags: %+v", len(diags), diags)
	}
}
