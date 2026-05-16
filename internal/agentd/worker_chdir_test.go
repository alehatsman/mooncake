package agentd

// F015 contract: when executeRun's chdir to BaseDir fails, the hub
// must close so any SSE subscriber that attached between Submit and
// worker pickup gets an end-of-stream signal on its channel. Today
// this happens via two redundant paths — the unified defer at the
// top of executeRun (F015's refactor) AND the cascade through
// RunEventSink.Close → s.hub.Close (jsonl_sink.go:122). Either
// alone is enough; a future change that unwires the cascade must
// not regress this contract.

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	// Side-effect import: ensures action handlers are registered so the
	// worker can build a plan if execution gets that far (it won't in the
	// chdir-error path, but the worker compiles its plan reader either way).
	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestWorkerChdirFailureClosesHub: a run with a non-existent BaseDir
// fails at os.Chdir inside executeRun. The hub must close — verified by
// asserting the subscriber's channel closes within a deadline. Pre-fix
// the channel stayed open forever and this test would block until the
// timeout.
func TestWorkerChdirFailureClosesHub(t *testing.T) {
	stateDir := t.TempDir()
	store, err := NewStore(stateDir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Plan file must exist on disk — the worker reads it to build the
	// apply plan even on the failure paths. (It doesn't matter that we
	// never reach the apply call, because chdir runs first.)
	planDir := t.TempDir()
	planPath := filepath.Join(planDir, "plan.yml")
	if err := os.WriteFile(planPath, []byte("- log: { msg: never-runs }\n"), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	// BaseDir is set to a path the store does NOT validate (it accepts
	// arbitrary strings; the HTTP layer is what calls os.Stat). At chdir
	// time inside executeRun this fails and we hit the F015 path.
	run, err := store.Create(SubmitReq{
		PlanPath: planPath,
		BaseDir:  filepath.Join(stateDir, "nonexistent-base-dir-for-f015"),
	})
	if err != nil {
		t.Fatalf("Store.Create: %v", err)
	}

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	w := NewWorker(store, log)

	// Submit pre-registers the hub (spec-49). Grab a handle and subscribe
	// before the worker dequeues, matching what a controller does when it
	// hits POST /v1/runs and immediately follows up with GET /v1/runs/{id}/events.
	w.Submit(run.ID)
	hub := w.GetHub(run.ID)
	if hub == nil {
		t.Fatal("Submit did not pre-register hub for run")
	}
	ch, _, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	// Start the worker pump in a goroutine. Shutdown the channel at the
	// end so worker.Run returns and Shutdown's done-wait completes.
	go w.Run()
	defer w.Shutdown()

	// The subscriber's channel must close once executeRun's deferred
	// hub.Close() runs. Pre-fix: channel stays open forever, this select
	// hits the deadline.
	select {
	case _, ok := <-ch:
		if ok {
			// A live event arrived — drain and keep waiting for the close.
			// (Normal-path runs would deliver run.started here; in this
			// chdir-error test the worker fails before publishing anything,
			// but be defensive.)
			for range ch {
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("hub.Close() did not fire on chdir-error path — F015 regressed (subscriber channel still open after 3s)")
	}

	// Belt-and-braces: also assert the worker recorded the failure so a
	// future regression that closes the hub for the wrong reason (e.g.
	// closing on success path while the chdir branch silently swallows
	// the failure) doesn't pass this test.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, err := store.Get(run.ID)
		if err == nil && r.Status == StatusFailed {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run never reached StatusFailed within 2s")
}
