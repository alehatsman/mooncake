package agentd

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func unixClient(socket string) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", socket)
			},
		},
		Timeout: 5 * time.Second,
	}
}

// startTestServer boots a daemon on a temp socket and returns it plus a
// cancel func that stops the daemon and waits for shutdown.
func startTestServer(t *testing.T) (cfg Config, client *http.Client, stop func()) {
	t.Helper()

	// Use a short-prefix temp dir for the socket to stay within the 104-byte
	// UNIX_PATH_MAX on macOS. t.TempDir() embeds the full test name, which
	// pushes long test names past the limit and causes net.Listen to fail.
	socketDir, err := os.MkdirTemp("", "mc")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })

	tmp := t.TempDir()
	cfg = Config{
		SocketPath: filepath.Join(socketDir, "s.sock"),
		StateDir:   filepath.Join(tmp, "state"),
		LogLevel:   "error",
	}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	// Wait for the socket to appear.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.SocketPath); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := os.Stat(cfg.SocketPath); err != nil {
		cancel()
		<-done
		t.Fatalf("socket never appeared: %v", err)
	}

	stop = func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	}
	return cfg, unixClient(cfg.SocketPath), stop
}

func TestHealthEndpoint(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/health")
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("want status=ok, got %v", body)
	}
}

func TestVersionEndpoint(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/version")
	if err != nil {
		t.Fatalf("GET version: %v", err)
	}
	defer resp.Body.Close()

	var body versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Version != "test" {
		t.Errorf("want version=test, got %q", body.Version)
	}
	if body.DaemonPID != os.Getpid() {
		t.Errorf("want pid=%d, got %d", os.Getpid(), body.DaemonPID)
	}
	if body.SystemMode {
		t.Errorf("system_mode should be false in test")
	}
	if body.Hostname == "" {
		t.Errorf("hostname is empty; expected os.Hostname() to populate it")
	}
	if body.SyncedRoot != filepath.Join(cfg.StateDir, "synced") {
		t.Errorf("synced_root = %q, want %q", body.SyncedRoot, filepath.Join(cfg.StateDir, "synced"))
	}
}

func TestFactsEndpoint(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/facts")
	if err != nil {
		t.Fatalf("GET facts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// "arch" is one of the few facts guaranteed to be present on every platform.
	if _, ok := body["arch"]; !ok {
		t.Errorf("expected 'arch' in facts, got keys: %v", keys(body))
	}
}

func TestMetricsEndpoint(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/metrics")
	if err != nil {
		t.Fatalf("GET metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["cpu_usage_pct"]; !ok {
		t.Errorf("expected 'cpu_usage_pct' in metrics, got keys: %v", keys(body))
	}
}

func TestMetricsEndpointFieldsFilter(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/metrics?fields=cpu_usage_pct")
	if err != nil {
		t.Fatalf("GET metrics: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["cpu_usage_pct"]; !ok {
		t.Errorf("expected 'cpu_usage_pct' in filtered response, got %v", keys(body))
	}
	if _, ok := body["_collected_at"]; !ok {
		t.Errorf("expected '_collected_at' sibling when fields= used, got %v", keys(body))
	}
	if _, ok := body["memory_used_mb"]; ok {
		t.Errorf("did not expect 'memory_used_mb' when fields=cpu_usage_pct, got %v", keys(body))
	}
}

func TestMCPEndpointToolsList(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	body := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	resp, err := client.Post("http://unix/v1/mcp", "application/json", body)
	if err != nil {
		t.Fatalf("POST mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var got struct {
		JSONRPC string `json:"jsonrpc"`
		Result  struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.JSONRPC != "2.0" {
		t.Errorf("want jsonrpc=2.0, got %q", got.JSONRPC)
	}
	if len(got.Result.Tools) == 0 {
		t.Errorf("expected tools list to be non-empty")
	}
}

func TestMCPEndpointRejectsGET(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/mcp")
	if err != nil {
		t.Fatalf("GET mcp: %v", err)
	}
	defer resp.Body.Close()
	// http.ServeMux's method-aware routing returns 405 when only POST is registered.
	if resp.StatusCode != http.StatusMethodNotAllowed && resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 405 or 404 for GET, got %d", resp.StatusCode)
	}
}

func TestUnknownRouteReturns404(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/nope")
	if err != nil {
		t.Fatalf("GET unknown: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestRequestIDIsEchoed(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	req, _ := http.NewRequest("GET", "http://unix/v1/health", nil)
	req.Header.Set("X-Request-ID", "my-correlation-id")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("X-Request-ID"); got != "my-correlation-id" {
		t.Errorf("want X-Request-ID echoed, got %q", got)
	}
}

func TestServerShutdownCleansSocket(t *testing.T) {
	cfg, _, stop := startTestServer(t)
	stop()
	if _, err := os.Stat(cfg.SocketPath); !os.IsNotExist(err) {
		t.Errorf("socket should be removed on shutdown, stat err = %v", err)
	}
}

func TestServerRefusesIfAlreadyBound(t *testing.T) {
	cfg, _, stop := startTestServer(t)
	defer stop()

	// Second server on the same socket should refuse.
	srv2, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	err = srv2.Serve(ctx)
	if err == nil {
		t.Fatal("expected error when second daemon tries to bind, got nil")
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
