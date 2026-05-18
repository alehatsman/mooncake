package observe

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

// fakeAgentd is a minimal HTTP server impersonating an agentd peer for
// the runner tests. Mirrors the one in internal/fleet/exec/run_test.go
// but tuned for observe — the streamFunc emits step.completed with a
// Result map carrying the observation envelope.
const fakeToken = "test-token-spec64"

type fakeAgentd struct {
	srv          *httptest.Server
	mu           sync.Mutex
	syncedRoot   string
	streamFunc   func(runID string, w http.ResponseWriter)
	uploadedPath string
}

func newFakeAgentd(t *testing.T) *fakeAgentd {
	t.Helper()
	fa := &fakeAgentd{syncedRoot: "/var/synced"}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "0.9.0",
			"daemon_pid":  1234,
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
		w.WriteHeader(http.StatusNoContent)
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
		fa.uploadedPath = req.PlanPath
		fa.mu.Unlock()
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"run_id": "RUN-fake-001",
			"status": "queued",
		})
	})
	mux.HandleFunc("/v1/runs/", func(w http.ResponseWriter, r *http.Request) {
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

func TestObserve_HappyPath_CapturesResult(t *testing.T) {
	fa := newFakeAgentd(t)
	fa.streamFunc = func(_ string, w http.ResponseWriter) {
		writeSSE(w, []map[string]any{
			{"type": "step.completed", "data": map[string]any{
				"step_id": "step-0001",
				"name":    "observe.port",
				"changed": false,
				"result": map[string]any{
					"found": true,
					"value": map[string]any{
						"open":     true,
						"port":     80.0,
						"protocol": "tcp",
					},
					"as_of": "2026-05-15T00:00:00Z",
					"error": "",
				},
			}},
			{"type": "run.completed", "data": map[string]any{"success": true}},
		})
	}
	planBytes, err := Synthesize(SynthOptions{Kind: "port", Port: 80})
	if err != nil {
		t.Fatalf("synth: %v", err)
	}
	outs, err := Observe(context.Background(), Options{
		Peers:        []Peer{{Name: "p1", Client: fa.client("p1")}},
		PlanBytes:    planBytes,
		ControllerID: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if len(outs) != 1 {
		t.Fatalf("expected 1 outcome; got %d", len(outs))
	}
	o := outs[0]
	if o.Status != "success" {
		t.Errorf("status = %q, want success", o.Status)
	}
	if o.Result == nil {
		t.Fatal("result must not be nil")
	}
	if found, _ := o.Result["found"].(bool); !found {
		t.Errorf("found should be true; got %v", o.Result["found"])
	}
	val, _ := o.Result["value"].(map[string]any)
	if val == nil {
		t.Fatal("value map missing")
	}
	if open, _ := val["open"].(bool); !open {
		t.Errorf("value.open should be true; got %v", val["open"])
	}
}

func TestObserve_FailedRun_StatusFailed(t *testing.T) {
	fa := newFakeAgentd(t)
	fa.streamFunc = func(_ string, w http.ResponseWriter) {
		writeSSE(w, []map[string]any{
			{"type": "step.failed", "data": map[string]any{
				"step_id":       "step-0001",
				"error_message": "validation failed",
			}},
			{"type": "run.completed", "data": map[string]any{"success": false}},
		})
	}
	planBytes, _ := Synthesize(SynthOptions{Kind: "port", Port: 80})
	outs, err := Observe(context.Background(), Options{
		Peers:        []Peer{{Name: "p1", Client: fa.client("p1")}},
		PlanBytes:    planBytes,
		ControllerID: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if outs[0].Status != "failed" {
		t.Errorf("status = %q, want failed", outs[0].Status)
	}
	if outs[0].Error == "" {
		t.Errorf("Error should carry the failure message")
	}
}

func TestObserve_UnreachablePeer(t *testing.T) {
	// Point a client at a closed port so GetVersion fails.
	closed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	closed.Close()
	u, _ := url.Parse(closed.URL)
	cli := transport.New("p1", u.Host, "anything")

	planBytes, _ := Synthesize(SynthOptions{Kind: "cpu"})
	outs, err := Observe(context.Background(), Options{
		Peers:        []Peer{{Name: "p1", Client: cli}},
		PlanBytes:    planBytes,
		ControllerID: "11111111-2222-3333-4444-555555555555",
	})
	if err != nil {
		t.Fatalf("Observe: %v", err)
	}
	if outs[0].Status != "unreachable" {
		t.Errorf("status = %q, want unreachable", outs[0].Status)
	}
}

func TestSynthesize_PortValidation(t *testing.T) {
	cases := []struct {
		name    string
		opts    SynthOptions
		wantErr bool
	}{
		{"missing port", SynthOptions{Kind: "port"}, true},
		{"out of range", SynthOptions{Kind: "port", Port: 70000}, true},
		{"ok", SynthOptions{Kind: "port", Port: 80}, false},
		{"http missing url", SynthOptions{Kind: "http"}, true},
		{"http ok", SynthOptions{Kind: "http", URL: "https://x"}, false},
		{"service missing name", SynthOptions{Kind: "service"}, true},
		{"service ok", SynthOptions{Kind: "service", ServiceName: "nginx"}, false},
		{"process missing both", SynthOptions{Kind: "process"}, true},
		{"process ok", SynthOptions{Kind: "process", ProcessName: "nginx"}, false},
		{"unknown kind", SynthOptions{Kind: "moon"}, true},
		{"cpu ok", SynthOptions{Kind: "cpu"}, false},
		{"memory ok", SynthOptions{Kind: "memory"}, false},
		{"disk ok", SynthOptions{Kind: "disk"}, false},
		{"gpu ok", SynthOptions{Kind: "gpu"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Synthesize(tc.opts)
			if (err != nil) != tc.wantErr {
				t.Errorf("Synthesize(%+v) err=%v, wantErr=%v", tc.opts, err, tc.wantErr)
			}
		})
	}
}

func TestParsePortShorthand(t *testing.T) {
	cases := []struct {
		in        string
		wantPort  int
		wantProto string
	}{
		{":80", 80, ""},
		{"80", 80, ""},
		{"tcp:80", 80, "tcp"},
		{"udp:53", 53, "udp"},
		{"", 0, ""},
		{":not-a-port", 0, ""},
		{":99999", 0, ""},
	}
	for _, tc := range cases {
		p, pr := ParsePortShorthand(tc.in)
		if p != tc.wantPort || pr != tc.wantProto {
			t.Errorf("ParsePortShorthand(%q) = (%d, %q), want (%d, %q)", tc.in, p, pr, tc.wantPort, tc.wantProto)
		}
	}
}
