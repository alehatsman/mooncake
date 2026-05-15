package executor

import (
	"os"
	"testing"
)

// TestMT82_EnvNamespaceInScope is a regression test for manual-test
// #82 (2026-05-15): templates had no way to read parent-process
// environment variables — `{{ env.HOME }}` and `{{ environ.X }}`
// both rendered empty, forcing users to wrap a shell step to read
// what bash already inherits naturally. AddGlobalVariables now
// snapshots os.Environ() into Scope.Env, exposed in ToMap() under
// the `env` key.
func TestMT82_EnvNamespaceInScope(t *testing.T) {
	t.Setenv("MT82_TEST_KEY", "mt82-value")

	scope := NewVariableScope()
	AddGlobalVariables(scope)

	if scope.Env == nil {
		t.Fatal("AddGlobalVariables did not populate Scope.Env")
	}
	if got := scope.Env["MT82_TEST_KEY"]; got != "mt82-value" {
		t.Errorf("Scope.Env[MT82_TEST_KEY] = %q, want %q", got, "mt82-value")
	}
	// HOME is a near-universal canary; it'll be set on any test runner
	// we'd plausibly run on.
	if scope.Env["HOME"] == "" && os.Getenv("HOME") != "" {
		t.Errorf("Scope.Env[HOME] empty despite os.Getenv(HOME)=%q", os.Getenv("HOME"))
	}

	flat := scope.ToMap()
	envMap, ok := flat["env"].(map[string]interface{})
	if !ok {
		t.Fatalf("flat[env] is not map[string]interface{}: %T", flat["env"])
	}
	if got, _ := envMap["MT82_TEST_KEY"].(string); got != "mt82-value" {
		t.Errorf("flat[env][MT82_TEST_KEY] = %v, want mt82-value", envMap["MT82_TEST_KEY"])
	}
}

// TestMT82_EmptyEnvOmitsKey guards the ToMap shape — if Env is empty
// or nil, no `env` key in the rendered map. Avoids forcing callers
// to think about a key that has no useful content.
func TestMT82_EmptyEnvOmitsKey(t *testing.T) {
	scope := NewVariableScope()
	flat := scope.ToMap()
	if _, present := flat["env"]; present {
		t.Errorf("expected no `env` key when Scope.Env is empty, got %v", flat["env"])
	}
}

// TestMT82_EnvSharedAcrossClones — Env is snapshot once at run start
// and read-only; Clone should reuse the parent's map by reference.
// Mutating the parent's Env (e.g. in tests) is visible to clones,
// and clones don't allocate fresh copies of the (potentially large)
// environment.
func TestMT82_EnvSharedAcrossClones(t *testing.T) {
	parent := NewVariableScope()
	parent.Env = map[string]string{"K": "v"}
	clone := parent.Clone()
	if clone.Env == nil {
		t.Fatal("clone lost Env reference")
	}
	if got := clone.Env["K"]; got != "v" {
		t.Errorf("clone.Env[K] = %q, want v", got)
	}
}
