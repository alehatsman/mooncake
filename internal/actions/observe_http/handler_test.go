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
	r := res.(*executor.Result)
	if found, _ := r.Data["found"].(bool); found {
		t.Errorf("unreachable: found must be false")
	}
	val, _ := r.Data["value"].(map[string]any)
	if reach, _ := val["reachable"].(bool); reach {
		t.Errorf("unreachable: reachable must be false")
	}
	// Proposal-06: transport failure surfaces on the envelope, not Data.
	if r.Error == "" {
		t.Errorf("unreachable: envelope Error must be non-empty")
	}
	if !r.Failed {
		t.Errorf("unreachable: envelope Failed must be true")
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

// --- Issue #18: follow_redirects opt-out --------------------------------
//
// Without follow_redirects:0 the canonical "is the 301 still in place"
// probe is impossible: Go's default client silently follows the
// redirect and the operator sees status_code=200 / found=false even
// though the redirect is working as intended.

func TestRun_FollowRedirectsDefault_FollowsTo200(t *testing.T) {
	// Two servers: target returns 200, source 301-redirects to target.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(301)
	}))
	defer src.Close()

	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{URL: src.URL}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	val := res.(*executor.Result).Data["value"].(map[string]any)
	if sc, _ := val["status_code"].(float64); int(sc) != 200 {
		t.Errorf("default behavior: should follow redirect to 200, got %v", val["status_code"])
	}
}

func TestRun_FollowRedirectsZero_SurfacesRedirectStatus(t *testing.T) {
	// Issue #18 headline assertion: follow_redirects:0 + expect_status:301
	// is the canonical "is the redirect still in place" probe.
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer target.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(301)
	}))
	defer src.Close()

	zero := 0
	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{
		URL:             src.URL,
		FollowRedirects: &zero,
		ExpectStatus:    301,
		CaptureHeaders:  []string{"Location"},
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expect_status:301 should match the unfollowed 301; data=%v", data)
	}
	val := data["value"].(map[string]any)
	if sc, _ := val["status_code"].(float64); int(sc) != 301 {
		t.Errorf("status_code should be the un-followed 301, got %v", val["status_code"])
	}
	headers, _ := val["headers"].(map[string]any)
	if loc, _ := headers["Location"].(string); loc != target.URL {
		t.Errorf("expected captured Location header pointing at target, got %v", headers)
	}
}

func TestRun_FollowRedirectsBoundCapsChainLength(t *testing.T) {
	// Build a chain of 5 redirects then a 200 at the end. follow_redirects:2
	// should stop after the second redirect and surface that redirect's
	// status (still a 3xx), not the eventual 200.
	final := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer final.Close()
	var hop3, hop2, hop1 *httptest.Server
	hop3 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", final.URL)
		w.WriteHeader(301)
	}))
	defer hop3.Close()
	hop2 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", hop3.URL)
		w.WriteHeader(301)
	}))
	defer hop2.Close()
	hop1 = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", hop2.URL)
		w.WriteHeader(301)
	}))
	defer hop1.Close()

	max := 2
	h := &Handler{}
	step := &config.Step{ObserveHTTP: &config.ObserveHTTP{
		URL:             hop1.URL,
		FollowRedirects: &max,
	}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	val := res.(*executor.Result).Data["value"].(map[string]any)
	if sc, _ := val["status_code"].(float64); int(sc) < 300 || int(sc) >= 400 {
		t.Errorf("follow_redirects:2 should stop at a 3xx hop, got status_code=%v", val["status_code"])
	}
}
