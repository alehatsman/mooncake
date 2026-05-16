package agentd

import (
	"encoding/json"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
)

// Worker is the single-goroutine FIFO runner of submitted plans. v1 makes no
// attempt at concurrency: concurrent applies of the same plan or of different
// plans touching the same paths/services can clobber each other. The worker
// also chdirs to the run's base_dir so executor.Start's os.Getwd()-based path
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
	defer func() {
		w.hubMu.Lock()
		delete(w.hubs, runID)
		w.hubMu.Unlock()
	}()

	sink, err := NewRunEventSink(runID, w.store.EventsPath(runID), hub, w.log)
	if err != nil {
		w.log.Error("worker: create sink", "run_id", runID, "err", err)
		w.markFailed(run, err.Error(), time.Now().UTC())
		hub.Close()
		return
	}

	publisher := events.NewPublisher()
	publisher.Subscribe(sink)
	// Also feed the global ~/.mooncake/runs.jsonl so daemon runs appear
	// alongside CLI runs.
	publisher.Subscribe(logger.NewRunLogSubscriber(run.PlanPath))

	// R2.1c: subscribe a result-capture sink so we can persist
	// result.json after the run terminates. The sink only records the
	// run.completed summary; the typed Plan + Steps tail comes from
	// the *executor.RunCapture passed to Start.
	summarySink := newDaemonSummarySink()
	publisher.Subscribe(summarySink)

	capture := &executor.RunCapture{}

	prevDir, _ := os.Getwd()
	if run.BaseDir != "" {
		if err := os.Chdir(run.BaseDir); err != nil {
			w.log.Error("worker: chdir base_dir", "run_id", runID, "base_dir", run.BaseDir, "err", err)
			publisher.Close()
			sink.Close()
			w.markFailed(run, "chdir base_dir: "+err.Error(), time.Now().UTC())
			return
		}
		defer func() { _ = os.Chdir(prevDir) }()
	}

	internalLog := logger.NewLogger(logger.ErrorLevel)
	execErr := executor.Start(executor.StartConfig{
		ConfigFilePath: run.PlanPath,
		VarsFilePaths:  run.VarsFiles,
		Tags:           run.Tags,
		Names:          run.Names,
		Capture:        capture,
	}, internalLog, publisher)

	// CRITICAL ORDER: drain publisher → close publisher → close sink → write
	// terminal record. Skipping any of this can leave events.jsonl missing
	// the tail while record.json says "success".
	publisher.Flush()
	publisher.Close()
	sink.Close()

	// R2.1c: persist the daemon's apply.KernelResult to result.json so a
	// controller-side fleet.Apply can fetch it via GET /v1/runs/{id}/result
	// and compose fleet.FleetKernelResult. Best-effort: a write failure
	// is logged but doesn't fail the run (events.jsonl + record.json are
	// the authoritative tail).
	if writeErr := w.writeResult(runID, capture, summarySink, execErr); writeErr != nil {
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

// writeResult assembles a *apply.KernelResult from the run's RunCapture
// and the summary-sink's captured run.completed data, then writes it to
// the run directory as result.json. Mirrors apply.Runner's
// assembleResult shape so the controller-side reader can unmarshal into
// the same type.
//
// Notes on the wire shape: Result.ReverseData and Result.Detail are
// json:"-" by design, so they don't cross the wire today — the
// controller-side fleet.Reverse therefore composes an empty FleetPlan
// for actions whose Reverser depends on ReverseData. Surfacing those
// fields is a separate spec (R2.1c phase 2 — per-handler ReverseInfo
// registry with type discriminator).
func (w *Worker) writeResult(runID string, capture *executor.RunCapture, summary *daemonSummarySink, execErr error) error {
	result := &apply.KernelResult{Plan: capture.Plan()}
	for _, rec := range capture.Steps() {
		result.Steps = append(result.Steps, apply.StepResult{
			Step:   rec.Step,
			Result: rec.Result,
		})
	}
	rd := summary.summary()
	result.Summary = apply.RunSummary{
		TotalSteps:   rd.TotalSteps,
		Ok:           rd.SuccessSteps,
		Changed:      rd.ChangedSteps,
		Skipped:      rd.SkippedSteps,
		Failed:       rd.FailedSteps,
		Reverted:     rd.RevertedSteps,
		DurationMs:   rd.DurationMs,
		Success:      rd.Success,
		ErrorMessage: rd.ErrorMessage,
		CheckMode:    rd.CheckMode,
	}
	// If the run never reached run.completed (catastrophic setup error),
	// reflect the actual error so consumers see something useful.
	if result.Summary.TotalSteps == 0 && execErr != nil {
		result.Summary.Success = false
		if result.Summary.ErrorMessage == "" {
			result.Summary.ErrorMessage = execErr.Error()
		}
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	path := w.store.ResultPath(runID)
	return os.WriteFile(path, data, 0o600)
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

// daemonSummarySink is the per-run events.Subscriber that records the
// run.completed event's RunCompletedData so writeResult can populate
// apply.RunSummary without round-tripping through events.jsonl. Stays
// inside agentd/ rather than reusing apply.captureSubscriber because
// (a) the daemon doesn't need the full event tail in result.json —
// /v1/runs/{id}/events serves that separately, and (b) keeping it
// here avoids exporting apply's internals.
type daemonSummarySink struct {
	mu   sync.Mutex
	data events.RunCompletedData
}

func newDaemonSummarySink() *daemonSummarySink { return &daemonSummarySink{} }

// OnEvent satisfies events.Subscriber. Only run.completed matters.
func (s *daemonSummarySink) OnEvent(e events.Event) {
	if e.Type != events.EventRunCompleted {
		return
	}
	d, ok := e.Data.(events.RunCompletedData)
	if !ok {
		return
	}
	s.mu.Lock()
	s.data = d
	s.mu.Unlock()
}

// Close satisfies events.Subscriber.
func (s *daemonSummarySink) Close() {}

func (s *daemonSummarySink) summary() events.RunCompletedData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}
