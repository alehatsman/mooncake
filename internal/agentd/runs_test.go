package agentd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	// Side-effect import: registers all action handlers so the worker can
	// actually execute submitted plans inside tests. Production wiring
	// happens in cmd/mooncake.go which imports the same package.
	_ "github.com/alehatsman/mooncake/internal/register"
)

const trivialPlan = `- name: greet
  log:
    msg: "hi from test"
`

// writeTrivialPlan creates a known-good YAML plan in a temp dir and returns
// its absolute path.
func writeTrivialPlan(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "plan.yml")
	if err := os.WriteFile(path, []byte(trivialPlan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	return path
}

// submitRun POSTs to /v1/runs and returns the parsed response.
func submitRun(t *testing.T, client *http.Client, planPath string) submitResponse {
	t.Helper()
	body, _ := json.Marshal(submitRequest{PlanPath: planPath})
	resp, err := client.Post("http://unix/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST runs: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("submit: status=%d body=%s", resp.StatusCode, bodyBytes)
	}
	var got submitResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode submit: %v", err)
	}
	return got
}

// waitForTerminal polls /v1/runs/{id} until status is terminal or timeout.
func waitForTerminal(t *testing.T, client *http.Client, runID string, timeout time.Duration) *Run {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get("http://unix/v1/runs/" + runID)
		if err != nil {
			t.Fatalf("GET run: %v", err)
		}
		var got Run
		if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
			resp.Body.Close()
			t.Fatalf("decode run: %v", err)
		}
		resp.Body.Close()
		if got.IsTerminal() {
			return &got
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never reached terminal status within %s", runID, timeout)
	return nil
}

func TestSubmitRunHappyPath(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	if sub.Status != StatusQueued {
		t.Errorf("want status=queued, got %s", sub.Status)
	}
	if len(sub.RunID) != 26 {
		t.Errorf("want ULID run_id, got %q", sub.RunID)
	}

	run := waitForTerminal(t, client, sub.RunID, 10*time.Second)
	if run.Status != StatusSuccess {
		t.Errorf("want success, got %s; error=%q", run.Status, run.Error)
	}
}

func TestSubmitRunRejectsRelativePlanPath(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	body := bytes.NewReader([]byte(`{"plan_path":"./plan.yml"}`))
	resp, err := client.Post("http://unix/v1/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "absolute") {
		t.Errorf("expected error mentioning 'absolute', got: %s", b)
	}
}

func TestSubmitRunRejectsMissingPlanPath(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	body := bytes.NewReader([]byte(`{"plan_path":"/nonexistent/plan.yml"}`))
	resp, err := client.Post("http://unix/v1/runs", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("want 400, got %d", resp.StatusCode)
	}
}

func TestListRunsAfterSubmit(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	waitForTerminal(t, client, sub.RunID, 10*time.Second)

	resp, err := client.Get("http://unix/v1/runs")
	if err != nil {
		t.Fatalf("GET runs: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Runs []Run `json:"runs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Runs) == 0 {
		t.Fatal("expected at least one run")
	}
	if body.Runs[0].ID != sub.RunID {
		t.Errorf("want newest run=%s, got %s", sub.RunID, body.Runs[0].ID)
	}
}

func TestGetRunNotFound(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	resp, err := client.Get("http://unix/v1/runs/01H00000000000000000000000")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
}

func TestRunEventsReplayMatchesJSONL(t *testing.T) {
	cfg, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	waitForTerminal(t, client, sub.RunID, 10*time.Second)

	// SSE the events. Because the run is already terminal, the handler just
	// replays JSONL and closes.
	resp, err := client.Get("http://unix/v1/runs/" + sub.RunID + "/events")
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}
	defer resp.Body.Close()
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Errorf("want text/event-stream, got %q", got)
	}
	streamBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}

	// Count SSE events ("\n\n" terminator).
	sseEventCount := strings.Count(string(streamBody), "\n\n")
	// Count JSONL lines.
	jsonlPath := filepath.Join(cfg.StateDir, "runs", sub.RunID, "events.jsonl")
	jsonlBytes, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	jsonlLines := strings.Count(string(jsonlBytes), "\n")

	if sseEventCount != jsonlLines {
		t.Errorf("SSE event count (%d) != JSONL lines (%d). Either streaming or persistence dropped events.\nSSE: %s", sseEventCount, jsonlLines, streamBody)
	}
	if jsonlLines == 0 {
		t.Errorf("no events captured")
	}
}

