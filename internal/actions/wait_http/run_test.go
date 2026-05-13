package wait_http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
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
		{"missing url", &config.Step{WaitHTTP: &config.WaitHTTP{}}, true},
		{"bad status", &config.Step{WaitHTTP: &config.WaitHTTP{URL: "http://x", Status: []int{99}}}, true},
		{"ok", &config.Step{WaitHTTP: &config.WaitHTTP{URL: "http://x"}}, false},
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

// TestRun_Plan surfaces method and URL.
func TestRun_Plan(t *testing.T) {
	step := &config.Step{WaitHTTP: &config.WaitHTTP{URL: "http://localhost/health"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "http://localhost/health") {
		t.Errorf("reason should include URL; got %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "GET") {
		t.Errorf("reason should include default method GET; got %q", r.Reason)
	}
}

// TestRun_Apply_HappyPath: server returns 200 on first call → done.
func TestRun_Apply_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ready"))
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{URL: srv.URL, Timeout: "2s"}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("expected success=true; data=%v", r.Data)
	}
}

// TestRun_Apply_BodyContains: server first returns wrong body then right body.
func TestRun_Apply_BodyContains(t *testing.T) {
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		if atomic.AddInt32(&n, 1) < 2 {
			_, _ = w.Write([]byte("not yet"))
			return
		}
		_, _ = w.Write([]byte("server is ready"))
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:          srv.URL,
		BodyContains: "ready",
		Timeout:      "2s",
		PollInterval: "100ms",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["iterations"].(int) < 2 {
		t.Errorf("expected >= 2 iterations; got %v", r.Data["iterations"])
	}
}

// TestRun_Apply_Timeout: server always 503 → timeout.
func TestRun_Apply_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:          srv.URL,
		Status:       []int{200},
		Timeout:      "300ms",
		PollInterval: "100ms",
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout; got %v", err)
	}
}

// TestRun_Apply_Headers: server requires Authorization header.
func TestRun_Apply_Headers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			w.WriteHeader(401)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:     srv.URL,
		Headers: map[string]string{"Authorization": "Bearer secret"},
		Timeout: "2s",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["last_status"].(int) != 200 {
		t.Errorf("expected last_status=200; got %v", r.Data["last_status"])
	}
}
