package exec

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

func TestExitCode_AllSuccessReturnsZero(t *testing.T) {
	got := ExitCode([]PeerOutcome{
		{Peer: "a", Status: "success"},
		{Peer: "b", Status: "success"},
	})
	if got != 0 {
		t.Errorf("all-success rc = %d, want 0", got)
	}
}

func TestExitCode_AnyFailReturnsOne(t *testing.T) {
	got := ExitCode([]PeerOutcome{
		{Peer: "a", Status: "success"},
		{Peer: "b", Status: "failed", ExitCode: 3},
	})
	if got != 1 {
		t.Errorf("one-failed rc = %d, want 1", got)
	}
}

func TestExitCode_UnreachableTrumpsFailed(t *testing.T) {
	got := ExitCode([]PeerOutcome{
		{Peer: "a", Status: "failed"},
		{Peer: "b", Status: "unreachable"},
	})
	if got != 2 {
		t.Errorf("mix unreachable+failed rc = %d, want 2", got)
	}
}

func TestExitCode_ErrorTreatedAsUnreachable(t *testing.T) {
	got := ExitCode([]PeerOutcome{{Peer: "a", Status: "error"}})
	if got != 2 {
		t.Errorf("error rc = %d, want 2", got)
	}
}

func TestSummary_AllOk(t *testing.T) {
	s := Summary([]PeerOutcome{
		{Peer: "a", Status: "success"},
		{Peer: "b", Status: "success"},
	})
	if s != "fleet exec: 2/2 ok" {
		t.Errorf("summary = %q", s)
	}
}

func TestSummary_NamesFailures(t *testing.T) {
	s := Summary([]PeerOutcome{
		{Peer: "a", Status: "success"},
		{Peer: "b", Status: "failed", ExitCode: 3},
		{Peer: "c", Status: "unreachable"},
	})
	if !strings.Contains(s, "1/3 ok") {
		t.Errorf("summary should report 1/3, got %q", s)
	}
	if !strings.Contains(s, "failed on b (exit 3)") {
		t.Errorf("summary should name b's exit, got %q", s)
	}
	if !strings.Contains(s, "unreachable: c") {
		t.Errorf("summary should name c as unreachable, got %q", s)
	}
}

func TestScopeFor_StaysWithinDaemonLimits(t *testing.T) {
	// validateScope (internal/agentd/files_handler.go) caps the scope at
	// 128 bytes total and 64 bytes per segment with at most one '/' separator.
	uuid := "00000000-0000-0000-0000-000000000000" // 36 chars
	s := scopeFor(uuid)
	if strings.Count(s, "/") > 1 {
		t.Fatalf("scope must have at most one '/' separator: %q", s)
	}
	if len(s) > 128 {
		t.Errorf("scope length %d exceeds daemon cap", len(s))
	}
	for _, seg := range strings.Split(s, "/") {
		if len(seg) > 64 {
			t.Errorf("scope segment %q exceeds 64 bytes", seg)
		}
	}
}

func TestNewULID_UniqueAndStable(t *testing.T) {
	a := newULID()
	b := newULID()
	if a == b {
		t.Errorf("ULIDs must differ across calls: %q == %q", a, b)
	}
	if len(a) != 26 {
		t.Errorf("ULID length = %d, want 26", len(a))
	}
}

// --- Fake-agentd integration test -----------------------------------------
//
// Spec-52 §"Tests" calls for an integration test against a real agentd.
// We approximate that with an httptest.Server that mimics the four
// endpoints exec actually uses: GET /v1/version, PUT /v1/files, POST
// /v1/runs, GET /v1/runs/{id}/events (SSE).

const fakeToken = "test-token-spec52"

type fakeAgentd struct {
	srv          *httptest.Server
	mu           sync.Mutex
	syncedRoot   string
	uploaded     map[string][]byte                         // path → bytes
	streamFunc   func(runID string, w http.ResponseWriter) // installed per-test
	planPathSeen string
	exitOnRun    int // simulated exit code (0 = success)
}