// TestRunEvents_SubscribeImmediatelyAfterSubmit guards two races discovered
// in spec-49 live testing on Windows:
//
//  1. worker.Submit() now pre-registers the run's SSE hub before pushing to
//     the queue. Without this, a controller fast enough to subscribe between
//     POST /v1/runs returning and the worker's executeRun getting scheduled
//     found GetHub(id)==nil and runEventsHandler bailed silently.
//
//  2. streamJSONL now treats a missing events.jsonl as "no replay needed"
//     instead of erroring out. Without this, the handler returned early
//     (dropping the live hub tail it had already subscribed to) whenever
//     the worker hadn't yet appended the first event.
//
// Both races are masked on Linux by fast scheduling; both are
// reproducible by using a sleep-y plan that delays the worker's first
// event write. We submit, subscribe immediately, and assert the stream
// delivers at least the run.started + run.completed events.
func TestRunEvents_SubscribeImmediatelyAfterSubmit(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	// Use a plan with a small artificial delay so the worker reliably
	// hasn't written the first event by the time we GET /events. Without
	// the delay this test would still pass on fast machines but wouldn't
	// reliably catch a regression of either race.
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.yml")
	plan := `- name: slow_first_step
  shell: sleep 0.25 && echo go
- name: trivial
  log:
    msg: hi
`
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	sub := submitRun(t, client, planPath)

	// Subscribe immediately. The worker is almost certainly still queued
	// at this point; events.jsonl does not exist yet.
	resp, err := client.Get("http://unix/v1/runs/" + sub.RunID + "/events")
	if err != nil {
		t.Fatalf("GET events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	// Read with a deadline so a regression (handler returns immediately
	// with no events) fails fast instead of hanging.
	type readResult struct {
		body []byte
		err  error
	}
	doneCh := make(chan readResult, 1)
	go func() {
		b, err := io.ReadAll(resp.Body)
		doneCh <- readResult{body: b, err: err}
	}()

	var body []byte
	select {
	case r := <-doneCh:
		if r.err != nil {
			t.Fatalf("read events: %v", r.err)
		}
		body = r.body
	case <-time.After(10 * time.Second):
		t.Fatal("SSE stream didn't terminate within 10s — handler likely hung")
	}

	// Sanity: stream must contain at least run.started and run.completed.
	// A regression that returns 0 bytes (the original Windows symptom)
	// would fail here with a clear message.
	if len(body) == 0 {
		t.Fatal("SSE stream returned 0 bytes — hub/JSONL race regressed")
	}
	s := string(body)
	if !strings.Contains(s, `"type":"run.started"`) {
		t.Errorf("missing run.started event in stream:\n%s", s)
	}
	if !strings.Contains(s, `"type":"run.completed"`) {
		t.Errorf("missing run.completed event in stream:\n%s", s)
	}
}

func TestSubscriberCloseFlushBeforeTerminalRecord(t *testing.T) {
	// Worker ordering invariant: by the time GET /v1/runs/{id} returns a
	// terminal status, all events must already be on disk in events.jsonl.
	// If the close ordering is wrong (record written before sink flushed),
	// callers see "completed" with missing tail events.
	cfg, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	run := waitForTerminal(t, client, sub.RunID, 10*time.Second)
	if run.Status != StatusSuccess {
		t.Fatalf("plan failed: %s", run.Error)
	}

	// The very last event published by executor.Start is run.completed. If
	// it's missing from JSONL, the close ordering is wrong.
	jsonlPath := filepath.Join(cfg.StateDir, "runs", sub.RunID, "events.jsonl")
	data, err := os.ReadFile(jsonlPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	if !strings.Contains(string(data), `"type":"run.completed"`) {
		t.Errorf("run.completed missing from events.jsonl — close ordering bug?\n%s", data)
	}
}
