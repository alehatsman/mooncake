package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/schemagen"
)

// TestRenderActionShowText_KnownAction — dx proposal-04: the per-action
// card surfaces metadata + capability matrix + required/optional params
// + a minimum example so users don't have to grep schema.json. The
// assertions are deliberately structural ("contains these landmark
// strings") rather than byte-exact, so a typography tweak doesn't
// require updating every test.
func TestRenderActionShowText_KnownAction(t *testing.T) {
	meta := &actions.ActionMetadata{
		Name:               "file.copy",
		Description:        "Copy a single file from source to destination.",
		Category:           actions.CategoryFile,
		SupportedPlatforms: nil, // "all"
		SupportsDryRun:     true,
		RequiresSudo:       false,
		ImplementsCheck:    true,
		ImplementsDiff:     true,
		ImplementsCost:     true,
		ImplementsReverse:  true,
		Version:            "1.0.0",
		EmitsEvents:        []string{"file.updated"},
	}
	def := &schemagen.Definition{
		Type:        "object",
		Description: "Copy a single file from source to destination, preserving mode.",
		Properties: map[string]*schemagen.Property{
			"src":  {Type: "string", Description: "Path to source file"},
			"dest": {Type: "string", Description: "Path to destination"},
			"mode": {Type: "string", Description: "File mode (e.g. \"0644\")"},
		},
		Required: []string{"src", "dest"},
	}

	var buf bytes.Buffer
	renderActionShowText(meta, def, &buf)
	out := buf.String()

	// Title + horizontal rule.
	if !strings.Contains(out, "file.copy") {
		t.Errorf("output missing title 'file.copy':\n%s", out)
	}
	// Description from the schemagen Definition (takes priority over
	// the registry description per the proposal — Definition is
	// usually more precise).
	if !strings.Contains(out, "preserving mode") {
		t.Errorf("output missing schema description:\n%s", out)
	}
	// Capability matrix from proposal-05.
	for _, want := range []string{
		"Category:",
		"Implements check: yes",
		"Implements diff:  yes",
		"Implements reverse: yes",
		"Version:          1.0.0",
		"Emits events:     file.updated",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// Required / optional split.
	requiredIdx := strings.Index(out, "Required parameters:")
	optionalIdx := strings.Index(out, "Optional parameters:")
	if requiredIdx < 0 || optionalIdx < 0 || optionalIdx < requiredIdx {
		t.Errorf("required/optional sections missing or out of order:\n%s", out)
	}
	// src/dest are in Required, not Optional.
	requiredSection := out[requiredIdx:optionalIdx]
	if !strings.Contains(requiredSection, "src") || !strings.Contains(requiredSection, "dest") {
		t.Errorf("Required section missing src/dest:\n%s", requiredSection)
	}
	// mode is in Optional.
	optionalSection := out[optionalIdx:]
	if !strings.Contains(optionalSection, "mode") {
		t.Errorf("Optional section missing mode:\n%s", optionalSection)
	}
	// Minimum example renders with the required fields only.
	if !strings.Contains(out, "Minimum example:") {
		t.Errorf("output missing minimum example:\n%s", out)
	}
	if !strings.Contains(out, "- file.copy:") {
		t.Errorf("example block missing action header:\n%s", out)
	}
}

// TestRenderActionShowText_NoRequiredOmitsExample — when an action has
// no required fields the minimum-example block is suppressed (a
// synthetic example with no semantics is worse than no example).
func TestRenderActionShowText_NoRequiredOmitsExample(t *testing.T) {
	meta := &actions.ActionMetadata{Name: "facts.snapshot", Category: actions.CategorySystem}
	def := &schemagen.Definition{
		Type: "object",
		Properties: map[string]*schemagen.Property{
			"refresh": {Type: "boolean", Description: "Force a fresh probe"},
		},
		// No required fields.
	}
	var buf bytes.Buffer
	renderActionShowText(meta, def, &buf)
	if strings.Contains(buf.String(), "Minimum example:") {
		t.Errorf("expected no minimum example for an action with no required fields:\n%s", buf.String())
	}
}

// TestFormatPropertyLine — name/type/description column shape.
// Guards against a width-tuning regression that would mangle the
// table alignment.
func TestFormatPropertyLine(t *testing.T) {
	got := formatPropertyLine("src", &schemagen.Property{Type: "string", Description: "Path to source file"})
	if !strings.Contains(got, "src") || !strings.Contains(got, "string") || !strings.Contains(got, "Path to source file") {
		t.Errorf("formatPropertyLine missing pieces: %q", got)
	}
	// Property with no description → "—" stand-in.
	got = formatPropertyLine("mode", &schemagen.Property{Type: "string"})
	if !strings.Contains(got, "—") {
		t.Errorf("formatPropertyLine should use '—' for empty description: %q", got)
	}
	// $ref-only property — type defaults to "ref".
	got = formatPropertyLine("apt", &schemagen.Property{Ref: "#/definitions/PkgRepoApt", Description: "apt driver block"})
	if !strings.Contains(got, "ref") {
		t.Errorf("formatPropertyLine should report 'ref' for $ref-only property: %q", got)
	}
}

// TestExampleValue — pick a sane stand-in literal per JSON Schema type.
// The output is what the minimum-example renderer embeds verbatim, so
// it must be valid YAML for every supported type.
func TestExampleValue(t *testing.T) {
	cases := []struct {
		in   *schemagen.Property
		want string
	}{
		{&schemagen.Property{Type: "string"}, `"…"`},
		{&schemagen.Property{Type: "integer"}, "0"},
		{&schemagen.Property{Type: "number"}, "0.0"},
		{&schemagen.Property{Type: "boolean"}, "false"},
		{&schemagen.Property{Type: "array"}, "[]"},
		{&schemagen.Property{Type: "object"}, "{}"},
		{&schemagen.Property{Type: "unknown"}, "null"},
		{nil, "null"},
	}
	for _, c := range cases {
		if got := exampleValue(c.in); got != c.want {
			t.Errorf("exampleValue(%+v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestNearestActionName — typo-tolerance for `actions show`. The
// suggestion mirrors the closestTag/levenshtein behaviour in
// internal/plan/filter so the UX is consistent across the CLI.
func TestNearestActionName(t *testing.T) {
	candidates := []string{"file.copy", "file.write", "file.template", "shell"}
	cases := []struct {
		needle string
		want   string
	}{
		{"file.cpy", "file.copy"},    // single-char typo
		{"file.wirte", "file.write"}, // transposition
		{"sehll", "shell"},           // transposition
		{"completely-unrelated", ""}, // too far — no suggestion
	}
	for _, c := range cases {
		if got := nearestActionName(c.needle, candidates); got != c.want {
			t.Errorf("nearestActionName(%q) = %q, want %q", c.needle, got, c.want)
		}
	}
}