func newFakeAgentd(t *testing.T) *fakeAgentd {
	t.Helper()
	fa := &fakeAgentd{
		syncedRoot: "/var/synced",
		uploaded:   make(map[string][]byte),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "0.9.0",
			"daemon_pid":  4242,
			"hostname":    "fake",
			"synced_root": fa.syncedRoot,
			"system_mode": false,
		})
	})
	mux.HandleFunc("/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		q := r.URL.Query()
		scope, p := q.Get("scope"), q.Get("path")
		body := make([]byte, r.ContentLength)
		if _, err := r.Body.Read(body); err != nil && err.Error() != "EOF" {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fa.mu.Lock()
		fa.uploaded[scope+"/"+p] = body
		fa.mu.Unlock()
		w.WriteHeader(http.StatusNoContent) // transport client expects 204
	})
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			PlanPath string `json:"plan_path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		fa.mu.Lock()
		fa.planPathSeen = req.PlanPath
		fa.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"run_id": "RUN-fake-001",
			"status": "queued",
		})
	})
	mux.HandleFunc("/v1/runs/", func(w http.ResponseWriter, r *http.Request) {
		// Two routes: /v1/runs/{id} (GET) and /v1/runs/{id}/events (SSE).
		switch {
		case strings.HasSuffix(r.URL.Path, "/events"):
			runID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/runs/"), "/events")
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if fa.streamFunc != nil {
				fa.streamFunc(runID, w)
			}
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":     "RUN-fake-001",
				"status": "success",
			})
		}
	})
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+fakeToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	})
	fa.srv = httptest.NewServer(authed)
	t.Cleanup(fa.srv.Close)
	return fa
}

func (fa *fakeAgentd) client(name string) *transport.Client {
	u, _ := url.Parse(fa.srv.URL)
	return transport.New(name, u.Host, fakeToken)
}

// writeSSE renders a sequence of typed events on the SSE wire. The
// daemon's real shape is: `id: <seq>\ndata: <json>\n\n`. We emit a
// minimal slice of run events suitable for execOne to drive a happy
// path or a failure path.
func writeSSE(w http.ResponseWriter, events []map[string]any) {
	for i, ev := range events {
		body, _ := json.Marshal(map[string]any{
			"seq":       i + 1,
			"type":      ev["type"],
			"timestamp": "2026-05-15T00:00:00Z",
			"data":      ev["data"],
		})
		fmt.Fprintf(w, "id: %d\ndata: %s\n\n", i+1, body)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func TestExec_HappyPath_SuccessExitZero(t *testing.T) {
	fa := newFakeAgentd(t)
	fa.streamFunc = func(_ string, w http.ResponseWriter) {
		writeSSE(w, []map[string]any{
			{"type": "step.stdout", "data": map[string]any{"line": "active"}},
			{"type": "run.completed", "data": map[string]any{"success": true}},
		})
	}
	planBytes, err := Synthesize(SynthOptions{Cmd: "systemctl is-active sshd"})
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	outs, err := Exec(context.Background(), ExecOptions{
		Peers:         []ExecPeer{{Name: "p1", Client: fa.client("p1")}},
		PlanBytes:     planBytes,
		ControllerID:  "11111111-2222-3333-4444-555555555555",
		CollectOutput: true,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(outs) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(outs))
	}
	o := outs[0]
	if o.Status != "success" || o.ExitCode != 0 {
		t.Errorf("outcome = %+v", o)
	}
	if !strings.Contains(o.Stdout, "active") {
		t.Errorf("stdout missing 'active': %q", o.Stdout)
	}
	if ExitCode(outs) != 0 {
		t.Errorf("aggregate rc = %d, want 0", ExitCode(outs))
	}
}

func TestExec_StepFailedSurfacesExitCodeAndStatus(t *testing.T) {
	fa := newFakeAgentd(t)
	fa.streamFunc = func(_ string, w http.ResponseWriter) {
		writeSSE(w, []map[string]any{
			{"type": "step.stdout", "data": map[string]any{"line": "inactive"}},
			{"type": "step.failed", "data": map[string]any{
				"exit_code":     3,
				"error_message": "command failed with exit code 3",
			}},
			{"type": "run.completed", "data": map[string]any{"success": false}},
		})
	}
	planBytes, _ := Synthesize(SynthOptions{Cmd: "systemctl is-active sshd"})
	outs, err := Exec(context.Background(), ExecOptions{
		Peers:         []ExecPeer{{Name: "p1", Client: fa.client("p1")}},
		PlanBytes:     planBytes,
		ControllerID:  "11111111-2222-3333-4444-555555555555",
		CollectOutput: true,
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	o := outs[0]
	if o.Status != "failed" {
		t.Errorf("status = %q, want failed", o.Status)
	}
	if o.ExitCode != 3 {
		t.Errorf("exit_code = %d, want 3", o.ExitCode)
	}
	if ExitCode(outs) != 1 {
		t.Errorf("aggregate rc = %d, want 1", ExitCode(outs))
	}
}

func TestExec_UnreachablePeerReturnsRcTwo(t *testing.T) {
	// Construct a transport.Client pointing at a closed listener so
	// GetVersion fails with a connection error.
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	closed.Close() // immediate close so the next dial fails
	u, _ := url.Parse(closed.URL)
	cli := transport.New("dead", u.Host, fakeToken)

	planBytes, _ := Synthesize(SynthOptions{Cmd: "true"})
	outs, err := Exec(context.Background(), ExecOptions{
		Peers:        []ExecPeer{{Name: "dead", Client: cli}},
		PlanBytes:    planBytes,
		ControllerID: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if outs[0].Status != "unreachable" {
		t.Errorf("status = %q, want unreachable", outs[0].Status)
	}
	if ExitCode(outs) != 2 {
		t.Errorf("aggregate rc = %d, want 2", ExitCode(outs))
	}
}

func TestExec_ParallelFanOutAllPeers(t *testing.T) {
	fa := newFakeAgentd(t)
	fa.streamFunc = func(_ string, w http.ResponseWriter) {
		writeSSE(w, []map[string]any{
			{"type": "run.completed", "data": map[string]any{"success": true}},
		})
	}
	planBytes, _ := Synthesize(SynthOptions{Cmd: "true"})
	peers := []ExecPeer{
		{Name: "p1", Client: fa.client("p1")},
		{Name: "p2", Client: fa.client("p2")},
		{Name: "p3", Client: fa.client("p3")},
	}
	outs, err := Exec(context.Background(), ExecOptions{
		Peers:        peers,
		PlanBytes:    planBytes,
		ControllerID: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(outs) != 3 {
		t.Fatalf("expected 3 outcomes, got %d", len(outs))
	}
	// Outcomes are in input order.
	for i, o := range outs {
		if o.Peer != peers[i].Name {
			t.Errorf("outcome %d peer = %q, want %q", i, o.Peer, peers[i].Name)
		}
		if o.Status != "success" {
			t.Errorf("outcome %d status = %q", i, o.Status)
		}
	}
}

func TestExec_NoPeersErrors(t *testing.T) {
	_, err := Exec(context.Background(), ExecOptions{
		PlanBytes:    []byte("foo"),
		ControllerID: "x",
	})
	if err == nil {
		t.Fatal("expected error for empty peers")
	}
}

func TestExec_EmptyPlanBytesErrors(t *testing.T) {
	_, err := Exec(context.Background(), ExecOptions{
		Peers:        []ExecPeer{{Name: "p"}},
		ControllerID: "x",
	})
	if err == nil {
		t.Fatal("expected error for empty plan bytes")
	}
}
