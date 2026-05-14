package fleet_test

// Integration test for the full Apply orchestration: a real agentd is
// started over TCP, the controller walks a temp plan-dir, syncs it, submits
// a run, and reads the streamed events. Lives in *_test package to use only
// the public API.

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/agentd"
	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// startAgentdTCP boots a real agentd on a random TCP port. Mirrors the
// transport-package integration helper, duplicated here because we can't
// cross-import _test packages.
func startAgentdTCP(t *testing.T) (addr, token string, stop func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe tcp port: %v", err)
	}
	tcpAddr := ln.Addr().String()
	_ = ln.Close()

	socketDir, err := os.MkdirTemp("", "mc-fa")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })

	stateDir := t.TempDir()
	cfg := agentd.Config{
		SocketPath:   filepath.Join(socketDir, "s.sock"),
		StateDir:     stateDir,
		LogLevel:     "error",
		BindAddr:     tcpAddr,
		Token:        "apply-token",
		MaxSyncBytes: agentd.DefaultMaxSyncBytes,
	}
	srv, err := agentd.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "fa")
	if err != nil {
		t.Fatalf("agentd.New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", tcpAddr, 100*time.Millisecond)
		if err == nil {
			_ = c.Close()
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	stop = func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve returned: %v", err)
		}
	}
	return tcpAddr, cfg.Token, stop
}

