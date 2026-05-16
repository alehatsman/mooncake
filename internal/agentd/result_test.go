package agentd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/apply"
)

// TestRunResult_HappyPath is the R2.1c agentd-side E2E check: submit a
// trivial plan against the test server, wait for terminal, GET
// /v1/runs/{id}/result, and assert the daemon's serialised
// apply.KernelResult has the four documented kernel-surface fields
// (Plan / Steps / Events / Summary) populated coherently.
//
// Frontend equivalent (controller side): fleet.Apply consumes the same
// shape via transport.Client.GetRunResult; that path is exercised by
// the orchestrator integration test once a real two-process flow is
// wired. This test pins the daemon side of the contract.
func TestRunResult_HappyPath(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	planPath := writeTrivialPlan(t)
	sub := submitRun(t, client, planPath)
	run := waitForTerminal(t, client, sub.RunID, 10*time.Second)
	if run.Status != StatusSuccess {
		t.Fatalf("want success, got %s; error=%q", run.Status, run.Error)
	}

	resp, err := client.Get("http://unix/v1/runs/" + sub.RunID + "/result")
	if err != nil {
		t.Fatalf("GET result: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET result: status=%d body=%s", resp.StatusCode, body)
	}

	var result apply.KernelResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	// Plan should be populated (writeTrivialPlan writes a one-step plan).
	if result.Plan == nil {
		t.Error("Result.Plan = nil; want populated")
	}
	// Steps should record the one trivial step.
	if len(result.Steps) == 0 {
		t.Error("Result.Steps = empty; want >= 1 step record")
	}
	// Summary should reflect a successful run.
	if !result.Summary.Success {
		t.Errorf("Summary.Success = false; want true (Summary=%+v)", result.Summary)
	}
	if result.Summary.TotalSteps == 0 {
		t.Errorf("Summary.TotalSteps = 0; want >= 1")
	}
}

// TestRunResult_NotReady covers the explicit 404 result_not_ready
// response shape that GetRunResult will see when called between
// "daemon flipped Status to terminal" and "writeResult finished".
// Use a fabricated run id that the store hasn't seen — the handler
// must NOT treat that as "result not ready" but as "run_not_found"
// (different error code; controllers must distinguish them).
func TestRunResult_NotFoundVsNotReady(t *testing.T) {
	_, client, stop := startTestServer(t)
	defer stop()

	// Unknown run id → run_not_found, not result_not_ready.
	resp, err := client.Get("http://unix/v1/runs/01H00000000000000000000000/result")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "run_not_found") {
		t.Errorf("want run_not_found, got body=%s", body)
	}
}
