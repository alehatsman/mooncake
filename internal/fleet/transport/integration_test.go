package transport_test

// Integration test: spin up a real agentd over TCP, drive every transport
// method against it, assert the wire contract holds end-to-end. Catches
// drift between the client's wire-shape assumptions and the daemon's
// actual responses.
//
// In a `*_test` package so it sees only the public API of transport, the
// way real callers do.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/agentd"
	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
	_ "github.com/alehatsman/mooncake/internal/register" // register action handlers for the in-process daemon
)

// startAgentdTCP boots a real agentd listening on a free TCP port. Returns
// the bind addr, the token, the synced root, and a stop function.
func startAgentdTCP(t *testing.T) (addr, token, syncedRoot string, stop func()) {
	t.Helper()

	// Resolve a free TCP port without holding it open.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("probe tcp port: %v", err)
	}
	tcpAddr := ln.Addr().String()
	_ = ln.Close()

	socketDir, err := os.MkdirTemp("", "mc-it")
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
		Token:        "integration-token",
		MaxSyncBytes: agentd.DefaultMaxSyncBytes,
	}
	srv, err := agentd.New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), "it")
	if err != nil {
		t.Fatalf("agentd.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	// Poll a real HTTP request — a bare TCP dial only proves the listener
	// exists; the HTTP handler can still RST the first request if it hasn't
	// started serving yet (same pattern as startTestServerTCP in agentd).
	probe := &http.Client{Timeout: 200 * time.Millisecond}
	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := probe.Get("http://" + tcpAddr + "/v1/version")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("agentd did not become ready within deadline: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	stop = func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("Serve returned error: %v", err)
		}
	}
	return tcpAddr, cfg.Token, cfg.SyncedRoot(), stop
}

