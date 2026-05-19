package http_request

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

func ctxWithPublisher(t *testing.T) (*executor.ExecutionContext, *testutil.MockPublisher) {
	t.Helper()
	ec := newCtx(t, false)
	pub := &testutil.MockPublisher{}
	ec.Svc.EventPublisher = pub
	return ec, pub
}

// TestRun_Apply_GET_HappyPath — basic GET captures status_code, body,
// auto-parses JSON, marks success=true, Changed=false (read-only).
func TestRun_Apply_GET_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"ok":true,"id":42}`))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{URL: srv.URL, Timeout: "2s"}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Failed {
		t.Errorf("not Failed; got %v", r)
	}
	if r.Changed {
		t.Error("GET must not set Changed=true (read-only)")
	}
	if r.Data["status_code"].(int) != 200 {
		t.Errorf("status_code = %v", r.Data["status_code"])
	}
	if r.Data["success"] != true {
		t.Errorf("success != true: %v", r.Data["success"])
	}
	if got, _ := r.Data["body"].(string); !strings.Contains(got, `"id":42`) {
		t.Errorf("body = %q", got)
	}
	js, ok := r.Data["json"].(map[string]interface{})
	if !ok {
		t.Fatalf("json fact not parsed; data=%v", r.Data)
	}
	// Wave 2 switched the JSON decoder to UseNumber() so integer IDs
	// template as "42" (not "42.000000"). Compare as json.Number's
	// string form for stability across response shapes.
	if js["ok"] != true || js["id"].(json.Number).String() != "42" {
		t.Errorf("json parsed wrong: %v", js)
	}
}

// TestRun_Apply_POST_JSONBody — `json:` shape auto-marshals + sets
// Content-Type. Server sees a valid JSON payload.
func TestRun_Apply_POST_JSONBody(t *testing.T) {
	var seenCT, seenBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCT.Store(r.Header.Get("Content-Type"))
		b, _ := io.ReadAll(r.Body)
		seenBody.Store(string(b))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Method:         "POST",
		JSON:           map[string]interface{}{"event": "deploy", "target": "host-1"},
		IdempotencyKey: "k",
		Timeout:        "2s",
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ct, _ := seenCT.Load().(string); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if b, _ := seenBody.Load().(string); !strings.Contains(b, `"event":"deploy"`) {
		t.Errorf("body = %q; want JSON-marshalled", b)
	}
}

// TestRun_Apply_POST_FormBody — `form:` → urlencoded + correct CT.
func TestRun_Apply_POST_FormBody(t *testing.T) {
	var seenCT, seenBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenCT.Store(r.Header.Get("Content-Type"))
		b, _ := io.ReadAll(r.Body)
		seenBody.Store(string(b))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Method:         "POST",
		Form:           map[string]string{"name": "alice", "city": "santa cruz"},
		IdempotencyKey: "k",
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if ct, _ := seenCT.Load().(string); ct != "application/x-www-form-urlencoded" {
		t.Errorf("Content-Type = %q", ct)
	}
	body, _ := seenBody.Load().(string)
	vals, _ := url.ParseQuery(body)
	if vals.Get("name") != "alice" || vals.Get("city") != "santa cruz" {
		t.Errorf("form-decoded = %v", vals)
	}
}

// TestRun_Apply_POST_FileBody — `file:` sends raw bytes verbatim.
func TestRun_Apply_POST_FileBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.bin")
	want := []byte("hello\x00\x01raw")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		seen.Store(b)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Method:         "POST",
		File:           path,
		IdempotencyKey: "k",
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := seen.Load().([]byte); string(got) != string(want) {
		t.Errorf("server saw %q; want %q", got, want)
	}
}

// TestRun_Apply_Auth_Bearer — bearer adds Authorization header.
func TestRun_Apply_Auth_Bearer(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("Authorization"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:  srv.URL,
		Auth: &config.HTTPAuth{Bearer: "s3cr3t"},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := seen.Load().(string); got != "Bearer s3cr3t" {
		t.Errorf("Authorization = %q", got)
	}
}

// TestRun_Apply_Auth_Basic — basic encodes user:pass as base64.
func TestRun_Apply_Auth_Basic(t *testing.T) {
	var seenAuth atomic.Value
	var seenUser, seenPass atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth.Store(r.Header.Get("Authorization"))
		u, p, _ := r.BasicAuth()
		seenUser.Store(u)
		seenPass.Store(p)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:  srv.URL,
		Auth: &config.HTTPAuth{Basic: &config.HTTPBasicAuth{User: "alice", Pass: "p4ss"}},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if u, _ := seenUser.Load().(string); u != "alice" {
		t.Errorf("user = %q", u)
	}
	if p, _ := seenPass.Load().(string); p != "p4ss" {
		t.Errorf("pass = %q", p)
	}
	if ah, _ := seenAuth.Load().(string); !strings.HasPrefix(ah, "Basic ") {
		t.Errorf("Authorization = %q; want Basic prefix", ah)
	}
}

// TestRun_Apply_Auth_Header — arbitrary header name+value.
func TestRun_Apply_Auth_Header(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("X-Api-Key"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL: srv.URL,
		Auth: &config.HTTPAuth{
			Header: &config.HTTPAuthHeader{Name: "X-Api-Key", Value: "abc"},
		},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := seen.Load().(string); got != "abc" {
		t.Errorf("X-Api-Key = %q", got)
	}
}

// TestRun_Apply_RedactBody_ReplacesRequestResponse — when redact_body
// is set, the body in result.Data is "<redacted>" (server still gets
// the real bytes; redaction is for logs/diffs/facts only).
func TestRun_Apply_RedactBody_ReplacesResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"secret":"swordfish"}`))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:        srv.URL,
		RedactBody: true,
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["body"] != "<redacted>" {
		t.Errorf("body should be redacted; got %v", r.Data["body"])
	}
	if _, ok := r.Data["json"]; ok {
		t.Error("json fact should not appear when body is redacted")
	}
}

