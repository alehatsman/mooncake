package resolver

import (
	"reflect"
	"testing"

	"github.com/alehatsman/mooncake/internal/security"
)

// TestWalkAndResolveSecrets_StringField swaps a marker in a top-level
// string field and asserts the resolved value lands in place + gets
// added to the redactor's denylist.
func TestWalkAndResolveSecrets_StringField(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_FIELD", "actual-secret")
	type victim struct {
		Field string
	}
	v := &victim{Field: security.SentinelPrefix + "env:MC_TEST_RESOLVE_FIELD"}

	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if v.Field != "actual-secret" {
		t.Errorf("field = %q, want 'actual-secret'", v.Field)
	}
	// Redactor must now redact the value when it appears in a log line.
	if got := redactor.Redact("leak: actual-secret here"); got != "leak: [REDACTED] here" {
		t.Errorf("redactor did not pick up the value; redacted = %q", got)
	}
}

// TestWalkAndResolveSecrets_PlainStringUntouched asserts non-marker
// strings are NOT modified — the walker is a no-op for them.
func TestWalkAndResolveSecrets_PlainStringUntouched(t *testing.T) {
	type victim struct {
		Field string
	}
	v := &victim{Field: "just a regular string"}
	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if v.Field != "just a regular string" {
		t.Errorf("plain string mutated: %q", v.Field)
	}
}

// TestWalkAndResolveSecrets_NestedStruct verifies the recursion into
// pointer-to-struct fields. Most action structs nest one level deep
// (Step → *FileWrite → fields), so this is the realistic shape.
func TestWalkAndResolveSecrets_NestedStruct(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_NESTED", "nested-secret")
	type inner struct {
		Path    string
		Content string
	}
	type outer struct {
		Action *inner
		Other  string
	}
	v := &outer{
		Action: &inner{
			Path:    "/tmp/x",
			Content: security.SentinelPrefix + "env:MC_TEST_RESOLVE_NESTED",
		},
		Other: "unchanged",
	}
	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if v.Action.Content != "nested-secret" {
		t.Errorf("nested content = %q, want 'nested-secret'", v.Action.Content)
	}
	if v.Action.Path != "/tmp/x" {
		t.Errorf("sibling field mutated: %q", v.Action.Path)
	}
	if v.Other != "unchanged" {
		t.Errorf("outer.Other mutated: %q", v.Other)
	}
}

// TestWalkAndResolveSecrets_SliceField covers []string in an action
// struct (e.g. PkgInstall.Names with one entry as a secret).
func TestWalkAndResolveSecrets_SliceField(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_SLICE", "slice-secret")
	type victim struct {
		Names []string
	}
	v := &victim{Names: []string{
		"plain",
		security.SentinelPrefix + "env:MC_TEST_RESOLVE_SLICE",
		"another-plain",
	}}
	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	want := []string{"plain", "slice-secret", "another-plain"}
	for i, w := range want {
		if v.Names[i] != w {
			t.Errorf("[%d] = %q, want %q", i, v.Names[i], w)
		}
	}
}

// TestWalkAndResolveSecrets_MapField covers map[string]string (e.g.
// env vars on a shell step).
func TestWalkAndResolveSecrets_MapField(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_MAP", "map-secret")
	type victim struct {
		Env map[string]string
	}
	v := &victim{Env: map[string]string{
		"PUBLIC":  "public-value",
		"PRIVATE": security.SentinelPrefix + "env:MC_TEST_RESOLVE_MAP",
	}}
	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if v.Env["PUBLIC"] != "public-value" {
		t.Errorf("public mutated: %q", v.Env["PUBLIC"])
	}
	if v.Env["PRIVATE"] != "map-secret" {
		t.Errorf("private = %q, want 'map-secret'", v.Env["PRIVATE"])
	}
}

// TestWalkAndResolveSecrets_UnsetVariableErrors: when the marker points
// at an unset env var, the walker surfaces the error with field
// context. The error message must NOT contain the secret path (the env
// var name) — verified separately in security_test.
func TestWalkAndResolveSecrets_UnsetVariableErrors(t *testing.T) {
	type victim struct {
		Field string
	}
	v := &victim{Field: security.SentinelPrefix + "env:DEFINITELY_NOT_SET_XYZQ"}
	redactor := security.NewRedactor()
	err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor)
	if err == nil {
		t.Fatal("expected error for unset env var")
	}
	// Field name should appear (debuggability).
	if !contains(err.Error(), "Field") {
		t.Errorf("error should name the field; got: %v", err)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
