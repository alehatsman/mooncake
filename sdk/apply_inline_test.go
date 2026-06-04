package mooncake_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// TestApplySteps_NoFileOnDisk is the #142 acceptance: a consumer synthesizes a
// one-step plan in memory, dispatches it through ApplySteps with no YAML on
// disk, gets a typed ApplyResult, and a caller Subscriber sees the lifecycle.
// The step uses the #111 generic carrier (Action/With) so the test needs no
// re-exported payload types — those land with the Edit/Write/Exec helpers (#144).
func TestApplySteps_NoFileOnDisk(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	reg := mooncake.DefaultRegistry()
	if err := reg.Register(markerHandler{path: marker}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	steps := []mooncake.Step{{
		Name:   "run custom marker",
		Action: "demo.marker",
		With:   map[string]interface{}{"note": "hi"},
	}}

	rec := &recordingSubscriber{}
	res, err := mooncake.ApplySteps(context.Background(), steps, mooncake.ApplyOptions{
		Registry:    reg,
		Subscribers: []mooncake.Subscriber{rec},
	})
	if err != nil {
		t.Fatalf("ApplySteps: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("custom action did not run: marker %s missing: %v", marker, statErr)
	}
	if !rec.saw(mooncake.EventRunStarted) || !rec.saw(mooncake.EventRunCompleted) {
		t.Errorf("subscriber missed run lifecycle; saw %v", rec.types)
	}
}

// TestApplyBytes_RawYAML proves ApplyBytes parses raw YAML and runs it with no
// temp file written to disk.
func TestApplyBytes_RawYAML(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "viabytes.txt")

	yamlBytes := []byte(`
- name: write via bytes
  file.write:
    path: ` + target + `
    state: file
    content: "from-bytes\n"
    mode: "0644"
`)

	res, err := mooncake.ApplyBytes(context.Background(), yamlBytes, mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("ApplyBytes: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	if got, _ := os.ReadFile(target); string(got) != "from-bytes\n" {
		t.Errorf("file content = %q; want %q", got, "from-bytes\n")
	}
}

// TestApplyConfig_CustomActionAndRegistry proves the in-memory path threads a
// consumer-owned Registry identically to ConfigPath-based Apply: a custom typed
// action registered only in the passed Registry dispatches and runs, and never
// leaks into the global.
func TestApplyConfig_CustomActionAndRegistry(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")

	reg := mooncake.DefaultRegistry()
	if err := reg.Register(markerHandler{path: marker}); err != nil {
		t.Fatalf("register custom handler: %v", err)
	}

	cfg := &mooncake.Config{
		Steps: []mooncake.Step{{
			Name:   "run custom marker",
			Action: "demo.marker",
			With:   map[string]interface{}{"note": "hi"},
		}},
	}

	res, err := mooncake.ApplyConfig(context.Background(), cfg, mooncake.ApplyOptions{Registry: reg})
	if err != nil {
		t.Fatalf("ApplyConfig: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	if _, statErr := os.Stat(marker); statErr != nil {
		t.Errorf("custom action did not run: marker %s missing: %v", marker, statErr)
	}
	if mooncake.GlobalRegistry().Has("demo.marker") {
		t.Error("custom action leaked into the global registry")
	}
}

// TestApplyConfig_NilRejected proves a nil config returns a typed failed result
// plus an error, not a panic.
func TestApplyConfig_NilRejected(t *testing.T) {
	res, err := mooncake.ApplyConfig(context.Background(), nil, mooncake.ApplyOptions{})
	if err == nil {
		t.Fatal("ApplyConfig(nil) returned nil error; want failure")
	}
	if res == nil {
		t.Fatal("ApplyConfig(nil) returned nil result; want a populated failed result")
	}
	if res.Summary.Success {
		t.Error("Summary.Success=true on nil config; want false")
	}
}

// TestApplyBytes_TemplateExpansion proves inline input goes through the
// planner's expansion pipeline — not a parallel raw path: a templated path
// ({{ out_dir }}) resolves from the config's own vars before execution.
func TestApplyBytes_TemplateExpansion(t *testing.T) {
	dir := t.TempDir()

	yamlBytes := []byte(`
vars:
  out_dir: ` + dir + `
steps:
  - name: write via template
    file.write:
      path: "{{ out_dir }}/templated.txt"
      state: file
      content: "expanded\n"
      mode: "0644"
`)

	res, err := mooncake.ApplyBytes(context.Background(), yamlBytes, mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("ApplyBytes: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	target := filepath.Join(dir, "templated.txt")
	if got, _ := os.ReadFile(target); string(got) != "expanded\n" {
		t.Errorf("template did not expand: %s content=%q want %q", target, got, "expanded\n")
	}
}

// TestApplyBytes_PolicyGate proves the Policy gate threads through the in-memory
// path: a denied action fails the run before its side effect.
func TestApplyBytes_PolicyGate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "shouldnotexist.txt")

	yamlBytes := []byte(`
- name: blocked write
  file.write:
    path: ` + target + `
    state: file
    content: "nope\n"
    mode: "0644"
`)

	res, _ := mooncake.ApplyBytes(context.Background(), yamlBytes, mooncake.ApplyOptions{
		Policy: &mooncake.Policy{DeniedActions: []string{"file.write"}},
	})
	if res.Summary.Success {
		t.Error("Summary.Success=true; policy should have blocked file.write")
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Errorf("file %s created despite policy deny", target)
	}
}
