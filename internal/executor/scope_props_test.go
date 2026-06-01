package executor

import "testing"

// TestScopeProps_ExposedAsPropsAndParameters is a regression test for #49:
// component props/parameters were dropped from scope by the time execute-time
// fields (when/cwd) were rendered, so a `use:`d component's `cwd: {{ props.x }}`
// resolved empty. The executor now restores Step.ComponentProps into
// Scope.Props per step; ToMap must surface it under both `props` and the
// legacy `parameters` alias.
func TestScopeProps_ExposedAsPropsAndParameters(t *testing.T) {
	scope := NewVariableScope()
	scope.Props = map[string]interface{}{"dir": "/srv/web", "enabled": true}

	flat := scope.ToMap()
	for _, key := range []string{"props", "parameters"} {
		m, ok := flat[key].(map[string]interface{})
		if !ok {
			t.Fatalf("flat[%q] is not map[string]interface{}: %T", key, flat[key])
		}
		if m["dir"] != "/srv/web" {
			t.Errorf("flat[%q][dir] = %v, want /srv/web", key, m["dir"])
		}
		if m["enabled"] != true {
			t.Errorf("flat[%q][enabled] = %v, want true", key, m["enabled"])
		}
	}
}

// TestScopeProps_OmittedWhenNil guards the ToMap shape: outside a component
// (Props nil) neither `props` nor `parameters` should appear, so a stray
// `{{ props.x }}` in an ordinary step renders empty rather than masking a typo.
func TestScopeProps_OmittedWhenNil(t *testing.T) {
	scope := NewVariableScope()
	flat := scope.ToMap()
	if _, present := flat["props"]; present {
		t.Errorf("expected no `props` key when Scope.Props is nil, got %v", flat["props"])
	}
	if _, present := flat["parameters"]; present {
		t.Errorf("expected no `parameters` key when Scope.Props is nil, got %v", flat["parameters"])
	}
}
