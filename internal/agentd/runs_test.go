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
