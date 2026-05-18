package containerruntime

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"text/template"
)

// TestMT64_InspectFormatReferencesRealFields is a regression test for
// manual-test #64 (2026-05-15): the inspectFormat constant used
// {{.ImageName}}, a field docker's `container inspect` JSON does not
// expose. The bad template produced
//
//	"template parsing error: ... map has no entry for key \"ImageName\""
//
// every time the idempotency check ran, so the second apply of any
// `container:` step failed. Pin the constant to the fields docker
// (and podman) actually expose.
func TestMT64_InspectFormatReferencesRealFields(t *testing.T) {
	if strings.Contains(inspectFormat, ".ImageName") {
		t.Fatalf("inspectFormat references .ImageName (does not exist on docker inspect JSON): %q", inspectFormat)
	}
	for _, want := range []string{".State.Status", ".Config.Image", ".Id"} {
		if !strings.Contains(inspectFormat, want) {
			t.Errorf("inspectFormat missing %q (got %q)", want, inspectFormat)
		}
	}
}

// TestMT64_InspectFormatParsesRealJSON applies the format against a
// docker-inspect-shaped JSON payload to prove the field paths
// actually resolve. Catches future regressions of the wrong-field
// type without needing a live docker engine.
func TestMT64_InspectFormatParsesRealJSON(t *testing.T) {
	// Minimal docker inspect JSON — covers the fields the format
	// reads and nothing else. docker emits this as a single-element
	// array; the runtime caller invokes inspect on a single name so
	// we test against the element shape directly.
	raw := []byte(`{
		"Id": "abc123",
		"State": { "Status": "running" },
		"Config": { "Image": "alpine:3.21" }
	}`)
	var data map[string]interface{}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("inspect").Parse(inspectFormat)
	if err != nil {
		t.Fatalf("parse format: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute format against real-shape JSON: %v", err)
	}
	got := buf.String()
	want := "running|alpine:3.21|abc123"
	if got != want {
		t.Errorf("inspectFormat result = %q, want %q", got, want)
	}
}
