package presets

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestLoadPreset_PropsKey verifies the spec-67 `props:` form parses identically
// to the legacy `parameters:` form and does NOT trigger the deprecation warning.
func TestLoadPreset_PropsKey(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "props-form.yml")
	content := `name: props-form
props:
  tls:  { type: bool, default: false }
  port: { type: string, default: "5432" }
steps:
  - name: noop
    log: "starting"
`
	if err := os.WriteFile(presetPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	withCaptured(t, func(buf *bytes.Buffer) {
		preset, err := LoadPreset("props-form")
		if err != nil {
			t.Fatalf("LoadPreset: %v", err)
		}
		if len(preset.Parameters) != 2 {
			t.Errorf("expected 2 params, got %d", len(preset.Parameters))
		}
		if preset.UsedParametersKey {
			t.Error("expected UsedParametersKey=false for props: form")
		}
		if buf.Len() != 0 {
			t.Errorf("unexpected deprecation output: %q", buf.String())
		}
		if preset.Parameters["tls"].Type != "bool" {
			t.Errorf("tls.Type = %q", preset.Parameters["tls"].Type)
		}
	})
}

// TestLoadPreset_ParametersKeyDeprecated verifies the legacy `parameters:` form
// still works AND emits a deprecation warning on stderr.
func TestLoadPreset_ParametersKeyDeprecated(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "params-form.yml")
	content := `name: params-form
parameters:
  foo:
    type: string
    default: bar
steps:
  - name: noop
    log: "{{ parameters.foo }}"
`
	if err := os.WriteFile(presetPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	withCaptured(t, func(buf *bytes.Buffer) {
		preset, err := LoadPreset("params-form")
		if err != nil {
			t.Fatalf("LoadPreset: %v", err)
		}
		if !preset.UsedParametersKey {
			t.Error("expected UsedParametersKey=true for parameters: form")
		}
		if !strings.Contains(buf.String(), "deprecated") {
			t.Errorf("expected deprecation warning, got: %q", buf.String())
		}
	})
}

// TestLoadPreset_PropsAndParametersConflict verifies that declaring both keys
// is rejected with a clear error.
func TestLoadPreset_PropsAndParametersConflict(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "conflict.yml")
	content := `name: conflict
props:
  a: { type: string }
parameters:
  b: { type: string }
steps:
  - name: noop
    log: "x"
`
	if err := os.WriteFile(presetPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPreset("conflict")
	if err == nil {
		t.Fatal("LoadPreset should fail when both props: and parameters: are present")
	}
	if !strings.Contains(err.Error(), "both") {
		t.Errorf("error = %q, want substring \"both\"", err.Error())
	}
}

// TestExpandPreset_InjectsPropsNamespace verifies the expander surfaces values
// under both `parameters` (legacy) and `props` (spec-67) namespaces.
func TestExpandPreset_InjectsPropsNamespace(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	if err := os.MkdirAll(presetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "ns.yml")
	content := `name: ns
props:
  msg: { type: string, required: true }
steps:
  - name: noop
    log: "{{ props.msg }}"
`
	if err := os.WriteFile(presetPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, namespace, _, err := ExpandPreset(&config.PresetInvocation{
		Name: "ns",
		With: map[string]interface{}{"msg": "hi"},
	})
	if err != nil {
		t.Fatalf("ExpandPreset: %v", err)
	}
	props, ok := namespace["props"].(map[string]interface{})
	if !ok {
		t.Fatal("expected props namespace to be a map")
	}
	if props["msg"] != "hi" {
		t.Errorf("props.msg = %v, want \"hi\"", props["msg"])
	}
	params, ok := namespace["parameters"].(map[string]interface{})
	if !ok {
		t.Fatal("expected parameters namespace to be a map")
	}
	if params["msg"] != "hi" {
		t.Errorf("parameters.msg = %v, want \"hi\"", params["msg"])
	}
}

// withCaptured redirects the deprecation writer for the body of f and restores
// it on return. The capture is also reset between calls within the same test
// process so prior runs don't suppress new warnings.
func withCaptured(t *testing.T, f func(buf *bytes.Buffer)) {
	t.Helper()
	buf := &bytes.Buffer{}
	deprecationMu.Lock()
	prev := deprecationWriter
	deprecationWriter = io.Writer(buf)
	for k := range deprecationWarned {
		delete(deprecationWarned, k)
	}
	deprecationMu.Unlock()
	defer func() {
		deprecationMu.Lock()
		deprecationWriter = prev
		deprecationMu.Unlock()
	}()
	f(buf)
}
