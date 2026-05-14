package agentd

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/metrics"
)

// Body size limit for POST /v1/mcp. MCP requests are small structured
// JSON-RPC calls; anything past 1 MiB is almost certainly malformed.
const maxMCPBodyBytes = 1 << 20

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type versionResponse struct {
	Version     string `json:"version"`
	DaemonPID   int    `json:"daemon_pid"`
	UptimeSec   int64  `json:"uptime_sec"`
	SystemMode  bool   `json:"system_mode"`
	QueueDepth  int    `json:"queue_depth"`
	RunsRunning int    `json:"runs_running"`
	// Hostname is the daemon's OS hostname, cached at startup. Used by the
	// controller's discovery / status flows to identify which peer is which
	// without an extra round-trip.
	Hostname string `json:"hostname"`
	// SyncedRoot is the absolute path under which PUT /v1/files writes scope
	// subtrees (`<state_dir>/synced`). The controller needs this to build
	// the `plan_path` it submits to POST /v1/runs.
	SyncedRoot string `json:"synced_root"`
}

func (s *Server) versionHandler(w http.ResponseWriter, _ *http.Request) {
	queued, running := s.worker.Stats()
	writeJSON(w, http.StatusOK, versionResponse{
		Version:     s.version,
		DaemonPID:   os.Getpid(),
		UptimeSec:   int64(time.Since(s.startedAt).Seconds()),
		SystemMode:  s.cfg.SystemMode,
		QueueDepth:  queued,
		RunsRunning: running,
		Hostname:    s.hostname,
		SyncedRoot:  s.cfg.SyncedRoot(),
	})
}

func factsHandler(w http.ResponseWriter, _ *http.Request) {
	f := facts.Collect()
	writeJSON(w, http.StatusOK, f.ToMap())
}

// metricsHandler mirrors the `mooncake metrics` CLI response shape:
//   - no fields  → full metrics map at the top level
//   - fields=... → only the requested top-level keys, plus `_collected_at`
// ?refresh=true bypasses the TTL cache.
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("refresh") == "true" {
		metrics.Refresh()
	}

	fields := parseFieldsParam(r.URL.Query().Get("fields"))

	m, collectedAt, collectErr := metrics.Collect(fields)
	mm := m.ToMap()

	payload := map[string]any{}
	if len(fields) > 0 {
		for _, k := range fields {
			payload[k] = mm[k]
		}
		payload["_collected_at"] = formatCollectedAt(collectedAt)
	} else {
		for k, v := range mm {
			payload[k] = v
		}
	}
	if collectErr != nil {
		payload["_warning"] = collectErr.Error()
	}

	writeJSON(w, http.StatusOK, payload)
}

func parseFieldsParam(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, f := range strings.Split(raw, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

func formatCollectedAt(ts map[string]time.Time) map[string]string {
	out := make(map[string]string, len(ts))
	for k, v := range ts {
		out[k] = v.UTC().Format(time.RFC3339)
	}
	return out
}

// mcpHandler accepts a single JSON-RPC 2.0 request body, dispatches it on the
// shared MCP server, and writes the JSON response. Stateless: each POST is one
// RPC. No SSE / batching yet — `tools/list` and `tools/call` are enough for
// every MCP client we care about today.
//
// Concurrency note: read-only methods (tools/list, initialize, ping, get_facts,
// get_metrics, fact_query, get_snapshot, check_plan) are safe to invoke
// concurrently. `run_plan` is NOT — it calls executor.Start, which reads the
// process working dir. Slice 3 will route run_plan through the same worker
// queue as POST /v1/runs to eliminate the dual code path; until then, treat
// concurrent run_plan calls as undefined behavior (the existing stdio MCP
// transport has the same property).
func (s *Server) mcpHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxMCPBodyBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	resp, err := s.mcp.DispatchBytes(r.Context(), body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "mcp_dispatch_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(resp)
}