// TestRun_Apply_SensitiveResponseHeader_Redacted — Set-Cookie in the
// response gets redacted in the headers fact.
func TestRun_Apply_SensitiveResponseHeader_Redacted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Set-Cookie", "sid=this-is-the-session-id; Path=/")
		w.Header().Set("X-Server", "nginx")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{URL: srv.URL}}
	res, _ := (&Handler{}).Run(newCtx(t, false), step)
	r := res.(*executor.Result)
	headers := r.Data["headers"].(map[string]string)
	// Set-Cookie should be redacted (any casing). The Go http library
	// preserves canonical case Set-Cookie.
	found := false
	for k, v := range headers {
		if strings.EqualFold(k, "set-cookie") {
			found = true
			if v != "<redacted>" {
				t.Errorf("Set-Cookie should be redacted; got %q", v)
			}
		}
	}
	if !found {
		t.Errorf("Set-Cookie not in headers: %v", headers)
	}
	if headers["X-Server"] != "nginx" {
		t.Errorf("non-sensitive header should NOT be redacted: %v", headers)
	}
}

// TestRun_Apply_ExpectStatus_Failure — response code outside the
// expect_status set ⇒ step fails.
func TestRun_Apply_ExpectStatus_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:          srv.URL,
		ExpectStatus: []int{200, 201},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error on status mismatch")
	}
	r := res.(*executor.Result)
	if !r.Failed {
		t.Error("Result.Failed should be true")
	}
	if r.Data["status_code"].(int) != 404 {
		t.Errorf("status_code = %v", r.Data["status_code"])
	}
}

