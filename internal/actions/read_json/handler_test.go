package read_json

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestMetadata_CaptureInPlanIsOn(t *testing.T) {
	m := Handler{}.Metadata()
	if !m.CaptureInPlan {
		t.Errorf("read.json must declare CaptureInPlan: true")
	}
	if m.Name != "read.json" {
		t.Errorf("name = %q", m.Name)
	}
}

func TestValidate_MissingPathErrors(t *testing.T) {
	h := Handler{}
	if err := h.Validate(&config.Step{ReadJSON: &config.ReadFile{}}); err == nil {
		t.Fatal("expected error for missing path")
	}
}

func TestValidate_BadQueryErrors(t *testing.T) {
	h := Handler{}
	err := h.Validate(&config.Step{ReadJSON: &config.ReadFile{Path: "a", Query: "a..b"}})
	if err == nil || !strings.Contains(err.Error(), "query") {
		t.Errorf("expected query error, got %v", err)
	}
}

func TestValidate_BadRedactErrors(t *testing.T) {
	h := Handler{}
	err := h.Validate(&config.Step{ReadJSON: &config.ReadFile{
		Path:   "a",
		Redact: []string{"[unclosed"},
	}})
	if err == nil || !strings.Contains(err.Error(), "redact[0]") {
		t.Errorf("expected redact[0] error, got %v", err)
	}
}

func TestValidate_BadMaxBytesErrors(t *testing.T) {
	h := Handler{}
	zero := int64(0)
	err := h.Validate(&config.Step{ReadJSON: &config.ReadFile{
		Path: "a", MaxBytes: &zero,
	}})
	if err == nil {
		t.Fatal("expected error for max_bytes <= 0")
	}
}

func newExecCtx(t *testing.T, dir string, mode actions.Mode) *executor.ExecutionContext {
	t.Helper()
	rend, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	mock := testutil.NewMockContext()
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       rend,
			EventPublisher: mock.Publisher,
			Logger:         mock.Log,
			PathUtil:       pathutil.NewPathExpander(rend),
			Stats:          executor.NewExecutionStats(),
			Mode:           mode,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: dir,
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func runRead(t *testing.T, ctx *executor.ExecutionContext, rf *config.ReadFile) (*executor.Result, error) {
	t.Helper()
	h := Handler{}
	step := &config.Step{ReadJSON: rf}
	if err := h.Validate(step); err != nil {
		return nil, err
	}
	res, err := h.Run(ctx, step)
	if res == nil {
		return nil, err
	}
	return res.(*executor.Result), err
}

func TestRun_WholeDocument(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.json", `{"version":"1.2.3","port":8080}`)

	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	d := res.Data
	if got, ok := d["found"].(bool); !ok || !got {
		t.Errorf("expected found=true, got %v", d["found"])
	}
	val, _ := d["value"].(map[string]any)
	if val["version"] != "1.2.3" {
		t.Errorf("value.version=%v", val["version"])
	}
	if d["bytes_read"].(int64) <= 0 {
		t.Errorf("bytes_read should be > 0, got %v", d["bytes_read"])
	}
}

// MT-79: integer JSON literals must round-trip as int64 (not
// float64), matching read.yaml's preservation of int vs float. Before
// the fix, `{"port": 8080}` came back as 8080.0 in templates —
// "--port 8080.000000" in command strings.
func TestRun_IntegerLiteralsStayInt64(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "ints.json", `{"port":8080,"workers":4,"rate":0.5,"big":2147483648,"neg":-7}`)

	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val := res.Data["value"].(map[string]any)

	if got, ok := val["port"].(int64); !ok || got != 8080 {
		t.Errorf("port: want int64(8080), got %T %v", val["port"], val["port"])
	}
	if got, ok := val["workers"].(int64); !ok || got != 4 {
		t.Errorf("workers: want int64(4), got %T %v", val["workers"], val["workers"])
	}
	if got, ok := val["rate"].(float64); !ok || got != 0.5 {
		t.Errorf("rate: want float64(0.5), got %T %v", val["rate"], val["rate"])
	}
	if got, ok := val["big"].(int64); !ok || got != 2147483648 {
		t.Errorf("big: want int64(2147483648), got %T %v", val["big"], val["big"])
	}
	if got, ok := val["neg"].(int64); !ok || got != -7 {
		t.Errorf("neg: want int64(-7), got %T %v", val["neg"], val["neg"])
	}
}

