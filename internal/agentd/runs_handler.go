package agentd

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type submitRequest struct {
	PlanPath string `json:"plan_path"`
	Goal     string `json:"goal,omitempty"`
	BaseDir  string `json:"base_dir,omitempty"`
}

type submitResponse struct {
	RunID  string    `json:"run_id"`
	Status RunStatus `json:"status"`
}

// submitRunHandler accepts a plan submission and enqueues it on the worker.
// The daemon reads plan_path from its own filesystem view — both plan_path
// and base_dir must be absolute paths that the daemon can stat. The submitter
// is responsible for ensuring the daemon has read access (in user mode, that
// just means the user owns the file).
func (s *Server) submitRunHandler(w http.ResponseWriter, r *http.Request) {
	var req submitRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.PlanPath == "" {
		writeError(w, http.StatusBadRequest, "missing_plan_path", "plan_path is required")
		return
	}
	if !filepath.IsAbs(req.PlanPath) {
		writeError(w, http.StatusBadRequest, "relative_plan_path", "plan_path must be absolute")
		return
	}
	planPath := filepath.Clean(req.PlanPath)
	info, err := os.Stat(planPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "plan_path_not_found", err.Error())
		return
	}
	if !info.Mode().IsRegular() {
		writeError(w, http.StatusBadRequest, "plan_path_not_file", "plan_path is not a regular file")
		return
	}

	baseDir := req.BaseDir
	if baseDir == "" {
		baseDir = filepath.Dir(planPath)
	} else {
		if !filepath.IsAbs(baseDir) {
			writeError(w, http.StatusBadRequest, "relative_base_dir", "base_dir must be absolute")
			return
		}
		baseDir = filepath.Clean(baseDir)
		bi, statErr := os.Stat(baseDir)
		if statErr != nil {
			writeError(w, http.StatusBadRequest, "base_dir_not_found", statErr.Error())
			return
		}
		if !bi.IsDir() {
			writeError(w, http.StatusBadRequest, "base_dir_not_dir", "base_dir is not a directory")
			return
		}
	}

	run, err := s.store.Create(SubmitReq{
		PlanPath: planPath,
		Goal:     req.Goal,
		BaseDir:  baseDir,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_create_failed", err.Error())
		return
	}
	s.worker.Submit(run.ID)
	writeJSON(w, http.StatusAccepted, submitResponse{RunID: run.ID, Status: run.Status})
}

func (s *Server) listRunsHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := ListFilter{
		Status: RunStatus(q.Get("status")),
		Before: q.Get("before"),
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a non-negative integer")
			return
		}
		filter.Limit = n
	}
	runs, err := s.store.List(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

func (s *Server) getRunHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "run_not_found", id)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// runEventsHandler streams a run's events via SSE. Replays the persisted
// JSONL log, then tails the in-memory Hub for live runs.
//
// Replay→tail bridge: subscribe to the Hub FIRST (snapshot lastSeq), then
// stream JSONL events with seq <= lastSeq, then forward hub messages with
// seq > lastSeq. Each event carries a `seq` field assigned by the sink, so
// duplicates around the transition are filtered.
//
// Last-Event-ID header is NOT honored in v1 — reconnects start fresh. A
// client that wants to backfill can GET /v1/runs/{id}/events again.
func (s *Server) runEventsHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	run, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			writeError(w, http.StatusNotFound, "run_not_found", id)
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_run_id", err.Error())
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "ResponseWriter does not support streaming")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	// Subscribe to the hub BEFORE reading JSONL so we capture any events
	// that arrive concurrently. If the run is terminal, hub is nil and we
	// just replay JSONL.
	var hubCh <-chan HubMessage
	var unsub func()
	var snapshotSeq int64
	if !run.IsTerminal() {
		if hub := s.worker.GetHub(id); hub != nil {
			hubCh, snapshotSeq, unsub = hub.Subscribe()
			defer unsub()
		}
	}

	if err := streamJSONL(r.Context(), w, flusher, s.store.EventsPath(id)); err != nil {
		// Best effort; client may have disconnected.
		return
	}

	if hubCh == nil {
		return
	}

	// Live tail. Filter out events with seq <= snapshotSeq — those are
	// already in the JSONL we just streamed.
	for {
		select {
		case <-r.Context().Done():
			return
		case msg, ok := <-hubCh:
			if !ok {
				return
			}
			if msg.Seq <= snapshotSeq {
				continue
			}
			if err := writeSSE(w, msg.Seq, msg.Line); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// streamJSONL reads the events.jsonl line by line and emits each as an SSE
// event. The file may grow during reading (worker still appending) — we
// stop at EOF; the caller continues tailing from the hub.
func streamJSONL(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, path string) error {
	f, err := os.Open(path) //nolint:gosec // path is server-controlled (eventsPath derived from runID)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		var head struct {
			Seq int64 `json:"seq"`
		}
		_ = json.Unmarshal(line, &head)
		if err := writeSSE(w, head.Seq, line); err != nil {
			return err
		}
		flusher.Flush()
	}
	return scanner.Err()
}

// writeSSE emits one Server-Sent Event frame: id + data + blank-line
// terminator. Strips trailing newlines on the data so SSE framing isn't
// broken when the input has a trailing '\n' (the broadcast bytes do; the
// scanner-stripped JSONL bytes do not).
func writeSSE(w http.ResponseWriter, seq int64, line []byte) error {
	for len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}
	_, err := fmt.Fprintf(w, "id: %d\ndata: %s\n\n", seq, line)
	return err
}