// TestIntegration_RoundTrip exercises Version → Head → Put → Head (hit) →
// Submit → Stream against a real agentd.
func TestIntegration_RoundTrip(t *testing.T) {
	addr, token, syncedRoot, stop := startAgentdTCP(t)
	defer stop()

	c := transport.New("it-peer", addr, token)

	// --- GetVersion: SyncedRoot must match the real daemon's view -------
	v, err := c.GetVersion(context.Background())
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if v.SyncedRoot != syncedRoot {
		t.Errorf("synced_root over wire = %q, want %q", v.SyncedRoot, syncedRoot)
	}
	if v.Version != "it" {
		t.Errorf("version = %q, want it", v.Version)
	}

	// --- HEAD miss for a not-yet-uploaded file --------------------------
	const scope = "controller-abc/dir-12345"
	const relPath = "config.yml"
	body := []byte("steps: []\n")
	sum := sha256.Sum256(body)
	shaHex := hex.EncodeToString(sum[:])

	hit, err := c.Head(context.Background(), scope, relPath, shaHex)
	if err != nil {
		t.Fatalf("Head pre-PUT: %v", err)
	}
	if hit {
		t.Errorf("Head returned hit before PUT")
	}

	// --- PUT, then HEAD hit ---------------------------------------------
	srcPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(srcPath, body, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := c.Put(context.Background(), scope, relPath, srcPath, shaHex); err != nil {
		t.Fatalf("Put: %v", err)
	}

	hit, err = c.Head(context.Background(), scope, relPath, shaHex)
	if err != nil {
		t.Fatalf("Head post-PUT: %v", err)
	}
	if !hit {
		t.Errorf("Head returned miss after PUT")
	}

	// --- Submit: plan_path points at the synced file --------------------
	planAbsOnPeer := filepath.Join(syncedRoot, scope, relPath)
	runID, err := c.Submit(context.Background(), transport.SubmitRequest{
		PlanPath: planAbsOnPeer,
		BaseDir:  filepath.Dir(planAbsOnPeer),
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if !strings.HasPrefix(runID, "01") || len(runID) != 26 {
		t.Errorf("run_id %q does not look like a ULID", runID)
	}

	// --- Stream: collect events until the run reaches terminal ----------
	sink := make(chan transport.Event, 16)
	streamCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := c.Stream(streamCtx, runID, sink); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	close(sink)

	var events []transport.Event
	for ev := range sink {
		events = append(events, ev)
	}
	if len(events) == 0 {
		t.Fatal("Stream returned no events")
	}
	// Seq numbers should be monotonically increasing.
	for i := 1; i < len(events); i++ {
		if events[i].Seq <= events[i-1].Seq {
			t.Errorf("seq non-monotonic at i=%d: %d → %d",
				i, events[i-1].Seq, events[i].Seq)
		}
	}
	// At least one event should be a known top-level type. Light assertion
	// because the executor's event taxonomy is its own concern; we just
	// want to confirm we got real events from a real run.
	var seenType string
	for _, ev := range events {
		if ev.Type != "" {
			seenType = ev.Type
			break
		}
	}
	if seenType == "" {
		t.Errorf("no event carried a non-empty Type field; events=%+v", events)
	}
}

// TestIntegration_GetRunResult exercises R2.1c's wire path end-to-end:
// spin up a real agentd over TCP, submit a plan, wait for the stream to
// close (run reaches terminal), then call GetRunResult and assert the
// daemon's serialised apply.KernelResult comes back with the four
// documented kernel-surface fields populated.
//
// This is the integration counterpart to the agentd-side
// TestRunResult_HappyPath; together they pin the wire contract from
// both ends.
func TestIntegration_GetRunResult(t *testing.T) {
	addr, token, syncedRoot, stop := startAgentdTCP(t)
	defer stop()

	c := transport.New("it-peer", addr, token)

	// Sync a trivial plan (one no-op step).
	const scope = "controller-r21c/dir-r21c"
	const relPath = "plan.yml"
	body := []byte(`
- name: r21c smoke
  log: "kernel-result roundtrip"
`)
	sum := sha256.Sum256(body)
	shaHex := hex.EncodeToString(sum[:])

	srcPath := filepath.Join(t.TempDir(), relPath)
	if err := os.WriteFile(srcPath, body, 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	if err := c.Put(context.Background(), scope, relPath, srcPath, shaHex); err != nil {
		t.Fatalf("Put: %v", err)
	}

	// Submit.
	planAbsOnPeer := filepath.Join(syncedRoot, scope, relPath)
	runID, err := c.Submit(context.Background(), transport.SubmitRequest{
		PlanPath: planAbsOnPeer,
		BaseDir:  filepath.Dir(planAbsOnPeer),
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// Drain stream until terminal so writeResult has had time to finish.
	sink := make(chan transport.Event, 16)
	streamCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Stream(streamCtx, runID, sink); err != nil {
		t.Fatalf("Stream: %v", err)
	}
	close(sink)
	for range sink { //nolint:revive // drain remaining buffered events
	}

	// Poll GetRunResult: writeResult races with status update; a tiny
	// retry loop tolerates the window without sleeping in the happy path.
	var result *apply.KernelResult
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, err := c.GetRunResult(context.Background(), runID)
		if err == nil {
			result = r
			break
		}
		if !errors.Is(err, transport.ErrRunResultNotReady) {
			t.Fatalf("GetRunResult: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if result == nil {
		t.Fatalf("GetRunResult never returned a populated result before deadline")
	}

	// The four documented kernel-surface fields, all populated.
	if result.Plan == nil {
		t.Error("KernelResult.Plan = nil; want compiled plan")
	}
	if len(result.Steps) == 0 {
		t.Error("KernelResult.Steps = empty; want >= 1 step")
	}
	if !result.Summary.Success {
		t.Errorf("Summary.Success = false; want true (Summary=%+v)", result.Summary)
	}
	if result.Summary.TotalSteps == 0 {
		t.Errorf("Summary.TotalSteps = 0; want >= 1")
	}

	// 404 path: a bogus run id must surface as a real error, not a
	// silent nil result. Use a syntactically valid ULID so the server
	// doesn't reject it as invalid_run_id first.
	_, err = c.GetRunResult(context.Background(), "01H00000000000000000000000")
	if err == nil {
		t.Error("GetRunResult on unknown run id returned nil error")
	}
}

// TestIntegration_BadTokenRejected sanity-checks that the bearer middleware
// from spec-43 PR1 actually gates the TCP listener.
func TestIntegration_BadTokenRejected(t *testing.T) {
	addr, _, _, stop := startAgentdTCP(t)
	defer stop()

	bad := transport.New("bad", addr, "wrong")
	_, err := bad.GetVersion(context.Background())
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("want 401 error, got %v", err)
	}
}
