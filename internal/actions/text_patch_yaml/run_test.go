//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func writeYAML(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readYAMLFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no path", &config.Step{TextPatchYAML: &config.TextPatchYAML{Set: map[string]interface{}{"x": 1}}}, true},
		{"no ops", &config.Step{TextPatchYAML: &config.TextPatchYAML{Path: "/tmp/a.yml"}}, true},
		{"bad path syntax", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path: "/tmp/a.yml",
			Set:  map[string]interface{}{"a.*.b": 1},
		}}, true},
		{"bad merge strategy", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path:          "/tmp/a.yml",
			Merge:         map[string]interface{}{"a": []interface{}{1}},
			MergeStrategy: "smush",
		}}, true},
		{"set+delete conflict", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path:   "/tmp/a.yml",
			Set:    map[string]interface{}{"a": 1},
			Delete: []string{"a"},
		}}, true},
		{"ok set", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path: "/tmp/a.yml",
			Set:  map[string]interface{}{"service.port": 8080},
		}}, false},
		{"ok delete", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path:   "/tmp/a.yml",
			Delete: []string{"deprecated.field"},
		}}, false},
		{"ok merge", &config.Step{TextPatchYAML: &config.TextPatchYAML{
			Path:  "/tmp/a.yml",
			Merge: map[string]interface{}{"tags": []interface{}{"v2"}},
		}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestSet_BasicScalar(t *testing.T) {
	src := "service:\n    port: 3000\n    host: 0.0.0.0\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"service.port": 8080},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "port: 8080") {
		t.Errorf("set did not apply; got %q", got)
	}
	if !strings.Contains(got, "host: 0.0.0.0") {
		t.Errorf("unrelated keys lost; got %q", got)
	}
}

func TestSet_PreservesCommentsOnUnchangedKeys(t *testing.T) {
	src := `# top-level comment
service:
    # inline comment on host
    host: 0.0.0.0
    port: 3000
`
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"service.port": 8080},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "# inline comment on host") {
		t.Errorf("adjacent comment lost; got %q", got)
	}
	// Note: yaml.v3 may move some head comments during round-trip; the
	// "comments on unchanged keys" guarantee is what matters here.
}

func TestSet_PreservesKeyOrder(t *testing.T) {
	src := "alpha: 1\nbeta: 2\ngamma: 3\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"beta": 22},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	idxA := strings.Index(got, "alpha")
	idxB := strings.Index(got, "beta")
	idxG := strings.Index(got, "gamma")
	if !(idxA < idxB && idxB < idxG) {
		t.Errorf("key order disturbed; got %q", got)
	}
}

func TestSet_Idempotent(t *testing.T) {
	src := "a: 1\nb: 2\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"a": 1},
	}}
	r1 := mustRun(t, false, step)
	r2 := mustRun(t, false, step)
	if r1.Changed {
		t.Errorf("set to same value should be noop; reason=%q", r1.Reason)
	}
	if r2.Changed {
		t.Errorf("second run should be noop; reason=%q", r2.Reason)
	}
}

func TestSet_CreatesMissingPath(t *testing.T) {
	src := "a: 1\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"b.c": "hello"},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "c: hello") {
		t.Errorf("missing path not created; got %q", got)
	}
}

func TestDelete_Basic(t *testing.T) {
	src := "keep: yes\ndrop: go\nnested:\n  kept: 1\n  removed: 2\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path:   path,
		Delete: []string{"drop", "nested.removed"},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if strings.Contains(got, "drop") || strings.Contains(got, "removed") {
		t.Errorf("delete left residue; got %q", got)
	}
	if !strings.Contains(got, "keep: yes") || !strings.Contains(got, "kept: 1") {
		t.Errorf("delete removed too much; got %q", got)
	}
}

func TestDelete_MissingPathIsNoop(t *testing.T) {
	src := "a: 1\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path:   path,
		Delete: []string{"does.not.exist"},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("delete on missing path should be noop; reason=%q", r.Reason)
	}
}

func TestArrayIndex(t *testing.T) {
	src := "list:\n  - a\n  - b\n  - c\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"list[1]": "B"},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "B") {
		t.Errorf("array index set failed; got %q", got)
	}
	if !strings.Contains(got, "- a") || !strings.Contains(got, "- c") {
		t.Errorf("array siblings lost; got %q", got)
	}
}

func TestMerge_ObjectNonDestructive(t *testing.T) {
	src := "env:\n  EXISTING: keep-me\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Merge: map[string]interface{}{
			"env": map[string]interface{}{
				"EXISTING": "overwritten?",
				"NEW":      "added",
			},
		},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "EXISTING: keep-me") {
		t.Errorf("merge overwrote existing key; got %q", got)
	}
	if !strings.Contains(got, "NEW: added") {
		t.Errorf("merge did not add new key; got %q", got)
	}
}

func TestMerge_ArrayAppendUnique(t *testing.T) {
	src := "tags:\n  - a\n  - b\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path:  path,
		Merge: map[string]interface{}{"tags": []interface{}{"a", "c"}},
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if !strings.Contains(got, "- c") {
		t.Errorf("append_unique missed new element; got %q", got)
	}
	aCount := strings.Count(got, "- a")
	if aCount != 1 {
		t.Errorf("append_unique duplicated existing; got %d 'a's: %q", aCount, got)
	}
}

func TestMerge_ArrayReplace(t *testing.T) {
	src := "tags:\n  - a\n  - b\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path:          path,
		Merge:         map[string]interface{}{"tags": []interface{}{"x", "y"}},
		MergeStrategy: "replace",
	}}
	_ = mustRun(t, false, step)
	got := readYAMLFile(t, path)
	if strings.Contains(got, "- a") || strings.Contains(got, "- b") {
		t.Errorf("replace kept old elements; got %q", got)
	}
}

func TestPlan_DoesNotWrite(t *testing.T) {
	src := "a: 1\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"a": 2},
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should set WouldChange; reason=%q", r.Reason)
	}
	got := readYAMLFile(t, path)
	if got != src {
		t.Errorf("plan modified file; got %q want %q", got, src)
	}
}

func TestMissingFileIsError(t *testing.T) {
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: "/tmp/definitely-not-a-real-file-abc123.yml",
		Set:  map[string]interface{}{"a": 1},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error on missing file")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected file-not-found message; got %v", err)
	}
}

func TestParseErrorSurfaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yml")
	// Mapping with the same key twice is rejected by yaml.v3 in strict
	// mode but accepted otherwise; use mismatched braces for a guaranteed
	// parse error.
	if err := os.WriteFile(path, []byte("{not yaml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path: path,
		Set:  map[string]interface{}{"a": 1},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error message; got %v", err)
	}
}

func TestBackupCreated(t *testing.T) {
	src := "a: 1\n"
	path := writeYAML(t, src)
	step := &config.Step{TextPatchYAML: &config.TextPatchYAML{
		Path:   path,
		Set:    map[string]interface{}{"a": 2},
		Backup: true,
	}}
	_ = mustRun(t, false, step)
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Errorf("backup not created: %v", err)
	}
}

func TestPathParseRejectsUnsupportedSyntax(t *testing.T) {
	cases := []string{
		"a.*.b",
		"$.a",
		".a",
		"a.",
		"a..b",
		"a[-1]",
		"a[",
		"a[]",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := validatePath(p); err == nil {
				t.Errorf("expected error for %q", p)
			}
		})
	}
}
