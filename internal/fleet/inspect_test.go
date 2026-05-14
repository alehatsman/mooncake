package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeAgentd builds an httptest.Server that answers the three endpoints
// `fleet status` probes, with the responses configurable per-test via the
// returned options struct.
type fakeAgentd struct {
	Version map[string]any   // body for GET /v1/version; nil → 500
	Runs    []map[string]any // bodies for GET /v1/runs; nil → 500
	Facts   map[string]any   // body for GET /v1/facts; nil → 500

	// Per-endpoint forced error. When set, the matching handler returns
	// 500 regardless of the value above. Useful for testing partial
	// failure (e.g. version succeeds but facts errors).
	FailVersion bool
	FailRuns    bool
	FailFacts   bool

	// ExpectToken matches the Authorization header. Empty disables
	// the check.
	ExpectToken string
}

func (f *fakeAgentd) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		if f.FailVersion || f.Version == nil {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.Version)
	})
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		if f.FailRuns {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": f.Runs})
	})
	mux.HandleFunc("/v1/facts", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		if f.FailFacts || f.Facts == nil {
			http.Error(w, "boom", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.Facts)
	})
	return mux
}

func (f *fakeAgentd) checkAuth(w http.ResponseWriter, r *http.Request) (denied bool) {
	if f.ExpectToken == "" {
		return false
	}
	got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if got != f.ExpectToken {
		http.Error(w, "unauthorized", 401)
		return true
	}
	return false
}

func (f *fakeAgentd) start(t *testing.T) (addr, token string) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	// strip the leading "http://" — the controller-side Client adds it.
	return strings.TrimPrefix(srv.URL, "http://"), f.ExpectToken
}

func defaultFakeOK() *fakeAgentd {
	return &fakeAgentd{
		ExpectToken: "test-tok",
		Version: map[string]any{
			"version":      "0.9.0",
			"hostname":     "h",
			"synced_root":  "/tmp/state/synced",
			"queue_depth":  0,
			"runs_running": 0,
		},
		Runs: []map[string]any{{
			"id":          "01H00000000000000000000000",
			"status":      "success",
			"plan_path":   "/tmp/plan.yml",
			"queued_at":   time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano),
			"started_at":  time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano),
			"finished_at": time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano),
		}},
		Facts: map[string]any{
			"os":         "ubuntu",
			"os_version": "24.04",
			"arch":       "amd64",
		},
	}
}

// TestProbe_HappyPath asserts the most common shape: peer reachable, last
// run succeeded, OS reported. Everything populated, State == ok.
func TestProbe_HappyPath(t *testing.T) {
	f := defaultFakeOK()
	addr, tok := f.start(t)

	s := Probe(context.Background(), "peer", addr, tok, time.Second)

	if s.State != StateOK {
		t.Errorf("State = %q, want ok", s.State)
	}
	if s.OS != "ubuntu 24.04" {
		t.Errorf("OS = %q, want 'ubuntu 24.04'", s.OS)
	}
	if s.Arch != "amd64" {
		t.Errorf("Arch = %q, want amd64", s.Arch)
	}
	if s.Mooncake != "0.9.0" {
		t.Errorf("Mooncake = %q", s.Mooncake)
	}
	if s.QueueDepth != 0 {
		t.Errorf("QueueDepth = %d, want 0", s.QueueDepth)
	}
	if s.LastRunStatus != "success" {
		t.Errorf("LastRunStatus = %q", s.LastRunStatus)
	}
	if !strings.HasSuffix(s.LastRunAge, " ago") {
		t.Errorf("LastRunAge = %q, want suffix ' ago'", s.LastRunAge)
	}
	if s.Error != "" {
		t.Errorf("Error = %q, want empty", s.Error)
	}
}

// TestProbe_UnreachableWhenVersionFails locks in the gating-call decision:
// if /v1/version errors, the whole peer is unreachable. Failures of the
// other two GETs don't gate.
func TestProbe_UnreachableWhenVersionFails(t *testing.T) {
	f := defaultFakeOK()
	f.FailVersion = true
	addr, tok := f.start(t)

	s := Probe(context.Background(), "peer", addr, tok, time.Second)

	if s.State != StateUnreachable {
		t.Errorf("State = %q, want unreachable", s.State)
	}
	if s.Error == "" {
		t.Error("Error empty; want the underlying version-probe error")
	}
}

