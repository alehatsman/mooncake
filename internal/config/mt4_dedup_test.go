package config

import (
	"os"
	"strings"
	"testing"
)

// TestMT4_UnknownFieldSuppressesGenericOneOf addresses the
// verification-2026-05-15.md gap on MT-4: when a step names a valid
// action but uses an unknown sub-property, the validator was emitting
// TWO diagnostics for the same line — the precise "unknown field
// 'content'" message AND the misleading "Step must have exactly one
// action (artifact.capture, …, file.template, …)" message. The
// second was confusing because the step DID have file.template.
// The dedup pass now suppresses the generic oneOf message at any
// line that already has an unknown-field diagnostic.
func TestMT4_UnknownFieldSuppressesGenericOneOf(t *testing.T) {
	body := `- name: bad
  file.template:
    dest: /tmp/x
    content: "should be src"
`
	tmp, err := os.CreateTemp("", "mt4-dedup-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(body); err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	_, diags, _ := ReadConfigWithValidation(tmp.Name())

	gotUnknown := false
	gotGenericOneOf := false
	for _, d := range diags {
		if strings.Contains(d.Message, "unknown field") || strings.Contains(d.Message, "Unknown field") {
			gotUnknown = true
		}
		if strings.Contains(d.Message, "Step must have exactly one action") {
			gotGenericOneOf = true
		}
	}
	if !gotUnknown {
		t.Errorf("expected `unknown field` diagnostic, got: %+v", diags)
	}
	if gotGenericOneOf {
		t.Errorf("MT-4 regression: generic oneOf diagnostic still appears alongside unknown-field one: %+v", diags)
	}
}

// TestMT4_NoActionStillShowsVocabulary guards against an over-suppression:
// a genuinely action-less step must still emit the full action
// vocabulary so the user learns what's available.
func TestMT4_NoActionStillShowsVocabulary(t *testing.T) {
	body := `- name: no action step
  when: "true"
`
	tmp, err := os.CreateTemp("", "mt4-vocab-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString(body)
	tmp.Close()

	_, diags, _ := ReadConfigWithValidation(tmp.Name())

	gotVocab := false
	for _, d := range diags {
		if strings.Contains(d.Message, "Step must have exactly one action (") &&
			strings.Contains(d.Message, "shell") &&
			strings.Contains(d.Message, "file.write") {
			gotVocab = true
		}
	}
	if !gotVocab {
		t.Errorf("expected populated action vocabulary for genuinely action-less step, got: %+v", diags)
	}
}
