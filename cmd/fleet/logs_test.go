package fleet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// fakeAgentd is the test-side double of agentd's run + SSE surface.
// Separate from internal/fleet's fakeAgentd because this one needs the
// /v1/runs/{id}/events SSE endpoint that inspect_test.go doesn't bother
// with (probes never stream).
type fakeAgentd struct {
	// Runs is returned by GET /v1/runs?limit=N (newest-first per the
	// daemon contract). Set explicitly per test.
	Runs []map[string]any

	// Events keyed by run_id; consulted on GET /v1/runs/{id}/events. A
	// missing run_id returns 404. An empty slice returns an immediately-
	// closed SSE stream (the daemon's "terminal-only, replay-then-close"
	// shape).
	Events map[string][]sseFrame

	// Facts is returned by GET /v1/facts. nil → 500 (used by tests that
	// want to exercise the unreachable-fact path).
	Facts map[string]any

	// ExpectToken matches the bearer header. Empty disables the check.
	ExpectToken string
}

type sseFrame struct {
	Type string
	Data map[string]any
	Seq  int64
}

func (f *fakeAgentd) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "0.9.0",
			"hostname":    "h",
			"synced_root": "/tmp/state/synced",
		})
	})
	mux.HandleFunc("/v1/facts", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		if f.Facts == nil {
			http.Error(w, "facts not configured", 500)
			return
		}
		_ = json.NewEncoder(w).Encode(f.Facts)
	})
	// ServeMux: "/v1/runs" is exact, "/v1/runs/" is prefix. Register both.
	mux.HandleFunc("/v1/runs", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"runs": f.Runs})
	})
	mux.HandleFunc("/v1/runs/", func(w http.ResponseWriter, r *http.Request) {
		if f.checkAuth(w, r) {
			return
		}
		path := r.URL.Path
		if strings.HasSuffix(path, "/events") {
			rid := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/runs/"), "/events")
			frames, ok := f.Events[rid]
			if !ok {
				http.Error(w, "not found", 404)
				return
			}
			f.writeSSE(w, frames)
			return
		}
		rid := strings.TrimPrefix(path, "/v1/runs/")
		for _, r := range f.Runs {
			if r["id"] == rid {
				_ = json.NewEncoder(w).Encode(r)
				return
			}
		}
		http.Error(w, "not found", 404)
	})
	return mux
}

// writeSSE emits frames in the wire shape agentd uses: `event: <type>` +
// `data: <json>` + blank line. Flushes after every frame so the
// controller's bufio.Scanner sees them as they arrive (the multiplexer's
// line-atomic rendering depends on this).
func (f *fakeAgentd) writeSSE(w http.ResponseWriter, frames []sseFrame) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	for _, fr := range frames {
		payload := map[string]any{
			"seq":       fr.Seq,
			"type":      fr.Type,
			"timestamp": "2026-05-14T22:00:00Z",
			"data":      fr.Data,
		}
		b, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", fr.Type, b)
		if flusher != nil {
			flusher.Flush()
		}
	}
	// Server-side close indicates a terminal run; the controller sees
	// io.EOF and Stream returns nil.
}

