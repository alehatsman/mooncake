package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// componentFixture writes a component YAML into ./components and returns its
// name, cleaning the dir up afterwards.
func componentFixture(t *testing.T, name, content string) string {
	t.Helper()
	componentsDir := filepath.Join(".", "components")
	if err := os.MkdirAll(componentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(componentsDir) })

	if err := os.WriteFile(filepath.Join(componentsDir, name+".yml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestLoadComponent_PropsKey(t *testing.T) {
	name := componentFixture(t, "props-form", `name: props-form
props:
  tls:  { type: bool, default: false }
  port: { type: string, default: "5432" }
steps:
  - name: noop
    log: "starting"
`)

	component, err := LoadComponent(name)
	if err != nil {
		t.Fatalf("LoadComponent: %v", err)
	}
	if len(component.Props) != 2 {
		t.Errorf("expected 2 props, got %d", len(component.Props))
	}
	if component.Props["tls"].Type != "bool" {
		t.Errorf("tls.Type = %q", component.Props["tls"].Type)
	}
}

// `parameters:` is retired. It must fail at parse time naming the replacement,
// not be ignored into a confusing "unknown prop" error at the point of use.
func TestLoadComponent_ParametersKeyIsRejected(t *testing.T) {
	name := componentFixture(t, "legacy", `name: legacy
parameters:
  msg: { type: string }
steps:
  - name: noop
    log: "x"
`)

	_, err := LoadComponent(name)
	if err == nil {
		t.Fatal("LoadComponent accepted the retired `parameters:` key")
	}
	if !strings.Contains(err.Error(), "props:") {
		t.Errorf("error should name the replacement key, got: %v", err)
	}
}

// The props namespace is the only one. The legacy `parameters` alias is gone,
// so a template referencing it must not silently resolve.
func TestExpandComponent_InjectsOnlyPropsNamespace(t *testing.T) {
	name := componentFixture(t, "ns", `name: ns
props:
  msg: { type: string, required: true }
steps:
  - name: noop
    log: "{{ props.msg }}"
`)

	_, namespace, _, err := ExpandComponent(name, map[string]interface{}{"msg": "hi"})
	if err != nil {
		t.Fatalf("ExpandComponent: %v", err)
	}
	props, ok := namespace["props"].(map[string]interface{})
	if !ok {
		t.Fatal("expected props namespace to be a map")
	}
	if props["msg"] != "hi" {
		t.Errorf("props.msg = %v, want \"hi\"", props["msg"])
	}
	if _, ok := namespace["parameters"]; ok {
		t.Error("the retired `parameters` namespace is still injected")
	}
}
