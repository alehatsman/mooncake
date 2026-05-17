package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
)

// startLoopbackUDP binds a UDP listener on 127.0.0.1 and returns it.
// fleet_up tests point --broadcast at this addr so wol.Send writes to
// a port we own — hermetic, and doesn't depend on broadcast bits.
func startLoopbackUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("ListenUDP: %v", err)
	}
	return conn
}

// fakeShutdownAgentd is a narrow stand-in covering only the surface
// fleet shutdown / up / mac-refresh exercise: /v1/self/mac,
// /v1/self/shutdown, /v1/version. Separate from fakeAgentd
// (logs_test) to keep the broader test fixture untouched.
type fakeShutdownAgentd struct {
	mac                  string
	macStatus            int // 0 → 200; otherwise sent as the response code with no_mac error
	shutdownStatus       int // 0 → 202
	shutdownCalls        atomic.Int32
	macCalls             atomic.Int32
	versionCalls         atomic.Int32
	versionDownUntilCall int32 // 0-based call index threshold; calls before this fail
}

func (f *fakeShutdownAgentd) start(t *testing.T) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/self/mac", func(w http.ResponseWriter, r *http.Request) {
		f.macCalls.Add(1)
		if f.macStatus != 0 {
			w.WriteHeader(f.macStatus)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "no_mac", "message": "no usable interface"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"mac": f.mac, "interface": "eth0"})
	})
	mux.HandleFunc("/v1/self/shutdown", func(w http.ResponseWriter, r *http.Request) {
		f.shutdownCalls.Add(1)
		if f.shutdownStatus != 0 {
			w.WriteHeader(f.shutdownStatus)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "runs_in_flight", "message": "n run(s) in flight"})
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"daemon_pid": 4321, "scheduled_in_sec": 1})
	})
	mux.HandleFunc("/v1/version", func(w http.ResponseWriter, r *http.Request) {
		got := f.versionCalls.Add(1)
		if got < f.versionDownUntilCall {
			http.Error(w, "down", 503)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": "0.9.0", "hostname": "h", "synced_root": "/tmp",
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return strings.TrimPrefix(srv.URL, "http://")
}

// writePeersTomlWithMAC variant that also writes an optional MAC.
// Existing writePeersToml in fleet_logs_test.go is fine for unset-MAC
// cases; this helper is used by tests that pre-seed a MAC.
func writePeersTomlWithMAC(t *testing.T, dir, name, addr, token, mac string) string {
	t.Helper()
	body := fmt.Sprintf("[[peers]]\nname = %q\naddr = %q\ntoken = %q\n", name, addr, token)
	if mac != "" {
		body += fmt.Sprintf("mac = %q\n", mac)
	}
	path := filepath.Join(dir, "peers.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write peers.toml: %v", err)
	}
	return path
}

func TestFleetMACRefresh_WritesMACToPeersToml(t *testing.T) {
	fake := &fakeShutdownAgentd{mac: "AA:BB:CC:11:22:33"}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "mac-refresh", "--peers-file", peersPath, "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}

	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(cfg.Peers) != 1 {
		t.Fatalf("peers = %d, want 1", len(cfg.Peers))
	}
	// MAC stored in canonical lowercase colon form.
	if cfg.Peers[0].MAC != "aa:bb:cc:11:22:33" {
		t.Errorf("stored MAC = %q, want aa:bb:cc:11:22:33", cfg.Peers[0].MAC)
	}
	if !strings.Contains(out.String(), "laptop:") {
		t.Errorf("output missing peer name line: %s", out.String())
	}
}

func TestFleetMACRefresh_ReportsErrorOnNoMACFromPeer(t *testing.T) {
	fake := &fakeShutdownAgentd{macStatus: http.StatusNotFound}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"nas": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "mac-refresh", "--peers-file", peersPath, "nas"})
	if err == nil {
		t.Fatalf("want error, got nil; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "no_mac") {
		t.Errorf("output missing no_mac signal: %s", out.String())
	}

	// peers.toml MAC must remain unset.
	cfg, _ := fleet.LoadPeers(peersPath)
	if cfg.Peers[0].MAC != "" {
		t.Errorf("MAC was modified on failure: %q", cfg.Peers[0].MAC)
	}
}