// TestRun_Apply_ExpectStatus_DefaultIs2xx — without expect_status, any
// 2xx passes; 3xx/4xx/5xx fail.
func TestRun_Apply_ExpectStatus_DefaultIs2xx(t *testing.T) {
	for _, code := range []int{200, 201, 204, 299} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()
			_, err := (&Handler{}).Run(newCtx(t, false), &config.Step{
				HTTPRequest: &config.HTTPRequest{URL: srv.URL},
			})
			if err != nil {
				t.Errorf("%d should pass default 2xx; got %v", code, err)
			}
		})
	}
	for _, code := range []int{301, 400, 500} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()
			// FollowRedirects=0 means we see the 301 directly (no follow).
			zero := 0
			step := &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:             srv.URL,
				FollowRedirects: &zero,
			}}
			_, err := (&Handler{}).Run(newCtx(t, false), step)
			if err == nil {
				t.Errorf("%d should fail default 2xx; nil err", code)
			}
		})
	}
}

// Multi-attempt retry behavior is owned by the executor's runWithRetry;
// the in-handler retry loop was deleted along with HTTPRequest.Retries
// and HTTPRequest.RetryDelay. The four end-to-end retry tests
// (TestRun_Apply_Retries_5xx / _Exhausted / _429 / _Timeout) used to
// live here. Their generic loop invariants are covered by
// internal/executor/retry_test.go (TestRunWithRetry_HonorsMaxAttempts /
// _WrapsAfterNAttempts / _BackoffHonored). The HTTP-specific
// classifier semantics live in TestIsRetryable below.

// TestIsRetryable_TransportErrors — a transport-level failure with no
// response is classified via the timeout/connection_error tokens.
func TestIsRetryable_TransportErrors(t *testing.T) {
	h := &Handler{}
	stepWith := func(tokens ...string) *config.Step {
		return &config.Step{HTTPRequest: &config.HTTPRequest{RetryOn: tokens}}
	}
	noResult := func() actions.Result {
		r := executor.NewResult()
		r.Data = map[string]interface{}{"status_code": 0, "success": false}
		return r
	}
	timeoutErr := context.DeadlineExceeded

	if !h.IsRetryable(noResult(), timeoutErr, stepWith("timeout")) {
		t.Error("timeout in retry_on should retry")
	}
	if h.IsRetryable(noResult(), timeoutErr, stepWith("5xx")) {
		t.Error("timeout NOT in retry_on should not retry")
	}
	connErr := errors.New("connection refused")
	if !h.IsRetryable(noResult(), connErr, stepWith("connection_error")) {
		t.Error("connection_error in retry_on should retry")
	}
	if h.IsRetryable(noResult(), connErr, stepWith("timeout")) {
		t.Error("connection_error NOT in retry_on should not retry")
	}
}

// TestIsRetryable_StatusClassification — response-level failures are
// classified by status code against the 5xx/4xx/429 tokens.
func TestIsRetryable_StatusClassification(t *testing.T) {
	h := &Handler{}
	stepWith := func(tokens ...string) *config.Step {
		return &config.Step{HTTPRequest: &config.HTTPRequest{RetryOn: tokens}}
	}
	withStatus := func(code int) actions.Result {
		r := executor.NewResult()
		r.Data = map[string]interface{}{"status_code": code, "success": false}
		return r
	}
	statusErr := errors.New("returned 503")

	cases := []struct {
		name   string
		status int
		tokens []string
		want   bool
	}{
		{"503 + 5xx", 503, []string{"5xx"}, true},
		{"404 + 5xx", 404, []string{"5xx"}, false},
		{"429 + 429", 429, []string{"429"}, true},
		{"429 + 5xx (no 429 token)", 429, []string{"5xx"}, false},
		{"404 + 4xx", 404, []string{"4xx"}, true},
		{"200 + 5xx (success status — caller passes nil err)", 200, []string{"5xx"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := h.IsRetryable(withStatus(tc.status), statusErr, stepWith(tc.tokens...))
			if got != tc.want {
				t.Errorf("IsRetryable status=%d tokens=%v = %v, want %v",
					tc.status, tc.tokens, got, tc.want)
			}
		})
	}
}

