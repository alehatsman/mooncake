package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/security"
)

// TestSavePlanToFile_RedactsMarker exercises the spec-23 §3 plan-output
// rule: in-memory marker strings get rewritten to readable `!secret <ref>`
// form on disk. The actual secret value never appears in the plan (it's
// never even resolved during plan-mode), but the marker itself contains
// a control byte — replacing it with the readable form makes plans
// share-able / inspectable.
func TestSavePlanToFile_RedactsMarker(t *testing.T) {
	// Build a minimal plan with a step whose action carries a marker.
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
		// The in-memory marker (which contains a NUL byte) must NOT appear
		// on disk. The on-disk form is `!secret env:APP_TOKEN`.
		if strings.Contains(out, security.SentinelPrefix) {
			t.Errorf("%s output contains raw sentinel:\n%s", ext, out)
		}
		if !strings.Contains(out, "!secret env:APP_TOKEN") {
			t.Errorf("%s output missing readable form:\n%s", ext, out)
		}
	}
}

// TestRedactSecretMarkers_NoMarkerNoChange: when the input has no
// marker, the function returns the input unchanged (and ideally the
// SAME byte slice — but at minimum equal contents).
func TestRedactSecretMarkers_NoMarkerNoChange(t *testing.T) {
	in := []byte(`{"foo": "bar"}`)
	out := redactSecretMarkers(in)
	if string(out) != string(in) {
		t.Errorf("plain input mutated:\ngot:  %q\nwant: %q", out, in)
	}
}

// TestRedactSecretMarkers_MultipleMarkers replaces every occurrence,
// not just the first. Realistic for a multi-step plan.
func TestRedactSecretMarkers_MultipleMarkers(t *testing.T) {
	in := []byte(`{"a":"` + security.SentinelPrefix + `env:A","b":"` + security.SentinelPrefix + `env:B"}`)
	out := redactSecretMarkers(in)
	s := string(out)
	if strings.Contains(s, security.SentinelPrefix) {
		t.Errorf("marker still present: %q", s)
	}
	if !strings.Contains(s, "!secret env:A") || !strings.Contains(s, "!secret env:B") {
		t.Errorf("missing one or both readable forms: %q", s)
	}
}
