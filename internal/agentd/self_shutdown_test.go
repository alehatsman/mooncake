package agentd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withTestShutdownHooks swaps shutdownExec for a counter-returning
// stub and resets shutdownArmed + shutdownDelay so tests don't
// actually power off the host (or wait 1s for the goroutine).
// Returns a func that returns the call count and restores the
// originals.
func withTestShutdownHooks(t *testing.T) func() (calls int32) {
	t.Helper()
	var n int32
	prevExec := SetShutdownExec(func() error {
		atomic.AddInt32(&n, 1)
		return nil
	})
	prevDelay := SetShutdownDelay(5 * time.Millisecond)
	shutdownMu.Lock()
	shutdownArmed = false
	shutdownMu.Unlock()
	t.Cleanup(func() {
		SetShutdownExec(prevExec)
		SetShutdownDelay(prevDelay)
		shutdownMu.Lock()
		shutdownArmed = false
		shutdownMu.Unlock()
	})
	return func() int32 { return atomic.LoadInt32(&n) }
}

func TestSelfShutdown_RepliesAccepted(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()
	getCalls := withTestShutdownHooks(t)

	resp, err := client.Post("http://unix/v1/self/shutdown", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202; body=%s", resp.StatusCode, string(b))
	}
	var out shutdownResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.DaemonPID == 0 {
		t.Errorf("daemon_pid = 0, want non-zero")
	}
	// Wait for the goroutine to fire.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if getCalls() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if getCalls() != 1 {
		t.Errorf("shutdown exec calls = %d, want 1", getCalls())
	}
}

func TestSelfShutdown_RejectsRedundantPost(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()
	// Block the exec goroutine so shutdownArmed stays true.
	prevExec := SetShutdownExec(func() error {
		select {} // block forever
	})
	defer SetShutdownExec(prevExec)
	prevDelay := SetShutdownDelay(5 * time.Millisecond)
	defer SetShutdownDelay(prevDelay)
	shutdownMu.Lock()
	shutdownArmed = false
	shutdownMu.Unlock()
	defer func() {
		shutdownMu.Lock()
		shutdownArmed = false
		shutdownMu.Unlock()
	}()

	resp, err := client.Post("http://unix/v1/self/shutdown", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("first POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("first POST status = %d, want 202", resp.StatusCode)
	}

	// Second POST must be rejected — the goroutine is blocked but
	// shutdownArmed is set.
	resp2, err := client.Post("http://unix/v1/self/shutdown", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("second POST: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second POST status = %d, want 409", resp2.StatusCode)
	}
	body, _ := io.ReadAll(resp2.Body)
	if !bytes.Contains(body, []byte("shutdown_in_progress")) {
		t.Errorf("want shutdown_in_progress in body, got %s", body)
	}
}

func TestSelfShutdown_ToleratesEmptyBody(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()
	withTestShutdownHooks(t)

	// No body at all (Content-Length: 0).
	req, err := http.NewRequest(http.MethodPost, "http://unix/v1/self/shutdown", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 202; body=%s", resp.StatusCode, string(b))
	}
}
