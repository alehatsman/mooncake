package read_yaml

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
		t.Errorf("read.yaml must declare CaptureInPlan: true")
	}
	if m.Name != "read.yaml" {
		t.Errorf("name = %q", m.Name)
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
	step := &config.Step{ReadYAML: rf}
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
	p := writeFile(t, dir, "a.yml", "service:\n  port: 8080\nname: api\n")
	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	v := res.Data["value"].(map[string]any)
	if v["name"] != "api" {
		t.Errorf("name = %v", v["name"])
	}
}

func TestRun_QueryHits(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yml", "service:\n  port: 8080\n")
	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "service.port"})
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if res.Data["value"] != 8080 {
		t.Errorf("value = %v (%T)", res.Data["value"], res.Data["value"])
	}
}

func TestRun_QueryMissesIsNotError(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yml", "a: 1\n")
	ctx := newExecCtx(t, dir, actions.ModeApply)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "b"})
	if err != nil {
		t.Fatalf("query miss must not error: %v", err)
	}
	if res.Data["found"].(bool) {
		t.Error("expected found=false")
	}
}

func TestRun_MissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	ctx := newExecCtx(t, dir, actions.ModeApply)
	_, err := runRead(t, ctx, &config.ReadFile{Path: filepath.Join(dir, "absent.yml")})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected not-found error, got %v", err)
	}
}

func TestRun_MultiDocYAMLRejected(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "multi.yml", "a: 1\n---\nb: 2\n")
	ctx := newExecCtx(t, dir, actions.ModeApply)
	_, err := runRead(t, ctx, &config.ReadFile{Path: p})
	if err == nil || !strings.Contains(err.Error(), "multi-document") {
		t.Errorf("expected multi-document error, got %v", err)
	}
}

func TestRun_RedactPatternApplies(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "s.yml", "token: ghp_AAAAAAAAAAAAAAAAAAAAAAAAA\nport: 80\n")
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
	if v["port"] != 80 {
		t.Errorf("expected port=80, got %v", v["port"])
	}
}

func TestRun_PlanModeCheckable(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "a.yml", "v: 1.0\n")
	ctx := newExecCtx(t, dir, actions.ModePlan)
	res, err := runRead(t, ctx, &config.ReadFile{Path: p, Query: "v"})
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if !res.Checkable || res.Changed {
		t.Errorf("plan-mode result must be Checkable, never Changed (got checkable=%v changed=%v)",
			res.Checkable, res.Changed)
	}
}
