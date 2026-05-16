package agentd

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/events"
)

// Worker is the single-goroutine FIFO runner of submitted plans. v1 makes no
// attempt at concurrency: concurrent applies of the same plan or of different
// plans touching the same paths/services can clobber each other. The worker
// also chdirs to the run's base_dir so apply.Runner's os.Getwd()-based path
// resolution matches what the submitter intended — this is only safe under
// the single-worker invariant.
type Worker struct {
	store *Store
	log   *slog.Logger

	submit chan string // run IDs queued for execution

	hubMu sync.Mutex
	hubs  map[string]*Hub // active runs only — removed on terminal

	done chan struct{}

	mu          sync.Mutex
	queueDepth  int
	runsRunning int
}

func NewWorker(store *Store, log *slog.Logger) *Worker {
	return &Worker{
		store:  store,
		log:    log,
		submit: make(chan string, 1024),
		hubs:   make(map[string]*Hub),
		done:   make(chan struct{}),
	}
}

// Submit enqueues a run for execution. Non-blocking up to the channel
// buffer.
//
// A hub is created and registered eagerly here, before the worker dequeues
// the run. Without this, a controller that subscribes to /v1/runs/{id}/events
// between submit and worker pickup would see GetHub(id)==nil, the SSE
// handler would bail, and the controller would see a 0-byte stream — a
// race that's masked on Linux by fast scheduling and exposed on Windows
// where worker pickup latency is hundreds of milliseconds. Spec-49.
func (w *Worker) Submit(runID string) {
	w.hubMu.Lock()
	if _, exists := w.hubs[runID]; !exists {
		w.hubs[runID] = NewHub()
	}
	w.hubMu.Unlock()

	w.mu.Lock()
	w.queueDepth++
	w.mu.Unlock()
	w.submit <- runID
}

// Stats returns the queue and running counts.
func (w *Worker) Stats() (queued, running int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.queueDepth, w.runsRunning
}

// GetHub returns the active Hub for a run, or nil if the run is not currently
// being executed (queued or already terminal).
func (w *Worker) GetHub(runID string) *Hub {
	w.hubMu.Lock()
	defer w.hubMu.Unlock()
	return w.hubs[runID]
}

// Run consumes the submit channel until Shutdown is called. Should be invoked
// in its own goroutine.
func (w *Worker) Run() {
	defer close(w.done)
	for runID := range w.submit {
		w.executeRun(runID)
	}
}

// Shutdown closes the submit channel and waits for the in-flight run, if any,
// to finish. Per the v1 plan there is no cancellation: in-flight runs run to
// completion.
func (w *Worker) Shutdown() {
	close(w.submit)
	<-w.done
}