// TestIsRetryable_EmptyRetryOn — no retry_on tokens means no retry at
// the handler's discretion. The executor's default behavior (retry on
// any err) does NOT apply here because http_request implements
// Retryable explicitly.
func TestIsRetryable_EmptyRetryOn(t *testing.T) {
	h := &Handler{}
	step := &config.Step{HTTPRequest: &config.HTTPRequest{}}
	r := executor.NewResult()
	r.Data = map[string]interface{}{"status_code": 503}
	if h.IsRetryable(r, errors.New("503"), step) {
		t.Error("empty retry_on should not retry")
	}
}

// TestRun_Apply_Templating_URLBodyHeaders — variables in scope are
// substituted into URL, body, and headers.
func TestRun_Apply_Templating_URLBodyHeaders(t *testing.T) {
	var seenPath, seenBody, seenAuth atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath.Store(r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		seenBody.Store(string(b))
		seenAuth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ec, _ := ctxWithPublisher(t)
	ec.Scope.User["id"] = "42"
	ec.Scope.User["token"] = "tok-abc"

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL + "/users/{{ id }}",
		Method:         "POST",
		Body:           `{"id":"{{ id }}"}`,
		Headers:        map[string]string{"X-Trace": "trace-{{ id }}"},
		Auth:           &config.HTTPAuth{Bearer: "{{ token }}"},
		IdempotencyKey: "k-{{ id }}",
	}}
	if _, err := (&Handler{}).Run(ec, step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if p, _ := seenPath.Load().(string); p != "/users/42" {
		t.Errorf("path = %q; want /users/42", p)
	}
	if b, _ := seenBody.Load().(string); b != `{"id":"42"}` {
		t.Errorf("body = %q", b)
	}
	if a, _ := seenAuth.Load().(string); a != "Bearer tok-abc" {
		t.Errorf("Authorization = %q", a)
	}
}

// TestRun_Apply_IdempotencyKey_SentAsHeader — the value (rendered)
// arrives as the standard `Idempotency-Key` header.
func TestRun_Apply_IdempotencyKey_SentAsHeader(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.Store(r.Header.Get("Idempotency-Key"))
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Method:         "POST",
		Body:           "{}",
		IdempotencyKey: "deploy-2026-05-17",
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, _ := seen.Load().(string); got != "deploy-2026-05-17" {
		t.Errorf("Idempotency-Key = %q", got)
	}
}

// TestRun_Apply_MaxResponseBytes_Truncates — body larger than cap is
// truncated; `truncated:true` exposed in facts.
func TestRun_Apply_MaxResponseBytes_Truncates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(strings.Repeat("x", 1024)))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:              srv.URL,
		MaxResponseBytes: 16,
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["truncated"] != true {
		t.Error("truncated should be true")
	}
	body, _ := r.Data["body"].(string)
	if len(body) > 16 {
		t.Errorf("body len = %d; want <= 16", len(body))
	}
}

// TestRun_Apply_EmitsEvent_HostOnly — emitted event strips the query
// string so secrets in URLs don't leak into the event stream.
func TestRun_Apply_EmitsEvent_HostOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer srv.Close()

	ec, pub := ctxWithPublisher(t)
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL: srv.URL + "/path?token=secret&id=1",
	}}
	if _, err := (&Handler{}).Run(ec, step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(pub.Events) != 1 {
		t.Fatalf("want 1 event; got %d", len(pub.Events))
	}
	ev := pub.Events[0]
	if ev.Type != events.EventHTTPRequested {
		t.Errorf("event type = %v", ev.Type)
	}
	data := ev.Data.(events.HTTPRequestedData)
	if strings.Contains(data.URL, "token=secret") || strings.Contains(data.URL, "?") {
		t.Errorf("event URL leaked query string: %q", data.URL)
	}
	if !strings.Contains(data.URL, "/path") {
		t.Errorf("event URL should retain path: %q", data.URL)
	}
	if data.StatusCode != 204 {
		t.Errorf("status_code = %d", data.StatusCode)
	}
	if data.Attempts != 1 {
		t.Errorf("attempts = %d", data.Attempts)
	}
}

