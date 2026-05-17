package http_request

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestRun_Plan_ProbeAndCreatesWhen_ResourceMissing — probe returns 404
// for the target resource; creates_when says "true when missing"; plan
// reports WouldChange=true ("would call POST").
func TestRun_Plan_ProbeAndCreatesWhen_ResourceMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(404)
			return
		}
		w.WriteHeader(201)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:         srv.URL + "/hooks",
		Method:      "POST",
		CreatesWhen: "probe.status_code == 404",
		Probe: &config.HTTPRequest{
			URL:    srv.URL + "/hooks/123",
			Method: "GET",
			// Probe expects 200 OR 404 (both valid signals); set
			// expect_status broadly so the probe doesn't fail with a
			// "status mismatch" inside sendOnce. Wave 1 default 2xx
			// would fail on 404; we want the 404 to flow back as a
			// fact for creates_when to evaluate.
			ExpectStatus: []int{200, 404},
		},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("WouldChange should be true (resource missing → would create); got %v, reason %q", r.WouldChange, r.Reason)
	}
	if !strings.Contains(r.Reason, "creates_when: true") {
		t.Errorf("reason should mention creates_when=true; got %q", r.Reason)
	}
}

// TestRun_Plan_ProbeAndCreatesWhen_ResourceExists — probe returns 200;
// creates_when="probe.status_code == 404" evaluates false; plan reports
// WouldChange=false ("skip — state matches").
func TestRun_Plan_ProbeAndCreatesWhen_ResourceExists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:         srv.URL + "/hooks",
		Method:      "POST",
		CreatesWhen: "probe.status_code == 404",
		Probe: &config.HTTPRequest{
			URL:          srv.URL + "/hooks/123",
			Method:       "GET",
			ExpectStatus: []int{200, 404},
		},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("WouldChange should be false (resource exists → no-op); got reason %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "creates_when: false") {
		t.Errorf("reason should mention creates_when=false; got %q", r.Reason)
	}
}

// TestRun_Plan_ProbeFailure_FallsBack — probe URL is unreachable; the
// handler logs and falls back to method-based defaults (POST →
// WouldChange=true). The plan still reports — operators see the probe
// failure and the fallback reason.
func TestRun_Plan_ProbeFailure_FallsBack(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:            "http://localhost/hooks",
		Method:         "POST",
		IdempotencyKey: "k",
		Probe: &config.HTTPRequest{
			URL:     "http://127.0.0.1:1/probe", // closed port
			Method:  "GET",
			Timeout: "200ms",
		},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("fallback WouldChange should be true for POST; got %v", r.WouldChange)
	}
	if !strings.Contains(r.Reason, "probe failed") {
		t.Errorf("reason should surface probe failure; got %q", r.Reason)
	}
}

// TestRun_Plan_CreatesWhenWithoutProbe — creates_when may be set
// without probe; it evaluates against the existing scope.
func TestRun_Plan_CreatesWhenWithoutProbe(t *testing.T) {
	ec := newCtx(t, true)
	ec.Scope.User["hook_exists"] = true

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:         "http://localhost/hooks",
		Method:      "POST",
		CreatesWhen: "hook_exists == false",
	}}
	res, err := (&Handler{}).Run(ec, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("hook_exists=true → creates_when=false → WouldChange should be false; got reason %q", r.Reason)
	}
}

// TestRun_Plan_CreatesWhenNonBool — predicate must return bool; integer
// 0/1 are rejected by the evaluator → handler surfaces error.
func TestRun_Plan_CreatesWhenNonBool(t *testing.T) {
	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:         "http://localhost/hooks",
		Method:      "POST",
		CreatesWhen: `"yes"`,
	}}
	_, err := (&Handler{}).Run(newCtx(t, true), step)
	if err == nil {
		t.Fatal("expected error on non-bool creates_when")
	}
	if !strings.Contains(err.Error(), "creates_when") {
		t.Errorf("error should mention creates_when; got %v", err)
	}
}

// TestRun_Plan_Probe_DoesNotExecuteMainRequest — when probe is set,
// the main request URL must NOT be hit during plan mode (even if the
// probe ends up evaluating to "would create"). Guards against a
// regression where the main request leaks through plan.
func TestRun_Plan_Probe_DoesNotExecuteMainRequest(t *testing.T) {
	var mainHits, probeHits atomic.Int32

	mainSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mainHits.Add(1)
		w.WriteHeader(201)
	}))
	defer mainSrv.Close()
	probeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		probeHits.Add(1)
		w.WriteHeader(404)
	}))
	defer probeSrv.Close()

	step := &config.Step{HTTPRequest: &config.HTTPRequest{
		URL:         mainSrv.URL,
		Method:      "POST",
		CreatesWhen: "probe.status_code == 404",
		Probe: &config.HTTPRequest{
			URL:          probeSrv.URL,
			Method:       "GET",
			ExpectStatus: []int{200, 404},
		},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, true), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if mainHits.Load() != 0 {
		t.Errorf("main URL was hit during plan; that's the bug we're guarding against")
	}
	if probeHits.Load() != 1 {
		t.Errorf("probe should run exactly once; got %d", probeHits.Load())
	}
}
