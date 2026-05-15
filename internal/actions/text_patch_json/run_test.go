//nolint:revive // package name follows action convention
package text_patch_json

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

func writeFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
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
		{"no path", &config.Step{TextPatchJSON: &config.TextPatchJSON{Set: map[string]interface{}{"x": 1}}}, true},
		{"no ops", &config.Step{TextPatchJSON: &config.TextPatchJSON{Path: "/tmp/a.json"}}, true},
		{"bad path syntax", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path: "/tmp/a.json",
			Set:  map[string]interface{}{"a.*.b": 1},
		}}, true},
		{"bad merge strategy", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path:          "/tmp/a.json",
			Merge:         map[string]interface{}{"a": []interface{}{1}},
			MergeStrategy: "smush",
		}}, true},
		{"set+delete conflict", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path:   "/tmp/a.json",
			Set:    map[string]interface{}{"a": 1},
			Delete: []string{"a"},
		}}, true},
		{"ok set", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path: "/tmp/a.json",
			Set:  map[string]interface{}{"service.port": 8080},
		}}, false},
		{"ok delete", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path:   "/tmp/a.json",
			Delete: []string{"deprecated.field"},
		}}, false},
		{"ok merge", &config.Step{TextPatchJSON: &config.TextPatchJSON{
			Path:  "/tmp/a.json",
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

// MT-32: the "no ops" validate error should explicitly call out
// RFC 6902 / JSON Patch operations: as unsupported, since that's
// the canonical form users from a json-patch background type
// first.
func TestValidate_NoOpsErrorMentionsRFC6902(t *testing.T) {
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{Path: "/tmp/a.json"}}
	err := (&Handler{}).Validate(step)
	if err == nil {
		t.Fatal("expected validate to error on no ops")
	}
	if !strings.Contains(err.Error(), "RFC 6902") {
		t.Errorf("error should cite RFC 6902 explicitly; got: %v", err)
	}
	if !strings.Contains(err.Error(), "set:") || !strings.Contains(err.Error(), "delete:") || !strings.Contains(err.Error(), "merge:") {
		t.Errorf("error should redirect to set:/delete:/merge:; got: %v", err)
	}
}

func TestSet_PreservesOrderAndIndent(t *testing.T) {
	src := `{
  "name": "myapp",
  "version": "1.0.0",
  "service": {
    "port": 3000,
    "host": "0.0.0.0"
  },
  "scripts": {
    "start": "node index.js"
  }
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"service.port": 8080},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	want := `{
  "name": "myapp",
  "version": "1.0.0",
  "service": {
    "port": 8080,
    "host": "0.0.0.0"
  },
  "scripts": {
    "start": "node index.js"
  }
}
`
	got := readFile(t, path)
	if got != want {
		t.Errorf("content mismatch\n got %q\nwant %q", got, want)
	}
}

func TestSet_CreatesMissingPath(t *testing.T) {
	src := `{
  "a": 1
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"b.c": "hello"},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if !strings.Contains(got, `"c": "hello"`) {
		t.Errorf("missing new key path; got %q", got)
	}
}

func TestSet_Idempotent(t *testing.T) {
	src := `{
  "a": 1,
  "b": 2
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a": 1},
	}}
	r1 := mustRun(t, false, step)
	r2 := mustRun(t, false, step)
	if r1.Changed {
		t.Errorf("first run set to same value should be noop; reason=%q", r1.Reason)
	}
	if r2.Changed {
		t.Errorf("second run should be noop; reason=%q", r2.Reason)
	}
}

func TestSet_TwoSpaceIndent(t *testing.T) {
	src := `{
  "a": {
    "b": 1
  }
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a.b": 2},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if !strings.Contains(got, "  \"a\": {\n    \"b\": 2\n  }") {
		t.Errorf("indent not preserved; got %q", got)
	}
}

func TestSet_TabIndent(t *testing.T) {
	src := "{\n\t\"a\": 1\n}\n"
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a": 2},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	want := "{\n\t\"a\": 2\n}\n"
	if got != want {
		t.Errorf("tab indent not preserved\n got %q\nwant %q", got, want)
	}
}

func TestDelete_Basic(t *testing.T) {
	src := `{
  "keep": "yes",
  "drop": "go",
  "nested": {
    "kept": 1,
    "removed": 2
  }
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path:   path,
		Delete: []string{"drop", "nested.removed"},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if strings.Contains(got, "drop") || strings.Contains(got, "removed") {
		t.Errorf("delete left residue; got %q", got)
	}
	if !strings.Contains(got, `"keep": "yes"`) || !strings.Contains(got, `"kept": 1`) {
		t.Errorf("delete removed too much; got %q", got)
	}
}

func TestDelete_MissingPathIsNoop(t *testing.T) {
	src := `{"a": 1}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path:   path,
		Delete: []string{"does.not.exist"},
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("delete on missing path should be noop; reason=%q", r.Reason)
	}
}

func TestArrayIndex(t *testing.T) {
	src := `{
  "list": [
    "a",
    "b",
    "c"
  ]
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"list[1]": "B"},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if !strings.Contains(got, `"B"`) {
		t.Errorf("array index set failed; got %q", got)
	}
	if !strings.Contains(got, `"a"`) || !strings.Contains(got, `"c"`) {
		t.Errorf("array sibling elements lost; got %q", got)
	}
}

func TestMerge_ObjectNonDestructive(t *testing.T) {
	src := `{
  "env": {
    "EXISTING": "keep-me"
  }
}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Merge: map[string]interface{}{
			"env": map[string]interface{}{
				"EXISTING": "overwritten?",
				"NEW":      "added",
			},
		},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if !strings.Contains(got, `"EXISTING": "keep-me"`) {
		t.Errorf("merge overwrote existing key; got %q", got)
	}
	if !strings.Contains(got, `"NEW": "added"`) {
		t.Errorf("merge did not add new key; got %q", got)
	}
}

func TestMerge_ArrayAppendUnique(t *testing.T) {
	src := `{"tags": ["a", "b"]}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path:  path,
		Merge: map[string]interface{}{"tags": []interface{}{"a", "c"}},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if !strings.Contains(got, `"c"`) {
		t.Errorf("append_unique missed new element; got %q", got)
	}
	if strings.Count(got, `"a"`) != 1 {
		t.Errorf("append_unique duplicated existing; got %q", got)
	}
}

func TestMerge_ArrayReplace(t *testing.T) {
	src := `{"tags": ["a", "b"]}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path:          path,
		Merge:         map[string]interface{}{"tags": []interface{}{"x", "y"}},
		MergeStrategy: "replace",
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if strings.Contains(got, `"a"`) || strings.Contains(got, `"b"`) {
		t.Errorf("replace kept old elements; got %q", got)
	}
}

func TestMerge_ArrayAppend(t *testing.T) {
	src := `{"events": ["a"]}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path:          path,
		Merge:         map[string]interface{}{"events": []interface{}{"a", "b"}},
		MergeStrategy: "append",
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	// append (not unique) → duplicate of "a"
	if strings.Count(got, `"a"`) != 2 {
		t.Errorf("append should duplicate; got %q", got)
	}
}

func TestPlan_DoesNotWrite(t *testing.T) {
	src := `{"a": 1}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a": 2},
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should set WouldChange; reason=%q", r.Reason)
	}
	got := readFile(t, path)
	if got != src {
		t.Errorf("plan modified file; got %q", got)
	}
}

func TestMissingFileIsError(t *testing.T) {
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: "/tmp/definitely-not-a-real-file-abc123.json",
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
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
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

func TestCompactJSONStaysCompact(t *testing.T) {
	src := `{"a":1,"b":2}`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a": 5},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	want := `{"a":5,"b":2}`
	if got != want {
		t.Errorf("compact emit broken\n got %q\nwant %q", got, want)
	}
}

func TestNoTrailingNewlinePreserved(t *testing.T) {
	src := `{"a": 1}` // no trailing \n
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
		Path: path,
		Set:  map[string]interface{}{"a": 2},
	}}
	_ = mustRun(t, false, step)
	got := readFile(t, path)
	if strings.HasSuffix(got, "\n") {
		t.Errorf("trailing newline added; got %q", got)
	}
}

func TestBackupCreated(t *testing.T) {
	src := `{"a": 1}
`
	path := writeFile(t, src)
	step := &config.Step{TextPatchJSON: &config.TextPatchJSON{
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

func TestPathParseAccepts(t *testing.T) {
	cases := []string{
		"a",
		"a.b.c",
		"a[0]",
		"a.b[3].c",
		"x[0][1]",
	}
	for _, p := range cases {
		t.Run(p, func(t *testing.T) {
			if err := validatePath(p); err != nil {
				t.Errorf("unexpected error for %q: %v", p, err)
			}
		})
	}
}
