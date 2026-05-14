package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/security"
)

// TestSavePlanToFile_RedactsMarkerDefault verifies the default plan
// output redacts secrets to a bare `!secret` (spec-23 §247) — the
// ref is stripped so plans are safe to share or attach to PRs without
// leaking any secret-shape information.
func TestSavePlanToFile_RedactsMarkerDefault(t *testing.T) {
	// Ensure the show-refs override is OFF.
	t.Setenv("MOONCAKE_SHOW_SECRET_REFS", "")

	plan := &Plan{
		Steps: []config.Step{
			{
				Name: "writes a secret",
				FileWrite: &config.File{
					Path:    "/tmp/x",
					Content: security.SentinelPrefix + "env:APP_TOKEN",
				},
			},
		},
	}

	dir := t.TempDir()
	for _, ext := range []string{".json", ".yaml"} {
		path := filepath.Join(dir, "plan"+ext)
		if err := SavePlanToFile(plan, path); err != nil {
			t.Fatalf("SavePlanToFile(%s): %v", ext, err)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		out := string(body)
		// Sentinel must be gone.
		if strings.Contains(out, security.SentinelPrefix) {
			t.Errorf("%s output contains raw sentinel:\n%s", ext, out)
		}
		// Bare `!secret` must be present.
		if !strings.Contains(out, "!secret") {
			t.Errorf("%s output missing bare !secret marker:\n%s", ext, out)
		}
		// The ref MUST be absent — that's the whole point of the default.
		if strings.Contains(out, "env:APP_TOKEN") {
			t.Errorf("%s output leaked the ref (default should hide it):\n%s", ext, out)
		}
	}
}

// TestSavePlanToFile_RedactsMarkerWithShowRefs covers the debug opt-in.
// Operators who want to see which secret refs are in a plan set
// MOONCAKE_SHOW_SECRET_REFS=1; the output then keeps the ref alongside
// `!secret`. The resolved value is still never present.
func TestSavePlanToFile_RedactsMarkerWithShowRefs(t *testing.T) {
	t.Setenv("MOONCAKE_SHOW_SECRET_REFS", "1")

	plan := &Plan{
		Steps: []config.Step{
			{
				Name: "writes a secret",
				FileWrite: &config.File{
					Path:    "/tmp/x",
					Content: security.SentinelPrefix + "env:APP_TOKEN",
				},
			},
		},
	}

	dir := t.TempDir()
	for _, ext := range []string{".json", ".yaml"} {
		path := filepath.Join(dir, "plan"+ext)
		if err := SavePlanToFile(plan, path); err != nil {
			t.Fatalf("SavePlanToFile(%s): %v", ext, err)
		}
		body, _ := os.ReadFile(path)
		out := string(body)
		if !strings.Contains(out, "!secret env:APP_TOKEN") {
			t.Errorf("%s output missing readable form with ref:\n%s", ext, out)
		}
	}
}

// TestRedactSecretMarkers_NoMarkerNoChange: plain input passes through
// unchanged regardless of the env var.
func TestRedactSecretMarkers_NoMarkerNoChange(t *testing.T) {
	in := []byte(`{"foo": "bar"}`)
	if got := redactSecretMarkers(in); string(got) != string(in) {
		t.Errorf("plain input mutated:\ngot:  %q\nwant: %q", got, in)
	}
}

// TestRedactSecretMarkers_MultipleMarkersDefault: every occurrence is
// rewritten under the bare-default rule, and no ref bleeds through.
func TestRedactSecretMarkers_MultipleMarkersDefault(t *testing.T) {
	t.Setenv("MOONCAKE_SHOW_SECRET_REFS", "")

	in := []byte(`{"a":"` + security.SentinelPrefix + `env:A","b":"` + security.SentinelPrefix + `env:B"}`)
	out := redactSecretMarkers(in)
	s := string(out)
	if strings.Contains(s, security.SentinelPrefix) {
		t.Errorf("marker still present: %q", s)
	}
	if strings.Contains(s, "env:A") || strings.Contains(s, "env:B") {
		t.Errorf("default output leaked a ref: %q", s)
	}
	// Two `!secret` occurrences, one per original marker.
	if got := strings.Count(s, "!secret"); got != 2 {
		t.Errorf("!secret count = %d, want 2: %q", got, s)
	}
}

// TestRedactSecretMarkers_MultipleMarkersWithShowRefs is the
// debuggability counterpart: every marker keeps its ref when
// MOONCAKE_SHOW_SECRET_REFS=1.
func TestRedactSecretMarkers_MultipleMarkersWithShowRefs(t *testing.T) {
	t.Setenv("MOONCAKE_SHOW_SECRET_REFS", "1")

	in := []byte(`{"a":"` + security.SentinelPrefix + `env:A","b":"` + security.SentinelPrefix + `env:B"}`)
	s := string(redactSecretMarkers(in))
	if !strings.Contains(s, "!secret env:A") || !strings.Contains(s, "!secret env:B") {
		t.Errorf("show-refs mode missing one or both readable forms: %q", s)
	}
}
