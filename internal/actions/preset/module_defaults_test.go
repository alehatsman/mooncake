package preset

import (
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// render stub: uppercases {{ X }} → value from a tiny env, else passthrough.
func testRender(vars map[string]string) func(string) (string, error) {
	return func(s string) (string, error) {
		for k, v := range vars {
			s = strings.ReplaceAll(s, "{{ "+k+" }}", v)
		}
		return s, nil
	}
}

func declared(names ...string) map[string]config.PresetParameter {
	m := make(map[string]config.PresetParameter, len(names))
	for _, n := range names {
		m[n] = config.PresetParameter{}
	}
	return m
}

// TestMergeModuleDefaults_FiltersUndeclared is the #57 core: a default prop the
// component doesn't declare is silently skipped (not injected → no "unknown
// parameter" failure), while declared defaults are applied and rendered.
func TestMergeModuleDefaults_FiltersUndeclared(t *testing.T) {
	render := testRender(map[string]string{"GO_TAGS": "sqlite_fts5"})

	// Component declares only cap_gocyclo (like goq/budget-status) — the
	// go_tags default must be dropped.
	got, err := mergeModuleDefaults(
		nil,
		map[string]interface{}{"go_tags": "{{ GO_TAGS }}"},
		declared("cap_gocyclo"),
		render,
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if _, leaked := got["go_tags"]; leaked {
		t.Errorf("undeclared default go_tags leaked into props: %v", got)
	}

	// Component DOES declare go_tags (like goq/lint) — applied + rendered.
	got, err = mergeModuleDefaults(
		nil,
		map[string]interface{}{"go_tags": "{{ GO_TAGS }}"},
		declared("go_tags", "pkg"),
		render,
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got["go_tags"] != "sqlite_fts5" {
		t.Errorf("declared default go_tags = %v, want rendered sqlite_fts5", got["go_tags"])
	}
}

// TestMergeModuleDefaults_CallerWins verifies a per-call prop overrides the
// module default, and caller-only keys survive.
func TestMergeModuleDefaults_CallerWins(t *testing.T) {
	render := testRender(nil)
	got, err := mergeModuleDefaults(
		map[string]interface{}{"dir": "other", "extra": true},
		map[string]interface{}{"dir": "web"},
		declared("dir"),
		render,
	)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if got["dir"] != "other" {
		t.Errorf("dir = %v, want other (per-call wins)", got["dir"])
	}
	if got["extra"] != true {
		t.Errorf("caller-only key dropped: %v", got)
	}
}

// TestMergeModuleDefaults_NoDefaults returns the caller map untouched.
func TestMergeModuleDefaults_NoDefaults(t *testing.T) {
	caller := map[string]interface{}{"dir": "web"}
	got, err := mergeModuleDefaults(caller, nil, declared("dir"), testRender(nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(got) != 1 || got["dir"] != "web" {
		t.Errorf("got %v, want the caller map unchanged", got)
	}
}
