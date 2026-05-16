package agentd

// F016 contract: Worker.Shutdown() cancels the in-flight apply via a
// shared runCtx; the executor's step-loop check (Spec-16 step-boundary
// cancellation) short-circuits at the next iteration and returns
// context.Canceled to apply.Runner. Pre-fix the worker passed
// context.Background() unconditionally and Shutdown blocked until the
// in-flight apply finished organically — daemon shutdown could hang
// indefinitely on a long apply.

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestWorkerShutdownCancelsInFlightApply submits a plan with many
// file.write steps, then immediately calls Shutdown. The runCtx
// cancel must propagate through apply.Runner → executor.Start →
// ExecuteSteps, breaking the step loop on its next iteration. We
// assert (a) Shutdown returns within a short deadline and (b) fewer
// than all step writes landed — proof that the loop short-circuited
// rather than running to completion.
func TestWorkerShutdownCancelsInFlightApply(t *testing.T) {
	stateDir := t.TempDir()
	store, err := NewStore(stateDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Build a 50-step plan. file.write is fast (~ms per step on a tmpfs)
	// but executing all 50 takes long enough that a cancel landing in
	// the middle leaves visible evidence: only a prefix of the files
	// exist. The exact count doesn't matter; "< 50" is the F016 invariant.
	const totalSteps = 50
	planDir := t.TempDir()
	writeDir := filepath.Join(planDir, "writes")
	if err := os.MkdirAll(writeDir, 0o755); err != nil {
		t.Fatalf("mkdir writes: %v", err)
	}
	var sb strings.Builder
	for i := 0; i < totalSteps; i++ {
		fmt := "- file.write: { path: " + filepath.Join(writeDir, "f") + "-"
		sb.WriteString(fmt)
		sb.WriteString(itoa(i))
		sb.WriteString(", content: x }\n")
	}
	planPath := filepath.Join(planDir, "plan.yml")
	if err := os.WriteFile(planPath, []byte(sb.String()), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	run, err := store.Create(SubmitReq{
		PlanPath: planPath,
		BaseDir:  planDir,
	})
	if err != nil {
		t.Fatalf("Store.Create: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := NewWorker(store, log)

	w.Submit(run.ID)
	go w.Run()

	// Cancel immediately after submit. Some steps may land before the
	// cancel propagates — that's fine. The contract is "not all of them
	// land, and Shutdown returns promptly."
	done := make(chan struct{})
	go func() {
		w.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Shutdown returned — the F016 invariant. Without the fix
		// Shutdown blocks on the in-flight apply running to completion,
		// which for 50 file.writes is still fast but for a real apply
		// with a hung step would be unbounded. The < 5s deadline is the
		// regression guard: a future change that breaks the
		// ctx-propagation chain will trip on a slow CI runner.
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown did not return within 5s — runCtx cancel not propagating to executor")
	}

	// Verify fewer than all step outputs exist. Belt-and-braces: a
	// future change that races cancel against the planner (e.g. plan
	// is built but executor.Start returns before the loop dispatches)
	// would also satisfy the "Shutdown returns" assertion above. The
	// step-output count is the harder assertion.
	entries, err := os.ReadDir(writeDir)
	if err != nil {
		t.Fatalf("ReadDir writes: %v", err)
	}
	if len(entries) >= totalSteps {
		t.Errorf("all %d files exist after Shutdown — cancel did not interrupt the step loop", totalSteps)
	}
}

// itoa is a tiny helper to avoid pulling strconv in for one call.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