// makePlanDir writes a minimal trivially-valid mooncake config tree.
// Returns the absolute plan-dir and the top-level YAML path.
func makePlanDir(t *testing.T) (dir, planPath string) {
	t.Helper()
	dir = t.TempDir()
	planPath = filepath.Join(dir, "config.yml")
	// Empty steps: an apply that does nothing is the cheapest "real run"
	// — it exercises the full event flow without depending on real
	// actions.
	if err := os.WriteFile(planPath, []byte("steps: []\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir, planPath
}

func TestApply_RoundTrip(t *testing.T) {
	addr, token, stop := startAgentdTCP(t)
	defer stop()

	dir, planPath := makePlanDir(t)
	client := transport.New("test-peer", addr, token)
	out := &bytes.Buffer{}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := fleet.Apply(ctx, fleet.ApplyOptions{
		PeerName:     "test-peer",
		Peer:         client,
		PlanDir:      dir,
		PlanPath:     planPath,
		ControllerID: "00000000-0000-4000-8000-000000000000",
		MaxSyncBytes: 100 << 20,
		Writer:       out,
	})
	if err != nil {
		t.Fatalf("Apply: %v\noutput:\n%s", err, out.String())
	}

	if res.RunID == "" {
		t.Error("RunID empty")
	}
	if res.Status != "success" {
		t.Errorf("Status = %q, want success. Output:\n%s", res.Status, out.String())
	}
	if res.Sync.Put == 0 {
		t.Errorf("Sync.Put = 0; expected at least the top-level YAML uploaded")
	}
	if res.Events == 0 {
		t.Errorf("Events = 0; expected at least run.started + run.completed")
	}

	// Output should carry [test-peer]-prefixed lines.
	s := out.String()
	if !strings.Contains(s, "[test-peer]") {
		t.Errorf("output missing peer prefix:\n%s", s)
	}
}

func TestApply_SecondRunSkipsSync(t *testing.T) {
	// Second Apply against the same plan-dir + controller-id should
	// HEAD-skip every file — Put == 0, Skipped == len(entries).
	addr, token, stop := startAgentdTCP(t)
	defer stop()

	dir, planPath := makePlanDir(t)
	client := transport.New("p", addr, token)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	opts := fleet.ApplyOptions{
		PeerName:     "p",
		Peer:         client,
		PlanDir:      dir,
		PlanPath:     planPath,
		ControllerID: "00000000-0000-4000-8000-000000000000",
		MaxSyncBytes: 100 << 20,
		Writer:       io.Discard,
	}
	first, err := fleet.Apply(ctx, opts)
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if first.Sync.Put != 1 || first.Sync.Skipped != 0 {
		t.Errorf("first: put=%d skipped=%d, want 1/0", first.Sync.Put, first.Sync.Skipped)
	}

	second, err := fleet.Apply(ctx, opts)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if second.Sync.Put != 0 || second.Sync.Skipped != 1 {
		t.Errorf("second: put=%d skipped=%d, want 0/1", second.Sync.Put, second.Sync.Skipped)
	}
}

func TestApply_RejectsVarsOutsidePlanDir(t *testing.T) {
	outsideDir := t.TempDir()
	outsideVars := filepath.Join(outsideDir, "vars.yml")
	if err := os.WriteFile(outsideVars, []byte("foo: bar\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	dir, planPath := makePlanDir(t)
	_, err := fleet.Apply(context.Background(), fleet.ApplyOptions{
		PeerName:     "p",
		Peer:         transport.New("p", "127.0.0.1:0", "tok"),
		PlanDir:      dir,
		PlanPath:     planPath,
		VarsFiles:    []string{outsideVars},
		ControllerID: "00000000-0000-4000-8000-000000000000",
		Writer:       io.Discard,
	})
	if err == nil {
		t.Fatal("want validation error for vars outside plan-dir")
	}
	if !strings.Contains(err.Error(), "outside PlanDir") {
		t.Errorf("err = %v, want outside-PlanDir msg", err)
	}
}

// TestApply_EventsChannel runs Apply with Events set instead of Writer and
// verifies the PR6 fan-in path: every observable phase (sync, submit, real
// SSE events) lands as a typed PeerEvent on the channel. Without this
// coverage, regressions in the emitter switch (e.g. dropped KindSubmit)
// would go unnoticed until the multiplexer renders nothing.
func TestApply_EventsChannel(t *testing.T) {
	addr, token, stop := startAgentdTCP(t)
	defer stop()

	dir, planPath := makePlanDir(t)
	client := transport.New("evpeer", addr, token)

	// Buffered enough to never block Apply: ~3 control + ~5 SSE events for
	// an empty plan.
	ch := make(chan fleet.PeerEvent, 64)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	res, err := fleet.Apply(ctx, fleet.ApplyOptions{
		PeerName:     "evpeer",
		Peer:         client,
		PlanDir:      dir,
		PlanPath:     planPath,
		ControllerID: "00000000-0000-4000-8000-000000000000",
		MaxSyncBytes: 100 << 20,
		Events:       ch,
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Status != "success" {
		t.Errorf("Status = %q, want success", res.Status)
	}
	close(ch)

	// Tally observed event kinds. Every successful Apply must emit at least
	// one KindSync, one KindSubmit, and one KindEvent. KindDisconnect /
	// KindError must NOT fire on the happy path.
	var sync, submit, ev, disc, errk int
	for pe := range ch {
		if pe.Peer != "evpeer" {
			t.Errorf("unexpected Peer field %q", pe.Peer)
		}
		switch pe.Kind {
		case fleet.KindSync:
			sync++
		case fleet.KindSubmit:
			submit++
		case fleet.KindEvent:
			ev++
		case fleet.KindDisconnect:
			disc++
		case fleet.KindError:
			errk++
		}
	}
	if sync != 1 {
		t.Errorf("KindSync count = %d, want 1", sync)
	}
	if submit != 1 {
		t.Errorf("KindSubmit count = %d, want 1", submit)
	}
	if ev == 0 {
		t.Errorf("KindEvent count = 0, want ≥1 (run.started + run.completed at minimum)")
	}
	if disc != 0 {
		t.Errorf("KindDisconnect should not fire on happy path, got %d", disc)
	}
	if errk != 0 {
		t.Errorf("KindError should not fire on happy path, got %d", errk)
	}
}

// TestApply_EventsChannelTakesPriorityOverWriter — when both Events and
// Writer are set, Events wins and Writer must stay untouched. Catches the
// failure mode where double-emission leaks the same line to both stdout
// (via Writer) and the multiplexer (via Events), producing duplicates.
func TestApply_EventsChannelTakesPriorityOverWriter(t *testing.T) {
	addr, token, stop := startAgentdTCP(t)
	defer stop()
	dir, planPath := makePlanDir(t)
	client := transport.New("p", addr, token)

	ch := make(chan fleet.PeerEvent, 64)
	w := &bytes.Buffer{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := fleet.Apply(ctx, fleet.ApplyOptions{
		PeerName:     "p",
		Peer:         client,
		PlanDir:      dir,
		PlanPath:     planPath,
		ControllerID: "00000000-0000-4000-8000-000000000000",
		MaxSyncBytes: 100 << 20,
		Events:       ch,
		Writer:       w,
	}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	close(ch)
	if w.Len() != 0 {
		t.Errorf("Writer should be untouched when Events is set; got %q", w.String())
	}
	count := 0
	for range ch {
		count++
	}
	if count == 0 {
		t.Error("Events channel got nothing — emitter likely no-op'd")
	}
}

func TestApply_RejectsRelativePaths(t *testing.T) {
	_, err := fleet.Apply(context.Background(), fleet.ApplyOptions{
		PeerName:     "p",
		Peer:         transport.New("p", "127.0.0.1:0", "tok"),
		PlanDir:      "relative/dir",
		PlanPath:     "relative/dir/config.yml",
		ControllerID: "00000000-0000-4000-8000-000000000000",
		Writer:       io.Discard,
	})
	if err == nil || !strings.Contains(err.Error(), "must be absolute") {
		t.Errorf("want absolute-path error, got %v", err)
	}
}
