package security

import (
	"strings"
	"testing"
)

func TestEnvProvider_HappyPath(t *testing.T) {
	t.Setenv("MOONCAKE_SECRET_TEST_KEY", "expected-value")
	val, err := EnvProvider{}.Resolve("MOONCAKE_SECRET_TEST_KEY")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "expected-value" {
		t.Errorf("got %q, want %q", val, "expected-value")
	}
}

func TestEnvProvider_UnsetErrors(t *testing.T) {
	// Make sure it's not set in this process.
	t.Setenv("MOONCAKE_SECRET_TEST_UNSET", "")
	_, err := EnvProvider{}.Resolve("MOONCAKE_SECRET_TEST_UNSET")
	if err == nil {
		t.Fatal("expected error for unset variable")
	}
	if !strings.Contains(err.Error(), "not set") {
		t.Errorf("err = %v, want substring 'not set'", err)
	}
}

func TestEnvProvider_EmptyVariableName(t *testing.T) {
	_, err := EnvProvider{}.Resolve("")
	if err == nil {
		t.Fatal("expected error for empty variable name")
	}
}

func TestRegistry_ResolveDispatches(t *testing.T) {
	t.Setenv("MOONCAKE_REG_TEST", "ok")
	val, err := DefaultRegistry.Resolve("env:MOONCAKE_REG_TEST")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if val != "ok" {
		t.Errorf("got %q, want 'ok'", val)
	}
}

func TestRegistry_UnknownProviderErrors(t *testing.T) {
	_, err := DefaultRegistry.Resolve("nope:something")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("err = %v, want substring 'unknown provider'", err)
	}
}

func TestRegistry_RefWithoutColonErrors(t *testing.T) {
	_, err := DefaultRegistry.Resolve("no-colon-here")
	if err == nil {
		t.Fatal("expected error for malformed ref")
	}
	if !strings.Contains(err.Error(), "<provider>:<path>") {
		t.Errorf("err = %v, want substring '<provider>:<path>'", err)
	}
}

// TestRegistry_ErrorRedactsPath: when a provider's Resolve errors, the
// wrapped message must not include the secret path beyond the provider
// prefix. Prevents accidentally leaking partial-secret data in logs.
func TestRegistry_ErrorRedactsPath(t *testing.T) {
	_, err := DefaultRegistry.Resolve("env:SOMETHING_UNSET_QXZ")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if strings.Contains(msg, "SOMETHING_UNSET_QXZ") {
		t.Errorf("error message should not contain the secret path; got: %v", err)
	}
	if !strings.Contains(msg, "env:") {
		t.Errorf("error message should name the provider; got: %v", err)
	}
}

func TestIsMarker_AndMarkerRef(t *testing.T) {
	marker := SentinelPrefix + "env:APP_TOKEN"
	if !IsMarker(marker) {
		t.Errorf("IsMarker(%q) should be true", marker)
	}
	if got := MarkerRef(marker); got != "env:APP_TOKEN" {
		t.Errorf("MarkerRef = %q, want 'env:APP_TOKEN'", got)
	}

	if IsMarker("plain string") {
		t.Error("IsMarker should be false for plain string")
	}
	if MarkerRef("plain string") != "" {
		t.Error("MarkerRef should be empty for non-marker")
	}
}

func TestResolveMarker_PassThroughNonMarker(t *testing.T) {
	got, resolved, err := ResolveMarker("plain")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if resolved {
		t.Error("resolved should be false for non-marker")
	}
	if got != "plain" {
		t.Errorf("got %q, want 'plain'", got)
	}
}

func TestResolveMarker_Resolves(t *testing.T) {
	t.Setenv("MOONCAKE_RESOLVE_TEST", "secret-val")
	got, resolved, err := ResolveMarker(SentinelPrefix + "env:MOONCAKE_RESOLVE_TEST")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !resolved {
		t.Error("resolved should be true")
	}
	if got != "secret-val" {
		t.Errorf("got %q, want 'secret-val'", got)
	}
}