func (f *fakeAgentd) checkAuth(w http.ResponseWriter, r *http.Request) bool {
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

func (f *fakeAgentd) start(t *testing.T) (addr string) {
	t.Helper()
	srv := httptest.NewServer(f.handler())
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// writePeersToml seeds a peers.toml fixture at <dir>/peers.toml with one
// or more entries, each pointing at a freshly-started fake agentd. Returns
// the file path so the test can pass --peers-file.
func writePeersToml(t *testing.T, dir string, entries map[string]string, token string) string {
	t.Helper()
	var b strings.Builder
	for name, addr := range entries {
		fmt.Fprintf(&b, "[[peers]]\nname = %q\naddr = %q\ntoken = %q\n\n", name, addr, token)
	}
	path := filepath.Join(dir, "peers.toml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write peers.toml: %v", err)
	}
	return path
}

// captureLogsApp returns a test CLI app whose stdout is a captured buffer
// so the test can assert on rendered output. Mirrors newTestFleetApp in
// fleet_test.go but exposes the buffer.
func captureLogsApp() (*cli.App, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &cli.App{
		Commands:       []*cli.Command{Command()},
		Writer:         out,
		ErrWriter:      out,
		ExitErrHandler: func(*cli.Context, error) {},
	}, out
}

// TestFleetLogs_LatestInFlightPreferredOverTerminal exercises the heart
// of resolveLatestRun: when one of the recent runs is in-flight, that wins
// over the more-recent (or equally-recent) terminal ones.
func TestFleetLogs_LatestInFlightPreferredOverTerminal(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Runs: []map[string]any{
			{"id": "RUN-002", "status": "running"}, // newer, in-flight
			{"id": "RUN-001", "status": "success"}, // older, terminal
		},
		Events: map[string][]sseFrame{
			"RUN-002": {{Type: "step.start", Data: map[string]any{"name": "install"}, Seq: 1}},
		},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	got := out.String()
	if !strings.Contains(got, "attached to run RUN-002") {
		t.Errorf("expected attach to RUN-002 (the in-flight one), got:\n%s", got)
	}
}

// TestFleetLogs_FallsBackToNewestTerminal: when there's no in-flight run,
// the algorithm picks the newest (index 0) terminal one.
func TestFleetLogs_FallsBackToNewestTerminal(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Runs: []map[string]any{
			{"id": "RUN-003", "status": "success"}, // newest
			{"id": "RUN-002", "status": "failed"},
			{"id": "RUN-001", "status": "success"},
		},
		Events: map[string][]sseFrame{
			"RUN-003": {{Type: "run.completed", Data: map[string]any{"success": true}, Seq: 1}},
		},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "attached to run RUN-003") {
		t.Errorf("expected attach to RUN-003 (newest terminal), got:\n%s", out.String())
	}
}

// TestFleetLogs_ExplicitRunIDBypassesResolver: passing a run-id positional
// arg skips resolveLatestRun entirely. Critical because the daemon may
// have older runs the user wants to inspect specifically.
func TestFleetLogs_ExplicitRunIDBypassesResolver(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Runs: []map[string]any{
			{"id": "RUN-002", "status": "running"}, // would be picked by resolver
		},
		Events: map[string][]sseFrame{
			// Explicit run: deliberately one not in the listing.
			"RUN-OLD": {{Type: "step.end", Data: map[string]any{}, Seq: 1}},
		},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "laptop", "RUN-OLD"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "attached to run RUN-OLD") {
		t.Errorf("expected attach to RUN-OLD, got:\n%s", out.String())
	}
}

// TestFleetLogs_NoRunsErrorsCleanly: a fresh peer with zero recorded runs
// must produce a user-actionable error rather than a confusing empty
// stream. The 'no runs recorded' message is the contract.
func TestFleetLogs_NoRunsErrorsCleanly(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Runs:        []map[string]any{},
		Events:      map[string][]sseFrame{},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, _ := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "laptop"})
	if err == nil {
		t.Fatal("expected error for peer with no runs")
	}
	if !strings.Contains(err.Error(), "no runs recorded") {
		t.Errorf("err = %v, want substring 'no runs recorded'", err)
	}
}

// TestFleetLogs_UnknownPeerErrors: a peer name not in peers.toml should
// fail fast with a clear message rather than mysteriously connecting to
// nothing.
func TestFleetLogs_UnknownPeerErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": "127.0.0.1:0"}, "tok")

	app, _ := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown peer")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want substring 'not found'", err)
	}
}

// TestFleetLogs_ArgValidation pins the CLI's input contract: --all forbids
// positional args; bare logs requires <peer> [run_id].
func TestFleetLogs_ArgValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": "127.0.0.1:0"}, "tok")

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{"no args, no --all", []string{"mooncake", "fleet", "--peers-file", peersPath, "logs"}, "expected"},
		{"three positional args", []string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "a", "b", "c"}, "expected"},
		{"--all + positional", []string{"mooncake", "fleet", "--peers-file", peersPath, "logs", "--all", "laptop"}, "--all takes no positional"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := captureLogsApp()
			err := app.Run(tt.argv)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want substring %q", err, tt.want)
			}
		})
	}
}