// MT-79: nested structures (maps in slices, slices in maps) must
// also walk for json.Number conversion. The recursive normalizer
// touches every numeric leaf, not just top-level ones.
func TestRun_IntegerLiteralsInNestedStructures(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "nested.json", `{
		"pkg": {"name": "jq", "deps": [{"name": "oniguruma", "version_major": 6}]},
		"counts": [1, 2, 3]
	}`)

	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val := res.Data["value"].(map[string]any)

	pkg := val["pkg"].(map[string]any)
	deps := pkg["deps"].([]any)
	dep0 := deps[0].(map[string]any)
	if got, ok := dep0["version_major"].(int64); !ok || got != 6 {
		t.Errorf("dep0.version_major: want int64(6), got %T %v", dep0["version_major"], dep0["version_major"])
	}

	counts := val["counts"].([]any)
	for i, want := range []int64{1, 2, 3} {
		if got, ok := counts[i].(int64); !ok || got != want {
			t.Errorf("counts[%d]: want int64(%d), got %T %v", i, want, counts[i], counts[i])
		}
	}
}

func TestRun_QueryHits(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.json", `{"tools":[{"name":"go"},{"name":"gopls"}]}`)
	ctx := newExecCtx(t, dir, actions.ModeApply)

	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "tools[1].name"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Data["value"] != "gopls" {
		t.Errorf("value = %v", res.Data["value"])
	}
}

func TestRun_QueryMissesIsNotError(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.json", `{"a":1}`)
	ctx := newExecCtx(t, dir, actions.ModeApply)

	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "b"})
	if err != nil {
		t.Fatalf("query miss must not error: %v", err)
	}
	if res.Data["found"].(bool) {
		t.Error("expected found=false")
	}
	if res.Data["value"] != nil {
		t.Errorf("expected nil value on miss, got %v", res.Data["value"])
	}
}

func TestRun_MissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	ctx := newExecCtx(t, dir, actions.ModeApply)
	_, err := runRead(t, ctx, &config.ReadFile{Path: filepath.Join(dir, "absent.json")})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestRun_MalformedJSONErrors(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "bad.json", `{not valid json`)
	ctx := newExecCtx(t, dir, actions.ModeApply)
	_, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err == nil || !strings.Contains(err.Error(), "JSON parse") {
		t.Errorf("expected parse error, got %v", err)
	}
}

func TestRun_MaxBytesOverflowErrors(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "big.json", `{"a":"`+strings.Repeat("x", 200)+`"}`)
	limit := int64(50)
	ctx := newExecCtx(t, dir, actions.ModeApply)
	_, err := runRead(t, ctx, &config.ReadFile{Path: p, MaxBytes: &limit})
	if err == nil || !strings.Contains(err.Error(), "exceeds max_bytes") {
		t.Errorf("expected max_bytes overflow error, got %v", err)
	}
}

func TestRun_RedactPatternApplies(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "secrets.json", `{"token":"ghp_AAAAAAAAAAAAAAAAAAAAAAAAA","port":80}`)
	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{
		Path:   p,
		Redact: []string{`ghp_[A-Za-z0-9]{20,}`},
	})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	v := res.Data["value"].(map[string]any)
	if v["token"] != "[REDACTED]" {
		t.Errorf("expected redacted token, got %v", v["token"])
	}
	// Non-string scalar untouched. MT-79: integer JSON literals come
	// back as int64 (not float64) so templated values render cleanly.
	if v["port"].(int64) != 80 {
		t.Errorf("expected port=80 (int64), got %v (%T)", v["port"], v["port"])
	}
}

func TestRun_PlanModeReasonReadsAndIsCheckable(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.json", `{"v":"1.0"}`)
	ctx := newExecCtx(t, dir, actions.ModePlan)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "v"})
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if !res.Checkable {
		t.Error("plan-mode result must be Checkable")
	}
	if !strings.Contains(res.Reason, "would read") {
		t.Errorf("plan reason missing 'would read': %q", res.Reason)
	}
	if res.Changed {
		t.Error("read.json must never set Changed=true")
	}
}

func TestRun_PlanModeQueryMissReasonMentionsMiss(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.json", `{"v":"1.0"}`)
	ctx := newExecCtx(t, dir, actions.ModePlan)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "missing"})
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if !strings.Contains(res.Reason, "query path missed") {
		t.Errorf("plan reason should mention miss, got %q", res.Reason)
	}
}