func TestFleetShutdown_AutoCollectsMAC(t *testing.T) {
	fake := &fakeShutdownAgentd{mac: "AA:BB:CC:DD:EE:FF"}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "shutdown", "--peers-file", peersPath, "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if fake.shutdownCalls.Load() != 1 {
		t.Errorf("shutdown calls = %d, want 1", fake.shutdownCalls.Load())
	}
	if fake.macCalls.Load() != 1 {
		t.Errorf("mac calls = %d, want 1 (auto-collect on shutdown)", fake.macCalls.Load())
	}
	cfg, _ := fleet.LoadPeers(peersPath)
	if cfg.Peers[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC not stored: got %q", cfg.Peers[0].MAC)
	}
	if !strings.Contains(out.String(), "shutdown scheduled") {
		t.Errorf("missing 'shutdown scheduled' confirmation: %s", out.String())
	}
}

func TestFleetShutdown_SkipsMACWhenAlreadySet(t *testing.T) {
	fake := &fakeShutdownAgentd{mac: "AA:BB:CC:DD:EE:FF"}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersTomlWithMAC(t, dir, "laptop", addr, "tok", "11:22:33:44:55:66")

	app, _ := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "shutdown", "--peers-file", peersPath, "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.macCalls.Load() != 0 {
		t.Errorf("mac calls = %d, want 0 (already set)", fake.macCalls.Load())
	}
	if fake.shutdownCalls.Load() != 1 {
		t.Errorf("shutdown calls = %d, want 1", fake.shutdownCalls.Load())
	}
}

func TestFleetShutdown_NoMACCollectFlag(t *testing.T) {
	fake := &fakeShutdownAgentd{mac: "AA:BB:CC:DD:EE:FF"}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, _ := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "shutdown", "--peers-file", peersPath, "--no-mac-collect", "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.macCalls.Load() != 0 {
		t.Errorf("mac calls = %d, want 0 (--no-mac-collect)", fake.macCalls.Load())
	}
	if fake.shutdownCalls.Load() != 1 {
		t.Errorf("shutdown calls = %d, want 1", fake.shutdownCalls.Load())
	}
}

func TestFleetShutdown_RefusesWithNoPeerSelected(t *testing.T) {
	fake := &fakeShutdownAgentd{}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "shutdown", "--peers-file", peersPath})
	if err == nil {
		t.Fatalf("want error when no peer selected, got nil; output: %s", out.String())
	}
	if fake.shutdownCalls.Load() != 0 {
		t.Errorf("shutdown called with no selection: %d", fake.shutdownCalls.Load())
	}
}

func TestFleetUp_RefusesWithoutStoredMAC(t *testing.T) {
	fake := &fakeShutdownAgentd{}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "up", "--peers-file", peersPath, "laptop"})
	if err == nil {
		t.Fatalf("want error when MAC is unset, got nil; output: %s", out.String())
	}
	if !strings.Contains(out.String(), "no stored MAC") && !strings.Contains(err.Error(), "no stored MAC") {
		t.Errorf("output missing helpful hint: out=%s err=%v", out.String(), err)
	}
	// No magic packet should have been sent — we can't easily detect
	// this via the fake (UDP is out-of-band), but we can at least
	// assert no agentd calls were made.
	if fake.versionCalls.Load() != 0 {
		t.Errorf("/v1/version polled before send: %d", fake.versionCalls.Load())
	}
}

func TestFleetUp_NoWaitSucceeds(t *testing.T) {
	// With a stored MAC and --no-wait, the command sends the magic
	// packet via wol.Send to the configured broadcast and returns
	// immediately. The broadcast hits 255.255.255.255:9 by default
	// which usually succeeds on Linux dev hosts; on hardened CI it
	// can fail, so we point at a loopback UDP port we own so the
	// test is hermetic.
	udp := startLoopbackUDP(t)
	defer udp.Close()

	fake := &fakeShutdownAgentd{}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersTomlWithMAC(t, dir, "laptop", addr, "tok", "aa:bb:cc:dd:ee:ff")

	app, out := captureLogsApp()
	err := app.Run([]string{
		"mooncake", "fleet", "up", "--peers-file", peersPath,
		"--no-wait", "--broadcast", udp.LocalAddr().String(),
		"laptop",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	if !strings.Contains(out.String(), "magic packet sent") {
		t.Errorf("missing send confirmation: %s", out.String())
	}
	// No /v1/version polls under --no-wait.
	if fake.versionCalls.Load() != 0 {
		t.Errorf("/v1/version polled despite --no-wait: %d", fake.versionCalls.Load())
	}
}
