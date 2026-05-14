package agentd

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// startTestServerTCP starts a daemon listening on BOTH the unix socket and a
// TCP port. Returns the config (with the resolved bind addr + token), an
// HTTP client targeting TCP, the unix-socket client for comparison, and a
// stop function.
func startTestServerTCP(t *testing.T) (cfg Config, tcpClient, unixHTTPClient *http.Client, stop func()) {
	t.Helper()

	// Resolve a free TCP port without holding it open.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe tcp port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	socketDir, err := os.MkdirTemp("", "mc")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })

	tmp := t.TempDir()
	cfg = Config{
		SocketPath:   filepath.Join(socketDir, "s.sock"),
		StateDir:     filepath.Join(tmp, "state"),
		LogLevel:     "error",
		BindAddr:     addr,
		Token:        "test-token-secret",
		MaxSyncBytes: DefaultMaxSyncBytes,
	}
	srv, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	// Wait for the TCP listener to accept.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	stop = func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	}
	return cfg, &http.Client{Timeout: 5 * time.Second}, unixClient(cfg.SocketPath), stop
}

func TestTCP_RequiresBearerToken(t *testing.T) {
	cfg, client, _, stop := startTestServerTCP(t)
	defer stop()

	resp, err := client.Get("http://" + cfg.BindAddr + "/v1/version")
	if err != nil {
		t.Fatalf("GET version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 on missing bearer, got %d", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got == "" {
		t.Errorf("want WWW-Authenticate header on 401")
	}
}

func TestTCP_RejectsWrongToken(t *testing.T) {
	cfg, client, _, stop := startTestServerTCP(t)
	defer stop()

	req, _ := http.NewRequest(http.MethodGet, "http://"+cfg.BindAddr+"/v1/version", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 on wrong bearer, got %d", resp.StatusCode)
	}
}

func TestTCP_AcceptsCorrectToken(t *testing.T) {
	cfg, client, _, stop := startTestServerTCP(t)
	defer stop()

	req, _ := http.NewRequest(http.MethodGet, "http://"+cfg.BindAddr+"/v1/version", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200, got %d", resp.StatusCode)
	}
}

func TestTCP_RejectsTokenWithExtraSuffix(t *testing.T) {
	// Sanity: constant-time compare must reject `Bearer <token>X` even
	// though the prefix matches. This guards against a sloppy
	// strings.HasPrefix implementation drift.
	cfg, client, _, stop := startTestServerTCP(t)
	defer stop()

	req, _ := http.NewRequest(http.MethodGet, "http://"+cfg.BindAddr+"/v1/version", nil)
	req.Header.Set("Authorization", "Bearer "+cfg.Token+"X")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("want 401 on token-with-suffix, got %d", resp.StatusCode)
	}
}

func TestUnixSocket_UnaffectedByAuth(t *testing.T) {
	// Even when the daemon has a TCP listener with bearer auth, the unix
	// socket continues to accept unauthenticated requests. Filesystem perms
	// are the access control there.
	_, _, unixCli, stop := startTestServerTCP(t)
	defer stop()

	resp, err := unixCli.Get("http://unix/v1/version")
	if err != nil {
		t.Fatalf("GET version over unix: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("want 200 on unauth unix request, got %d", resp.StatusCode)
	}
}

func TestNew_RejectsBindWithoutToken(t *testing.T) {
	cfg := Config{
		SocketPath: "/tmp/mc-test.sock",
		StateDir:   t.TempDir(),
		BindAddr:   "127.0.0.1:0",
		// Token deliberately empty.
	}
	_, err := New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "test")
	if err == nil {
		t.Fatal("want error: bind set without token")
	}
}
