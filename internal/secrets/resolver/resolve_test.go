package resolver

import (
	"reflect"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
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

// TestResolve_StepVarsContainsMarker is the F019 reproducer. Step.Vars
// is *map[string]interface{} (a pointer to an interface-valued map);
// markers inside it must resolve, otherwise `!secret` in `vars:` blocks
// silently passes the sentinel string through to downstream templates.
func TestResolve_StepVarsContainsMarker(t *testing.T) {
	t.Setenv("MC_TEST_VARS_SECRET", "leaked")
	m := map[string]interface{}{
		"api_key": security.SentinelPrefix + "env:MC_TEST_VARS_SECRET",
		"public":  "plain",
	}
	step := &config.Step{Vars: &m}

	redactor := security.NewRedactor()
	if err := Resolve(step, redactor); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	got, _ := (*step.Vars)["api_key"].(string)
	if got != "leaked" {
		t.Fatalf("step.Vars[api_key] = %q, want %q", got, "leaked")
	}
	if (*step.Vars)["public"] != "plain" {
		t.Errorf("public mutated: %v", (*step.Vars)["public"])
	}
	if redactor.Redact("seen: leaked here") != "seen: [REDACTED] here" {
		t.Error("resolved value not added to redactor denylist")
	}
}

// TestWalkAndResolveSecrets_PointerMapField covers the *map[K]V code
// path the F019 fix adds — a pointer to a map with interface{} values,
// like Step.Vars.
func TestWalkAndResolveSecrets_PointerMapField(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_PMAP", "pmap-secret")
	type victim struct {
		Vars *map[string]interface{}
	}
	m := map[string]interface{}{
		"key": security.SentinelPrefix + "env:MC_TEST_RESOLVE_PMAP",
	}
	v := &victim{Vars: &m}

	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	got, _ := (*v.Vars)["key"].(string)
	if got != "pmap-secret" {
		t.Errorf("(*v.Vars)[key] = %q, want 'pmap-secret'", got)
	}
}

// TestWalkAndResolveSecrets_InterfaceMapField covers top-level
// map[string]interface{} fields (without the pointer). Same kind set
// the F019 fix extends — markers in interface-valued maps must resolve.
func TestWalkAndResolveSecrets_InterfaceMapField(t *testing.T) {
	t.Setenv("MC_TEST_RESOLVE_IMAP", "imap-secret")
	type victim struct {
		Data map[string]interface{}
	}
	v := &victim{Data: map[string]interface{}{
		"public":  "plain",
		"private": security.SentinelPrefix + "env:MC_TEST_RESOLVE_IMAP",
		"number":  42, // non-string interface value: untouched
	}}

	redactor := security.NewRedactor()
	if err := walkAndResolveSecrets(reflect.ValueOf(v).Elem(), redactor); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if v.Data["public"] != "plain" {
		t.Errorf("public mutated: %v", v.Data["public"])
	}
	if got, _ := v.Data["private"].(string); got != "imap-secret" {
		t.Errorf("private = %v, want 'imap-secret'", v.Data["private"])
	}
	if v.Data["number"] != 42 {
		t.Errorf("non-string interface value mutated: %v", v.Data["number"])
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