// TestRun_Apply_NetworkError_PopulatesFact — when the request can't
// even reach the server (closed port), the step fails but still
// records attempts and emits an event with status=0.
func TestRun_Apply_NetworkError_PopulatesFact(t *testing.T) {
	// localhost:1 is reliably-closed across CI environments.
	ec, pub := ctxWithPublisher(t)
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:     "http://127.0.0.1:1/",
		Timeout: "300ms",
	}}
	res, err := (&Handler{}).Run(ec, step)
	if err == nil {
		t.Fatal("expected network error")
	}
	r := res.(*executor.Result)
	if !r.Failed {
		t.Error("Result.Failed should be true")
	}
	if r.Data["status_code"].(int) != 0 {
		t.Errorf("status_code = %v; want 0 on transport failure", r.Data["status_code"])
	}
	if len(pub.Events) != 1 {
		t.Fatalf("expected one event on network failure; got %d", len(pub.Events))
	}
}

// TestRun_Apply_WriteMethod_SetsChanged — write methods (POST/PUT/etc)
// report Changed=true even though we can't know the exact server-side
// effect; read methods do not.
func TestRun_Apply_WriteMethod_SetsChanged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	cases := []struct {
		method      string
		idemKey     string
		wantChanged bool
	}{
		{"GET", "", false},
		{"HEAD", "", false},
		{"OPTIONS", "", false},
		{"POST", "k", true},
		{"PUT", "", true},
		{"PATCH", "k", true},
		{"DELETE", "", true},
	}
	for _, c := range cases {
		t.Run(c.method, func(t *testing.T) {
			step := &config.Step{HTTPRequest: &config.HTTPRequest{
				URL:            srv.URL,
				Method:         c.method,
				IdempotencyKey: c.idemKey,
			}}
			res, err := (&Handler{}).Run(newCtx(t, false), step)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			r := res.(*executor.Result)
			if r.Changed != c.wantChanged {
				t.Errorf("Changed = %v, want %v", r.Changed, c.wantChanged)
			}
		})
	}
}

// TestRun_Apply_SaveTo_WritesBody — Wave 3 save_to persists the
// response bytes to the configured path. Parent dir is created.
// The path is template-rendered so callers can pin it under a
// per-run state dir.
func TestRun_Apply_SaveTo_WritesBody(t *testing.T) {
	const wantBody = `{"ok":true,"id":42}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(wantBody))
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Use a nested path so the mkdir -p branch fires.
	target := filepath.Join(dir, "nested", "hook.json")

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:     srv.URL,
		Timeout: "2s",
		SaveTo:  target,
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Failed {
		t.Fatalf("must not fail; got %+v", r)
	}
	if r.Data["saved_to"] != target {
		t.Errorf("Data[saved_to] = %v, want %s", r.Data["saved_to"], target)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read save_to target: %v", err)
	}
	if string(got) != wantBody {
		t.Errorf("file content = %q, want %q", got, wantBody)
	}
}

// TestRun_Apply_SaveTo_TemplateInterpolation — the rendered path
// may interpolate response facts (registered name + .json.id is
// the canonical "post a thing, store its server-assigned id"
// shape). Verifies render hook plumbed through writeResponseBody.
func TestRun_Apply_SaveTo_TemplateInterpolation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc-123"}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:     srv.URL,
		Timeout: "2s",
		SaveTo:  filepath.Join(dir, "{{ response.json.id }}.json"),
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	want := filepath.Join(dir, "abc-123.json")
	if r.Data["saved_to"] != want {
		t.Errorf("saved_to = %v, want %s", r.Data["saved_to"], want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Errorf("expected rendered path to exist: %v", err)
	}
}

// TestRun_Apply_SaveTo_WriteFailureFailsStep — if save_to points
// at an unwritable location (here: a path under a non-directory),
// the step fails rather than silently dropping the file. The user
// asked for it; losing it is surprising.
func TestRun_Apply_SaveTo_WriteFailureFailsStep(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	// Create a regular file, then try to save under it as if it were
	// a directory. mkdir -p will fail because the parent is a file.
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatalf("setup blocker: %v", err)
	}

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:     srv.URL,
		Timeout: "2s",
		SaveTo:  filepath.Join(blocker, "out.txt"), // parent is a file
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatalf("expected error for unwritable save_to; got nil (res=%+v)", res)
	}
	if r, ok := res.(*executor.Result); ok && !r.Failed {
		t.Errorf("Result.Failed should be true on save_to error; got %+v", r)
	}
	if !strings.Contains(err.Error(), "save_to") {
		t.Errorf("error should mention save_to; got: %v", err)
	}
}

// TestRun_Apply_ExpectJSONKeys_AllPresent — happy path: every
// declared key exists on the parsed JSON object, step succeeds.
func TestRun_Apply_ExpectJSONKeys_AllPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"abc","url":"https://x","secret":"s"}`))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Timeout:        "2s",
		ExpectJSONKeys: []string{"id", "url"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Failed {
		t.Errorf("expected success; got failed result: %+v", r)
	}
}

