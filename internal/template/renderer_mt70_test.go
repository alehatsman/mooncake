package template

import (
	"strings"
	"testing"
)

// TestMT70_BareMapStringifyToJSON is a regression test for manual-test
// #70 (2026-05-15): pongo2 stringifies a map or slice variable as
// `<TYPE Value>` (its internal reflect repr). For mooncake's `log:` /
// `print.message` surface that leaks into event payloads with no
// recovery path. Auto-route bare and dotted-path non-scalars through
// the registered tojson filter.
func TestMT70_BareMapStringifyToJSON(t *testing.T) {
	r, err := NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	vars := map[string]interface{}{
		"cfg": map[string]interface{}{
			"value": map[string]interface{}{
				"service": map[string]interface{}{
					"name": "web",
					"port": 8080,
				},
			},
		},
		"list": []interface{}{"a", "b", "c"},
	}

	cases := []struct {
		name string
		in   string
		want string
	}{
		// Bare map → JSON
		{"bare map", "{{ cfg }}", `{"value":{"service":{"name":"web","port":8080}}}`},
		// Dotted access to a map → JSON
		{"dotted to map", "{{ cfg.value }}", `{"service":{"name":"web","port":8080}}`},
		{"deeper dotted to map", "{{ cfg.value.service }}", `{"name":"web","port":8080}`},
		// Bare slice → JSON
		{"bare slice", "{{ list }}", `["a","b","c"]`},
		// Drilling to scalar stays unchanged — string renders as-is.
		{"dotted to scalar", "{{ cfg.value.service.name }}", "web"},
		{"dotted to int", "{{ cfg.value.service.port }}", "8080"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			got, err := r.Render(c.in, vars)
			if err != nil {
				t.Fatalf("Render(%q): %v", c.in, err)
			}
			if got != c.want {
				t.Errorf("Render(%q) = %q, want %q", c.in, got, c.want)
			}
			// Hard fail if the reflect.Value repr ever shows up again.
			if strings.Contains(got, "Value>") {
				t.Errorf("rendered output contains reflect.Value repr: %q", got)
			}
		})
	}
}

func TestMT70_ExplicitTojsonStillWorks(t *testing.T) {
	r, _ := NewPongo2Renderer()
	got, err := r.Render(`{{ x | tojson }}`, map[string]interface{}{
		"x": map[string]interface{}{"a": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != `{"a":1}` {
		t.Errorf("explicit tojson = %q, want %q", got, `{"a":1}`)
	}
}

func TestMT70_FilterChainNotAutoRewritten(t *testing.T) {
	// Users with explicit filters shouldn't get a second tojson injected.
	r, _ := NewPongo2Renderer()
	got, err := r.Render(`{{ x | upper }}`, map[string]interface{}{"x": "abc"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "ABC" {
		t.Errorf("upper filter = %q, want ABC", got)
	}
}

func TestMT70_ScalarBareRefUnchanged(t *testing.T) {
	// Scalar variables must keep rendering as scalars, not "1"-form
	// JSON, so number/string formatting isn't disturbed.
	r, _ := NewPongo2Renderer()
	got, err := r.Render(`{{ n }}`, map[string]interface{}{"n": 42})
	if err != nil {
		t.Fatal(err)
	}
	if got != "42" {
		t.Errorf("int bare-ref = %q, want 42", got)
	}
}
