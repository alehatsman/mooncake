package agentd

import (
	"log/slog"
	"os"
	"sync"
	"time"

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

// Submit enqueues a run for execution. Non-blocking up to the channel buffer.
func (w *Worker) Submit(runID string) {
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

	hub := NewHub()
	w.hubMu.Lock()
	w.hubs[runID] = hub
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
	}, internalLog, publisher)

	// CRITICAL ORDER: drain publisher → close publisher → close sink → write
	// terminal record. Skipping any of this can leave events.jsonl missing
	// the tail while record.json says "success".
	publisher.Flush()
	publisher.Close()
	sink.Close()

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