// TestRun_Apply_ExpectJSONKeys_MissingKeyFails — one or more keys
// missing fails the step with a deterministic, sorted error message.
func TestRun_Apply_ExpectJSONKeys_MissingKeyFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Returns id but NOT url + secret.
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Timeout:        "2s",
		ExpectJSONKeys: []string{"id", "secret", "url"},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for missing keys")
	}
	if !strings.Contains(err.Error(), "expect_json_keys") {
		t.Errorf("error should mention expect_json_keys; got: %v", err)
	}
	// Missing keys reported in sorted order so diffs stay stable.
	if !strings.Contains(err.Error(), "secret") || !strings.Contains(err.Error(), "url") {
		t.Errorf("error should list missing keys [secret url]; got: %v", err)
	}
	if strings.Index(err.Error(), "secret") > strings.Index(err.Error(), "url") {
		t.Errorf("missing keys must be sorted (secret before url); got: %v", err)
	}
}

// TestRun_Apply_ExpectJSONKeys_NonJSONResponseFails — content-type
// isn't JSON → data["json"] is nil → explicit error rather than a
// confusing "missing key X" against a string body.
func TestRun_Apply_ExpectJSONKeys_NonJSONResponseFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text body"))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Timeout:        "2s",
		ExpectJSONKeys: []string{"id"},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-JSON response")
	}
	if !strings.Contains(err.Error(), "not JSON") {
		t.Errorf("error should explain non-JSON cause; got: %v", err)
	}
}

// TestRun_Apply_ExpectJSONKeys_NonObjectFails — JSON parses but is
// an array, not an object. The narrow check only covers objects;
// arrays/scalars fail with a clear "want object" message.
func TestRun_Apply_ExpectJSONKeys_NonObjectFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`["a","b","c"]`))
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Timeout:        "2s",
		ExpectJSONKeys: []string{"id"},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-object JSON")
	}
	if !strings.Contains(err.Error(), "want object") {
		t.Errorf("error should mention 'want object'; got: %v", err)
	}
}

// TestValidate_ExpectJSONKeys_RejectsEmptyEntry guards against
// `expect_json_keys: [""]` silently no-op'ing — easy operator typo
// that would otherwise pass Validate and accept any response.
func TestValidate_ExpectJSONKeys_RejectsEmptyEntry(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://x",
		ExpectJSONKeys: []string{"id", "   ", "url"},
	}}
	err := (&Handler{}).Validate(step)
	if err == nil {
		t.Fatal("expected Validate to reject whitespace-only key")
	}
	if !strings.Contains(err.Error(), "expect_json_keys[1]") {
		t.Errorf("error should pinpoint the bad index; got: %v", err)
	}
}