// TestProbe_PartialFailureDoesNotFlipState — when only facts or only runs
// errors, State stays ok (or running/failed based on the other signals).
// The affected columns are blank, NOT the whole row.
func TestProbe_PartialFailureDoesNotFlipState(t *testing.T) {
	t.Run("facts fail", func(t *testing.T) {
		f := defaultFakeOK()
		f.FailFacts = true
		addr, tok := f.start(t)
		s := Probe(context.Background(), "peer", addr, tok, time.Second)
		if s.State != StateOK {
			t.Errorf("State = %q, want ok (facts-only failure shouldn't gate)", s.State)
		}
		if s.OS != "" {
			t.Errorf("OS = %q, want empty when facts probe failed", s.OS)
		}
		if s.Mooncake != "0.9.0" {
			t.Errorf("Mooncake = %q, want populated from version", s.Mooncake)
		}
	})
	t.Run("runs fail", func(t *testing.T) {
		f := defaultFakeOK()
		f.FailRuns = true
		addr, tok := f.start(t)
		s := Probe(context.Background(), "peer", addr, tok, time.Second)
		if s.State != StateOK {
			t.Errorf("State = %q, want ok (runs-only failure shouldn't gate)", s.State)
		}
		if s.LastRunStatus != "" {
			t.Errorf("LastRunStatus = %q, want empty when runs probe failed", s.LastRunStatus)
		}
	})
}

// TestProbe_StatePrecedence walks the priority order:
// running > failed > ok. Same-peer scenarios, with the precedence
// asserted via the State field on the returned Status.
func TestProbe_StatePrecedence(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*fakeAgentd)
		wantState   State
	}{
		{
			name: "running because runs_running>0",
			mutate: func(f *fakeAgentd) {
				f.Version["runs_running"] = 1
			},
			wantState: StateRunning,
		},
		{
			name: "running because latest run non-terminal",
			mutate: func(f *fakeAgentd) {
				f.Runs[0]["status"] = "running"
				f.Runs[0]["finished_at"] = nil
				delete(f.Runs[0], "finished_at")
			},
			wantState: StateRunning,
		},
		{
			name: "failed because latest run failed",
			mutate: func(f *fakeAgentd) {
				f.Runs[0]["status"] = "failed"
			},
			wantState: StateFailed,
		},
		{
			name: "failed because latest run interrupted",
			mutate: func(f *fakeAgentd) {
				f.Runs[0]["status"] = "interrupted"
			},
			wantState: StateFailed,
		},
		{
			name: "running beats failed (running is more recent signal)",
			mutate: func(f *fakeAgentd) {
				f.Version["runs_running"] = 1
				f.Runs[0]["status"] = "failed"
			},
			wantState: StateRunning,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := defaultFakeOK()
			tt.mutate(f)
			addr, tok := f.start(t)
			s := Probe(context.Background(), "peer", addr, tok, time.Second)
			if s.State != tt.wantState {
				t.Errorf("State = %q, want %q", s.State, tt.wantState)
			}
		})
	}
}

// TestProbe_NoRunsRecorded — fresh agentd with zero runs ever shouldn't
// trip the failed/running branches. State=ok, LastRunStatus empty.
func TestProbe_NoRunsRecorded(t *testing.T) {
	f := defaultFakeOK()
	f.Runs = nil
	addr, tok := f.start(t)
	s := Probe(context.Background(), "peer", addr, tok, time.Second)
	if s.State != StateOK {
		t.Errorf("State = %q, want ok", s.State)
	}
	if s.LastRunStatus != "" {
		t.Errorf("LastRunStatus = %q, want empty", s.LastRunStatus)
	}
	if s.LastRunAge != "" {
		t.Errorf("LastRunAge = %q, want empty", s.LastRunAge)
	}
}

// TestProbeAll_PreservesOrder — N peers in, N rows out in the same order.
// The Probe goroutines race, but the [i] indexed write makes order
// deterministic. Locks in that contract.
func TestProbeAll_PreservesOrder(t *testing.T) {
	f := defaultFakeOK()
	addr, tok := f.start(t)
	peers := []Peer{
		{Name: "alpha", Addr: addr, Transport: TransportAgentd, Token: tok},
		{Name: "beta", Addr: addr, Transport: TransportAgentd, Token: tok},
		{Name: "gamma", Addr: addr, Transport: TransportAgentd, Token: tok},
	}
	rows := ProbeAll(context.Background(), peers, time.Second, 0)
	if len(rows) != len(peers) {
		t.Fatalf("got %d rows, want %d", len(rows), len(peers))
	}
	for i, want := range []string{"alpha", "beta", "gamma"} {
		if rows[i].Name != want {
			t.Errorf("row %d Name = %q, want %q", i, rows[i].Name, want)
		}
	}
}

// TestHumanDuration_RoughBuckets — the formatter is intentionally rough.
// Lock in the bucket transitions so a future tidy doesn't accidentally
// switch to seconds-everywhere.
func TestHumanDuration_RoughBuckets(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{5 * time.Second, "5s"},
		{2 * time.Minute, "2m"},
		{90 * time.Minute, "1h"},
		{3 * time.Hour, "3h"},
		{30 * time.Hour, "1d"},
		{8 * 24 * time.Hour, "1w"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%v", tt.d), func(t *testing.T) {
			if got := humanDuration(tt.d); got != tt.want {
				t.Errorf("humanDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
