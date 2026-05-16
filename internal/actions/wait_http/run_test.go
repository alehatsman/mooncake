package wait_http

import (
	"io"
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

// TestRun_Apply_POSTWithBody — proposal-10: POST with a JSON body
// delivers the body to the server every poll, the same way curl -d
// would. The server here checks both the method and the body shape
// before answering 200, so a misrouted body or a method mix-up would
// flip the result to 'timeout'.
func TestRun_Apply_POSTWithBody(t *testing.T) {
	var lastBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		buf, _ := io.ReadAll(r.Body)
		lastBody.Store(string(buf))
		if !strings.Contains(string(buf), `"input":"ping"`) {
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:     srv.URL,
		Method:  "POST",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    `{"input":"ping","model":"qwen3-embedding"}`,
		Timeout: "2s",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("success != true; data=%v", r.Data)
	}
	if r.Data["method"] != "POST" {
		t.Errorf("method = %v, want POST", r.Data["method"])
	}
	got, _ := lastBody.Load().(string)
	if !strings.Contains(got, `"model":"qwen3-embedding"`) {
		t.Errorf("server saw body %q; want full JSON payload", got)
	}
}

// TestRun_Apply_BodyTemplateRendered — body goes through the same
// pongo2 path as URL/headers, so `{{ var }}` is substituted from the
// scope before each poll.
func TestRun_Apply_BodyTemplateRendered(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		seen.Store(string(buf))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ec := newCtx(t, false)
	ec.Scope.User["model_name"] = "qwen3-embedding"

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:     srv.URL,
		Method:  "POST",
		Body:    `{"model":"{{ model_name }}"}`,
		Timeout: "2s",
	}}
	if _, err := (&Handler{}).Run(ec, step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := seen.Load().(string)
	if got != `{"model":"qwen3-embedding"}` {
		t.Errorf("server saw body %q; want template-rendered payload", got)
	}
}

// TestRun_Apply_GETNoBody — empty Body keeps the request body
// http.NoBody so GET requests behave exactly as they did pre-proposal-10
// (Content-Length is unset, server sees no body). Guards against a
// regression where strings.NewReader("") leaks into the path.
func TestRun_Apply_GETNoBody(t *testing.T) {
	var sawContentLength atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > 0 {
			sawContentLength.Store(true)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:     srv.URL,
		Timeout: "2s",
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if sawContentLength.Load() {
		t.Error("server saw Content-Length on GET; want http.NoBody (no body, no Content-Length)")
	}
}

// TestRun_Plan_BodyHintInReason — plan mode includes a "body=N bytes"
// hint when a body is present, so `mooncake plan` output makes the
// difference between a GET and a POST-with-body request explicit.
func TestRun_Plan_BodyHintInReason(t *testing.T) {
	step := &config.Step{WaitHTTP: &config.WaitHTTP{
		URL:    "http://localhost/embed",
		Method: "POST",
		Body:   `{"x":1}`,
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "body=7 bytes") {
		t.Errorf("plan reason missing body-size hint; got %q", r.Reason)
	}
}
