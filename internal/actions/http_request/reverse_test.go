package http_request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestReverse_NoBlockDeclared — action without reverse: block is
// irreversible. Reverse() must return an explicit error so transaction
// rollback fails loudly rather than skipping the compensation silently.
func TestReverse_NoBlockDeclared(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://x",
		Method:         "POST",
		IdempotencyKey: "k",
	}}
	res := executor.NewResult()
	res.Data = map[string]interface{}{"status_code": 201}

	out, err := (&Handler{}).Reverse(nil, step, res)
	if err == nil {
		t.Fatal("expected error when no reverse: block declared")
	}
	if out != nil {
		t.Errorf("step should be nil; got %+v", out)
	}
	if !strings.Contains(err.Error(), "not reversible") {
		t.Errorf("error should mention not-reversible; got %v", err)
	}
}

// TestReverse_ReverseDataMissing — reverse: block declared, but the
// apply never produced a snapshot (e.g. it failed pre-snapshot).
// Reverse() refuses rather than fabricating a rollback.
func TestReverse_ReverseDataMissing(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://x",
		Method:         "POST",
		IdempotencyKey: "k",
		Reverse:        &config.HTTPRequest{URL: "http://x/1", Method: "DELETE"},
	}}
	res := executor.NewResult() // ReverseData nil
	_, err := (&Handler{}).Reverse(nil, step, res)
	if err == nil {
		t.Fatal("expected error when ReverseData missing")
	}
	if !strings.Contains(err.Error(), "snapshot missing") {
		t.Errorf("error should mention snapshot; got %v", err)
	}
}

// TestReverse_FullCycle_PostThenDelete — POST creates a webhook and
// returns {"id":42}; the reverse block templates `{{ hook.json.id }}`
// into the DELETE URL; runApply snapshots that *after* substitution;
// Reverse() returns a Step that DELETEs /hooks/42; we run that Step
// against the same server and confirm it hits /hooks/42 with DELETE.
func TestReverse_FullCycle_PostThenDelete(t *testing.T) {
	var sawDelete atomic.Value

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":42,"event":"deploy"}`))
		case http.MethodDelete:
			sawDelete.Store(r.URL.Path)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	defer srv.Close()

	step := &config.Step{
		Name: "register webhook",
		As:   "hook",
		HTTPRequest: &config.HTTPRequest{
			URL:            srv.URL + "/hooks",
			Method:         "POST",
			IdempotencyKey: "k-test",
			Reverse: &config.HTTPRequest{
				URL:    srv.URL + "/hooks/{{ hook.json.id }}",
				Method: "DELETE",
			},
		},
	}

	ec := newCtx(t, false)
	res, err := (&Handler{}).Run(ec, step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	if r.ReverseData == nil {
		t.Fatal("ReverseData should be populated after successful apply with reverse: declared")
	}
	rendered, ok := r.ReverseData.(*config.HTTPRequest)
	if !ok {
		t.Fatalf("ReverseData type = %T; want *config.HTTPRequest", r.ReverseData)
	}
	wantURL := srv.URL + "/hooks/42"
	if rendered.URL != wantURL {
		t.Errorf("reverse URL = %q; want %q (response.id should interpolate)", rendered.URL, wantURL)
	}

	// Reverse() returns the Step to apply for rollback.
	revStep, err := (&Handler{}).Reverse(ec, step, r)
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if revStep == nil || revStep.HTTPRequest == nil {
		t.Fatal("Reverse should return a Step with HTTPRequest")
	}
	if revStep.HTTPRequest.URL != wantURL {
		t.Errorf("reverse step URL = %q; want %q", revStep.HTTPRequest.URL, wantURL)
	}
	if revStep.HTTPRequest.Method != "DELETE" {
		t.Errorf("reverse method = %q; want DELETE", revStep.HTTPRequest.Method)
	}

	// Apply the reverse step and confirm the server saw DELETE /hooks/42.
	if _, err := (&Handler{}).Run(ec, revStep); err != nil {
		t.Fatalf("reverse apply: %v", err)
	}
	got, _ := sawDelete.Load().(string)
	if got != "/hooks/42" {
		t.Errorf("server saw DELETE %q; want /hooks/42", got)
	}
}

// TestReverse_ResponseAvailableUnderResponseKey — when `as:` is unset,
// the reverse block can still reference the response under the stable
// name `response`.
func TestReverse_ResponseAvailableUnderResponseKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(201)
			_, _ = w.Write([]byte(`{"id":7}`))
			return
		}
		buf, _ := io.ReadAll(r.Body)
		_ = buf
		w.WriteHeader(204)
	}))
	defer srv.Close()

	step := &config.Step{
		HTTPRequest: &config.HTTPRequest{
			URL:            srv.URL + "/items",
			Method:         "POST",
			IdempotencyKey: "k",
			Reverse: &config.HTTPRequest{
				URL:    srv.URL + "/items/{{ response.json.id }}",
				Method: "DELETE",
			},
		},
	}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	r := res.(*executor.Result)
	rendered := r.ReverseData.(*config.HTTPRequest)
	want := srv.URL + "/items/7"
	if rendered.URL != want {
		t.Errorf("reverse URL = %q; want %q", rendered.URL, want)
	}
}

// TestReverse_NotPopulatedOnFailedApply — when the apply returns an
// error (status mismatch, network error), ReverseData stays nil; a
// subsequent Reverse() must refuse rather than send a half-baked
// compensation.
func TestReverse_NotPopulatedOnFailedApply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            srv.URL,
		Method:         "POST",
		IdempotencyKey: "k",
		Reverse:        &config.HTTPRequest{URL: srv.URL + "/1", Method: "DELETE"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected apply to fail on 500")
	}
	r := res.(*executor.Result)
	if r.ReverseData != nil {
		t.Errorf("ReverseData should NOT be populated on failed apply; got %v", r.ReverseData)
	}
	_, revErr := (&Handler{}).Reverse(nil, step, r)
	if revErr == nil {
		t.Error("Reverse should refuse when ReverseData is missing")
	}
}
