package observe_http

import (
	"net/http"
	"net/http/httptest"
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
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func TestValidate(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObserveHTTP: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveHTTP: &config.ObserveHTTP{}}); err == nil {
		t.Fatal("expected error for empty URL")
	}
	if err := h.Validate(&config.Step{ObserveHTTP: &config.ObserveHTTP{URL: "https://x", Timeout: "bogus"}}); err == nil {
		t.Fatal("expected error for bad timeout")
	}
	if err := h.Validate(&config.Step{ObserveHTTP: &config.ObserveHTTP{URL: "https://x"}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_200OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Mooncake", "yes")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{
		URL:            srv.URL,
		CaptureHeaders: []string{"X-Mooncake"},
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true for 200; data=%v", data)
	}
	val, _ := data["value"].(map[string]any)
	if sc, _ := val["status_code"].(float64); int(sc) != 200 {
		t.Errorf("status_code = %v, want 200", val["status_code"])
	}
	headers, _ := val["headers"].(map[string]any)
	if h := headers["X-Mooncake"]; h != "yes" {
		t.Errorf("expected captured header X-Mooncake=yes; got %v", headers)
	}
	if body, _ := val["body_sample"].(string); body != "ok" {
		t.Errorf("body_sample = %q, want ok", body)
	}
}

func TestRun_ExpectStatusMismatch_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{
		URL:          srv.URL,
		ExpectStatus: 200,
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for status-mismatch")
	}
	val, _ := data["value"].(map[string]any)
	if sc, _ := val["status_code"].(float64); int(sc) != 500 {
		t.Errorf("expected typed payload to still carry actual status; got %v", val["status_code"])
	}
	if reach, _ := val["reachable"].(bool); !reach {
		t.Errorf("reachable should be true even when status mismatched")
	}
}

func TestRun_UnreachableHost(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{
		URL:     "http://127.0.0.1:1", // closed
		Timeout: "200ms",
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("unreachable: found must be false")
	}
	val, _ := data["value"].(map[string]any)
	if reach, _ := val["reachable"].(bool); reach {
		t.Errorf("unreachable: reachable must be false")
	}
	if errStr, _ := data["error"].(string); errStr == "" {
		t.Errorf("unreachable: error must be non-empty")
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{URL: "http://anywhere"}}
	ctx := newCtx(t, true)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}

func TestPermissions_Network(t *testing.T) {
	h := &Handler{}
	if !h.Permissions(&config.Step{}).Network {
		t.Errorf("observe.http must flag Network=true")
	}
}
