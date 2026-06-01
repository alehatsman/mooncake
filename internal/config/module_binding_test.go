package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestModuleBinding_YAML_StringForm verifies the back-compat bare-string
// form decodes to a Source-only binding and marshals back to a bare string.
func TestModuleBinding_YAML_StringForm(t *testing.T) {
	var m ModuleBinding
	if err := yaml.Unmarshal([]byte(`"host/owner/repo@v1.0.0"`), &m); err != nil {
		t.Fatalf("unmarshal string form: %v", err)
	}
	if m.Source != "host/owner/repo@v1.0.0" {
		t.Errorf("Source = %q", m.Source)
	}
	if m.Props != nil {
		t.Errorf("Props = %v, want nil", m.Props)
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(out); got != "host/owner/repo@v1.0.0\n" {
		t.Errorf("marshal = %q, want bare string", got)
	}
}

// TestModuleBinding_YAML_ObjectForm verifies the {source, props} form and
// that it marshals back to an object (props preserved).
func TestModuleBinding_YAML_ObjectForm(t *testing.T) {
	var m ModuleBinding
	in := "source: host/owner/repo@v1.0.0\nprops:\n  dir: web\n"
	if err := yaml.Unmarshal([]byte(in), &m); err != nil {
		t.Fatalf("unmarshal object form: %v", err)
	}
	if m.Source != "host/owner/repo@v1.0.0" {
		t.Errorf("Source = %q", m.Source)
	}
	if m.Props["dir"] != "web" {
		t.Errorf("Props[dir] = %v, want web", m.Props["dir"])
	}
	out, err := yaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var round ModuleBinding
	if err := yaml.Unmarshal(out, &round); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if round.Source != m.Source || round.Props["dir"] != "web" {
		t.Errorf("round-trip mismatch: %+v", round)
	}
}

// TestModuleBinding_ObjectForm_RequiresSource rejects an object without a
// source key with a clear error.
func TestModuleBinding_ObjectForm_RequiresSource(t *testing.T) {
	var m ModuleBinding
	err := yaml.Unmarshal([]byte("props:\n  dir: web\n"), &m)
	if err == nil {
		t.Fatal("expected error for object form missing source")
	}
}

// TestModuleBinding_JSON mirrors the YAML behavior for plan artifacts:
// string ↔ bare source, object ↔ {source, props}.
func TestModuleBinding_JSON(t *testing.T) {
	var s ModuleBinding
	if err := json.Unmarshal([]byte(`"host/owner/repo@v1"`), &s); err != nil {
		t.Fatalf("json string: %v", err)
	}
	if s.Source != "host/owner/repo@v1" || s.Props != nil {
		t.Errorf("string form = %+v", s)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(b) != `"host/owner/repo@v1"` {
		t.Errorf("marshal string form = %s", b)
	}

	withProps := ModuleBinding{Source: "x@v1", Props: map[string]interface{}{"dir": "web"}}
	b, err = json.Marshal(withProps)
	if err != nil {
		t.Fatalf("marshal object: %v", err)
	}
	var round ModuleBinding
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatalf("round-trip: %v", err)
	}
	if round.Source != "x@v1" || round.Props["dir"] != "web" {
		t.Errorf("json round-trip mismatch: %+v", round)
	}
}
