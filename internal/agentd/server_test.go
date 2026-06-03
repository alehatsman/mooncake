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

	client = unixClient(cfg.SocketPath)

	// Wait until the server is actually serving over the socket. The socket
	// file appearing (os.Stat) doesn't prove the accept loop is running, so a
	// real request could still race startup. Poll /v1/health (unauthenticated)
	// until it answers — any response means the handler is live.
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := client.Get("http://unix/v1/health")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-done
			t.Fatalf("server did not become ready within deadline: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	stop = func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	}
	return cfg, client, stop
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

// TestServe_TCPOnly boots agentd in TCP-only mode (SocketPath="") and
// asserts: the daemon answers /v1/version over TCP with the right bearer,
// and no socket file is created on disk. This is the contract that lets
// `mooncake agentd` run on Windows where AF_UNIX is supported but not the
// default deployment shape. Spec-49 §G2.
func TestServe_TCPOnly(t *testing.T) {
	// Grab a free port the OS-friendly way: bind+close.
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe listen: %v", err)
	}
	addr := probe.Addr().String()
	_ = probe.Close()

	stateDir := t.TempDir()
	cfg := Config{
		// SocketPath intentionally left empty — TCP-only.
		StateDir:     stateDir,
		BindAddr:     addr,
		Token:        "test-tcp-token",
		LogLevel:     "error",
		MaxSyncBytes: DefaultMaxSyncBytes,
	}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "tcp-only-test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	t.Cleanup(func() { cancel(); <-done })

	// Wait until the HTTP server is actually serving — a bare dial+close only
	// proves the listener accepts, not that the handler is live, so the real
	// request below could race a half-ready server and get RST under load.
	// Poll a real request instead: any HTTP response means it's serving.
	probeClient := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, perr := probeClient.Get("http://" + addr + "/v1/version")
		if perr == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("server did not become ready within deadline: %v", perr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Hit /v1/version with the right token.
	req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/v1/version", nil)
	req.Header.Set("Authorization", "Bearer test-tcp-token")
	c := &http.Client{Timeout: 2 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("GET /v1/version: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status=%d body=%q", resp.StatusCode, body)
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode version: %v", err)
	}
	if payload["hostname"] == nil {
		t.Errorf("hostname missing from /v1/version: %v", payload)
	}

	// Critical: no socket file should exist anywhere in stateDir or its
	// parent — TCP-only mode must not create one. We can't easily check
	// "anywhere on disk", so we look at the obvious places.
	for _, p := range []string{
		stateDir + "/agentd.sock",
		"./agentd.sock", // would happen if we MkdirAll("." + filepath.Dir(""))
	} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("TCP-only mode created socket file at %s", p)
		}
	}
}

// TestServe_RejectsBothListenersEmpty — defense in depth. Validate()
// catches this, but New() should too if anyone bypasses Validate. We
// confirm New refuses the no-listener config.
func TestServe_RejectsBothListenersEmpty(t *testing.T) {
	cfg := Config{
		StateDir: t.TempDir(),
		LogLevel: "error",
	}
	_, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err == nil {
		t.Fatal("want error for no-listener config, got nil")
	}
	if !strings.Contains(err.Error(), "socket_path or bind_addr") {
		t.Errorf("err = %v, want 'at least one of socket_path or bind_addr'", err)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