func (w *Worker) executeRun(runID string) {
	w.mu.Lock()
	w.queueDepth--
	w.runsRunning++
	w.mu.Unlock()
	defer func() {
		w.mu.Lock()
		w.runsRunning--
		w.mu.Unlock()
	}()

	startedAt := time.Now().UTC()
	run, err := w.store.Update(runID, func(r *Run) error {
		r.Status = StatusRunning
		r.StartedAt = &startedAt
		return nil
	})
	if err != nil {
		w.log.Error("worker: mark running", "run_id", runID, "err", err)
		return
	}

	// Hub was pre-registered by Submit() so subscribers attaching between
	// submit and worker pickup don't race. Look it up here; only create a
	// new one as a defensive fallback (shouldn't happen for runs that came
	// through Submit, but Reconcile()-style paths might call executeRun
	// directly in the future).
	w.hubMu.Lock()
	hub, ok := w.hubs[runID]
	if !ok {
		hub = NewHub()
		w.hubs[runID] = hub
	}
	w.hubMu.Unlock()
	// F015: hub lifetime is tied to executeRun. Every exit path (apply
	// success / failure / chdir-error / sink-create-error / panic) must
	// close the hub so subscribers' channels signal end-of-stream — pre-fix
	// the chdir-error path returned without calling hub.Close(), leaking
	// SSE subscribers (their goroutines blocked forever on a channel that
	// would never close). Close BEFORE delete from the map so a subscriber
	// that called GetHub() concurrently with the delete observes a closed
	// hub and lands on Subscribe's "already closed" branch (which closes
	// the subscriber's channel immediately) rather than a stale-but-open
	// one. Hub.Close is idempotent so the previously-explicit Close on the
	// sink-create-error path is now redundant — removed.
	defer func() {
		hub.Close()
		w.hubMu.Lock()
		delete(w.hubs, runID)
		w.hubMu.Unlock()
	}()

	sink, err := NewRunEventSink(runID, w.store.EventsPath(runID), hub, w.log)
	if err != nil {
		w.log.Error("worker: create sink", "run_id", runID, "err", err)
		w.markFailed(run, err.Error(), time.Now().UTC())
		return
	}

	prevDir, _ := os.Getwd()
	if run.BaseDir != "" {
		if err := os.Chdir(run.BaseDir); err != nil {
			w.log.Error("worker: chdir base_dir", "run_id", runID, "base_dir", run.BaseDir, "err", err)
			// sink was never handed to apply.Runner — close it directly so the
			// file is released before we return. (The hub closes via the
			// unified defer above.)
			sink.Close()
			w.markFailed(run, "chdir base_dir: "+err.Error(), time.Now().UTC())
			return
		}
		defer func() { _ = os.Chdir(prevDir) }()
	}

	// apply.Runner calls Flush() → ExtraSubscribers.Close() → publisher.Close()
	// (via defer) before returning. CRITICAL ORDER is preserved: events are
	// drained to disk before writeKernelResult runs. RunLogSubscriber for
	// run.PlanPath is wired internally by apply.Runner.
	kr, execErr := apply.NewRunner(&apply.Config{
		ConfigPath:       run.PlanPath,
		VarsFiles:        run.VarsFiles,
		Tags:             run.Tags,
		Names:            run.Names,
		OutputFormat:     "quiet",
		ExtraSubscribers: []events.Subscriber{sink},
	}).Run(context.Background())

	// R2.1c: persist result.json so the controller can fetch it via
	// GET /v1/runs/{id}/result. Best-effort: failure is logged but
	// doesn't fail the run (events.jsonl + record.json are authoritative).
	if writeErr := w.writeKernelResult(runID, kr); writeErr != nil {
		w.log.Warn("worker: write result.json", "run_id", runID, "err", writeErr)
	}

	finishedAt := time.Now().UTC()
	if execErr != nil {
		w.markFailed(run, execErr.Error(), finishedAt)
		return
	}
	if _, err := w.store.Update(runID, func(r *Run) error {
		r.Status = StatusSuccess
		r.FinishedAt = &finishedAt
		return nil
	}); err != nil {
		w.log.Error("worker: mark success", "run_id", runID, "err", err)
	}
}

// writeKernelResult writes the apply.KernelResult returned by apply.Runner
// to the run directory as result.json.
//
// Notes on the wire shape: Result.ReverseData and Result.Detail are
// json:"-" by design, so they don't cross the wire today — the
// controller-side fleet.Reverse therefore composes an empty FleetPlan
// for actions whose Reverser depends on ReverseData. Surfacing those
// fields is a separate spec (R2.1c phase 2 — per-handler ReverseInfo
// registry with type discriminator).
func (w *Worker) writeKernelResult(runID string, kr *apply.KernelResult) error {
	data, err := json.MarshalIndent(kr, "", "  ")
	if err != nil {
		return err
	}
	path := w.store.ResultPath(runID)
	// Atomic write: a reader between os.WriteFile's truncate and write would
	// see an empty file and decode it as invalid JSON. Write to a sibling tmp
	// file then rename atomically so readers always see either no file (404
	// result_not_ready) or the complete JSON. The hub.Close() that terminates
	// the SSE stream happens before this function is called, so fast-polling
	// clients can race this window without the atomic write.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (w *Worker) markFailed(run *Run, errMsg string, finishedAt time.Time) {
	if _, err := w.store.Update(run.ID, func(r *Run) error {
		r.Status = StatusFailed
		r.FinishedAt = &finishedAt
		r.Error = errMsg
		return nil
	}); err != nil {
		w.log.Error("worker: mark failed", "run_id", run.ID, "err", err)
	}
}
