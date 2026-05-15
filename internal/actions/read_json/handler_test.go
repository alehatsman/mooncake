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
	// Non-string scalar untouched.
	if v["port"].(float64) != 80 {
		t.Errorf("expected port=80, got %v", v["port"])
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
