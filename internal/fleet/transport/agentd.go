// Package transport implements the controller-side HTTP+SSE client for
// talking to a single agentd peer. It is intentionally standalone — it does
// not import internal/fleet — so cycles are impossible and tests can
// construct a Client directly without a peers.toml.
package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/apply"
)

// defaultRequestTimeout caps non-SSE round trips. SSE is unbounded by
// design (it streams events as long as the run is active).
const defaultRequestTimeout = 30 * time.Second

// maxJSONResponseBytes bounds the response body for the small JSON
// endpoints (Version, Submit). 1 MiB is generous.
const maxJSONResponseBytes int64 = 1 << 20

// Client is the controller-side handle to a single agentd peer.
type Client struct {
	// Name is the peer's name from peers.toml. Used in error messages to
	// make the source of failures unambiguous when N clients run in
	// parallel.
	Name string

	// BaseURL is the peer's `http://host:port` prefix.
	BaseURL string

	// Token is the bearer for the Authorization header.
	Token string

	http *http.Client
}

// New builds a Client for a peer identified by (name, host:port, token).
// HTTP client has a 10s connect timeout; per-call deadlines arrive via
// context.
func New(name, addr, token string) *Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	return &Client{
		Name:    name,
		BaseURL: "http://" + addr,
		Token:   token,
		http:    &http.Client{Transport: t},
	}
}

// SetHTTPClient swaps the underlying http.Client. Intended for tests.
func (c *Client) SetHTTPClient(h *http.Client) { c.http = h }

// authReq returns a request with the bearer header set. Use this instead of
// http.NewRequestWithContext directly to keep auth wiring in one place.
func (c *Client) authReq(ctx context.Context, method, fullURL string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	return req, nil
}

// withTimeout layers a default timeout on ctx when ctx has no deadline.
// SSE callers should NOT use this; they want to inherit only the caller's
// cancellation, not a deadline.
func withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, defaultRequestTimeout)
}

// readSmallBody reads a bounded JSON response body. Returns the bytes (for
// error messages) or an error if the body exceeded the limit.
func readSmallBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, maxJSONResponseBytes+1))
}

// Version is a subset of agentd's /v1/version response covering the fields
// the controller needs.
type Version struct {
	Version     string `json:"version"`
	DaemonPID   int    `json:"daemon_pid"`
	Hostname    string `json:"hostname"`
	SyncedRoot  string `json:"synced_root"`
	SystemMode  bool   `json:"system_mode"`
	UptimeSec   int64  `json:"uptime_sec"`
	QueueDepth  int    `json:"queue_depth"`
	RunsRunning int    `json:"runs_running"`
	// OS is runtime.GOOS on the peer ("darwin", "linux", "windows",
	// "freebsd", ...). Used by spec-50's `--peer-filter os=<x>` evaluator
	// in cmd/fleet.go. Empty on daemons predating the spec-50 change; the
	// evaluator treats that as ok=false (predicate fails + warning).
	OS string `json:"os"`
}

// GetVersion fetches /v1/version. Used by the controller to learn the
// peer's SyncedRoot (needed to compute the plan_path it later submits) and
// for liveness checks in spec-46's fleet status.
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	var v Version
	if err := getJSON(ctx, c, "GET /v1/version", "/v1/version", &v); err != nil {
		return nil, err
	}
	return &v, nil
}

// getJSON is the shared GET-small-JSON helper for endpoints that
// return a typed JSON body on 200 and a structured error on
// anything else. Wraps with-timeout-context, authReq, http.Do, the
// 1 MiB body cap, status check, and json.Unmarshal in one place so
// the per-endpoint methods become ~5-line one-liners.
//
// opLabel ("GET /v1/version", "GET /v1/self/mac", …) is the prefix
// used for c.wrap()/c.httpErr() so error messages keep their
// existing wire shape. Endpoints that need special status handling
// (e.g. GetRunResult's 404 → ErrRunResultNotReady) or a different
// body cap (GetRunResult uses readMediumBody) stay open-coded.
func getJSON[T any](ctx context.Context, c *Client, opLabel, path string, out *T) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	req, err := c.authReq(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return c.wrap(opLabel, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readSmallBody(resp)
	if err != nil {
		return c.wrap(opLabel+": read body", err)
	}
	if resp.StatusCode != http.StatusOK {
		return c.httpErr(opLabel, resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return c.wrap(opLabel+": decode", err)
	}
	return nil
}

// Head reports whether the peer already has a byte-identical copy of the
// file under (scope, relPath) matching the given sha256 hex. Returns
// (true, nil) on cache-hit, (false, nil) on miss, error on transport
// failures.
func (c *Client) Head(ctx context.Context, scope, relPath, sha256 string) (bool, error) {
	if sha256 == "" {
		return false, errors.New("Head: sha256 is required")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	u := c.BaseURL + "/v1/files?" + url.Values{
		"scope":  {scope},
		"path":   {relPath},
		"sha256": {sha256},
	}.Encode()
	req, err := c.authReq(ctx, http.MethodHead, u, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, c.wrap("HEAD /v1/files", err)
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, c.httpErr("HEAD /v1/files", resp.StatusCode, nil)
	}
}

// Put streams srcPath as the body of PUT /v1/files for (scope, relPath).
// When sha256 != "" the daemon verifies it via X-Sha256 and returns 422 on
// mismatch; that becomes a non-nil error here.
//
// Streams the file directly rather than buffering — keeps memory bounded
// regardless of plan-dir size. Caller is responsible for ensuring the file
// fits within the daemon's MaxSyncBytes; on overflow the daemon returns
// 413 and we surface that as an error.
func (c *Client) Put(ctx context.Context, scope, relPath, srcPath, sha256 string) error {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	f, err := os.Open(srcPath)
	if err != nil {
		return c.wrap("PUT /v1/files: open "+srcPath, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return c.wrap("PUT /v1/files: stat "+srcPath, err)
	}

	u := c.BaseURL + "/v1/files?" + url.Values{
		"scope": {scope},
		"path":  {relPath},
	}.Encode()
	req, err := c.authReq(ctx, http.MethodPut, u, f)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	if sha256 != "" {
		req.Header.Set("X-Sha256", sha256)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return c.wrap("PUT /v1/files", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	body, _ := readSmallBody(resp)
	return c.httpErr(fmt.Sprintf("PUT /v1/files scope=%s path=%s", scope, relPath), resp.StatusCode, body)
}

// RunRecord mirrors the daemon's persisted Run record (a subset of fields
// the controller actually consumes). Used by GetRun to read the final state
// of a run that may have failed before emitting events.
type RunRecord struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	PlanPath   string `json:"plan_path"`
	QueuedAt   string `json:"queued_at"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// GetRun fetches the run record from /v1/runs/{id}. Used by Apply after
// Stream returns to recover the run's final status when no run.completed
// event was emitted (e.g. planner setup failure before any events fire).
func (c *Client) GetRun(ctx context.Context, runID string) (*RunRecord, error) {
	if runID == "" {
		return nil, errors.New("GetRun: runID is empty")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	req, err := c.authReq(ctx, http.MethodGet, c.BaseURL+"/v1/runs/"+runID, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID+": read body", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.httpErr("GET /v1/runs/"+runID, resp.StatusCode, body)
	}
	var rec RunRecord
	if err := json.Unmarshal(body, &rec); err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID+": decode", err)
	}
	return &rec, nil
}

// GetRunResult fetches the daemon's apply.KernelResult for a terminal
// run from /v1/runs/{id}/result. Used by fleet.Apply (R2.1c) to surface
// per-peer KernelResults up to fleet.KernelResult so the
// fleet-scope Reverse() composition has typed Steps to walk.
//
// Returns ErrRunResultNotReady when the daemon responds 404
// result_not_ready (run hasn't reached terminal state). Returns the
// standard wrapped httpErr for other 4xx/5xx.
//
// Wire shape: matches internal/apply.KernelResult exactly. Result.Detail
// stays json:"-" (plan-time only — meaningless for already-applied
// steps). Result.ReverseData rides through executor.Result's custom
// MarshalJSON / UnmarshalJSON as a discriminator-tagged envelope
// (R2.1c phase 2 landed), so handlers that need pre-apply state for
// Reverse() see a fully restored *ReverseInfo on the controller side.
// Mixed-version fleets remain safe: an unknown discriminator decodes
// to ReverseData=nil and the handler's existing "no ReverseData
// captured" refusal surfaces — no panic, no silent corruption.
func (c *Client) GetRunResult(ctx context.Context, runID string) (*apply.KernelResult, error) {
	if runID == "" {
		return nil, errors.New("GetRunResult: runID is empty")
	}
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	req, err := c.authReq(ctx, http.MethodGet, c.BaseURL+"/v1/runs/"+runID+"/result", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID+"/result", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readMediumBody(resp)
	if err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID+"/result: read body", err)
	}
	if resp.StatusCode == http.StatusNotFound && bodyMentions(body, "result_not_ready") {
		return nil, ErrRunResultNotReady
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.httpErr("GET /v1/runs/"+runID+"/result", resp.StatusCode, body)
	}
	var result apply.KernelResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, c.wrap("GET /v1/runs/"+runID+"/result: decode", err)
	}
	return &result, nil
}

// ErrRunResultNotReady is the sentinel returned by GetRunResult when
// the run hasn't reached a terminal state yet (result.json isn't on
// disk). Callers should poll /v1/runs/{id} until terminal, then retry.
var ErrRunResultNotReady = errors.New("fleet/transport: run result not ready (run not terminal)")

// readMediumBody is GetRunResult's body reader. Result JSON can run
// into hundreds of KB for plans with many steps; readSmallBody's
// existing cap is too tight.
func readMediumBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, 16<<20)) // 16 MiB cap
}

// bodyMentions reports whether a JSON error response body contains the
// given error-code substring. Used to distinguish the daemon's two 404
// shapes (run_not_found vs result_not_ready) without redesigning the
// error envelope.
func bodyMentions(body []byte, needle string) bool {
	return bytes.Contains(body, []byte(needle))
}

// ListRunsOpts is the extended filter shape for /v1/runs (spec-54).
// All fields are optional; empty values send no query parameter.
type ListRunsOpts struct {
	// Status filters to one of "running", "queued", "success",
	// "failed", "interrupted". Empty means no filter (all statuses).
	Status string
	// Limit caps how many records the daemon returns. <=0 means "use
	// the daemon's default".
	Limit int
	// Before is a cursor (run id or RFC3339 timestamp — the daemon
	// decides) for pagination; empty means "newest".
	Before string
}

// ListRuns fetches recent run records from /v1/runs. Returns the runs
// newest-first. Used by `fleet status` and `fleet ps`; spec-46 + spec-54.
//
// limit <= 0 sends no `limit` query and asks the daemon for its default.
// In practice callers want 1 (most recent) or a small bounded window.
func (c *Client) ListRuns(ctx context.Context, limit int) ([]RunRecord, error) {
	return c.ListRunsWith(ctx, ListRunsOpts{Limit: limit})
}

// ListRunsWith is the extended-filter variant of ListRuns (spec-54).
// Splits the API so spec-46's call site stays terse and the new
// `fleet ps` paths can supply Status + Before without a positional
// arg explosion.
func (c *Client) ListRunsWith(ctx context.Context, opts ListRunsOpts) ([]RunRecord, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	u := c.BaseURL + "/v1/runs"
	q := url.Values{}
	if opts.Status != "" {
		q.Set("status", opts.Status)
	}
	if opts.Limit > 0 {
		q.Set("limit", strconv.Itoa(opts.Limit))
	}
	if opts.Before != "" {
		q.Set("before", opts.Before)
	}
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := c.authReq(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("GET /v1/runs", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("GET /v1/runs: read body", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.httpErr("GET /v1/runs", resp.StatusCode, body)
	}
	// Daemon shape: {"runs": [...]}. The records have more fields than
	// RunRecord exposes (tags, vars_files, base_dir, daemon_pid). They
	// decode into the subset we care about.
	var resp2 struct {
		Runs []RunRecord `json:"runs"`
	}
	if err := json.Unmarshal(body, &resp2); err != nil {
		return nil, c.wrap("GET /v1/runs: decode", err)
	}
	return resp2.Runs, nil
}

// GetFacts fetches /v1/facts. Returns the raw facts map — caller picks
// the keys it needs. The daemon doesn't yet support ?fields= filtering,
// so we transfer the full payload (typically a few KB) and let the caller
// cherry-pick os / arch / os_version / etc.
func (c *Client) GetFacts(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	if err := getJSON(ctx, c, "GET /v1/facts", "/v1/facts", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// SubmitRequest mirrors the daemon's submitRequest JSON shape. PlanPath
// must be an absolute path on the daemon's filesystem (typically under
// `<synced_root>/<scope>/...`); same for VarsFiles and BaseDir.
type SubmitRequest struct {
	PlanPath  string   `json:"plan_path"`
	VarsFiles []string `json:"vars_files,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	// Names is the spec-50 step-name filter; AND'd with Tags by the
	// daemon-side planner.
	Names   []string `json:"names,omitempty"`
	Goal    string   `json:"goal,omitempty"`
	BaseDir string   `json:"base_dir,omitempty"`
}

// Submit POSTs a run request and returns the run ID assigned by the daemon.
// The caller then uses Stream(runID) to read events.
func (c *Client) Submit(ctx context.Context, req SubmitRequest) (runID string, err error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	payload, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal submit: %w", err)
	}

	httpReq, err := c.authReq(ctx, http.MethodPost, c.BaseURL+"/v1/runs", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.ContentLength = int64(len(payload))

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return "", c.wrap("POST /v1/runs", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readSmallBody(resp)
	if err != nil {
		return "", c.wrap("POST /v1/runs: read body", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return "", c.httpErr("POST /v1/runs", resp.StatusCode, body)
	}

	var sr struct {
		RunID  string `json:"run_id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", c.wrap("POST /v1/runs: decode", err)
	}
	if sr.RunID == "" {
		return "", errors.New("POST /v1/runs: empty run_id in response")
	}
	return sr.RunID, nil
}

// httpErr builds an error from a non-success response. Includes the peer
// name, the operation, the status code, and a short snippet of the body
// (best-effort decoded as the standard apiError shape).
func (c *Client) httpErr(op string, status int, body []byte) error {
	snippet := strings.TrimSpace(string(body))
	if len(snippet) > 256 {
		snippet = snippet[:256] + "…"
	}
	// Try to extract the structured error code+message.
	var ae struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &ae) == nil && ae.Error != "" {
		return fmt.Errorf("peer %s: %s: HTTP %d: %s (%s)",
			c.Name, op, status, ae.Error, ae.Message)
	}
	if snippet == "" {
		return fmt.Errorf("peer %s: %s: HTTP %d", c.Name, op, status)
	}
	return fmt.Errorf("peer %s: %s: HTTP %d: %s", c.Name, op, status, snippet)
}

func (c *Client) wrap(op string, err error) error {
	if cause := classifyNetErr(err); cause != "" {
		return fmt.Errorf("peer %s: %s: %s: %w", c.Name, op, cause, err)
	}
	return fmt.Errorf("peer %s: %s: %w", c.Name, op, err)
}

// UploadBinaryResponse mirrors the daemon's 202 JSON from
// PUT /v1/self/binary.
type UploadBinaryResponse struct {
	StagedPath string `json:"staged_path"`
	SHA256     string `json:"sha256"`
}

// UploadBinary streams a candidate replacement binary to the peer and
// returns the staged-file metadata. The daemon hashes the body, refuses
// on sha/os/arch mismatch, and runs `<staged> --version` before
// confirming. binSHA256 must be the hex-encoded sha256 of the bytes at
// binPath; the daemon verifies it, so a mismatch is caller-side
// corruption.
//
// targetOS / targetArch tell the daemon what flavour of binary the
// controller built — typically `runtime.GOOS` / `runtime.GOARCH` of the
// peer, learned via GetVersion. The daemon rejects with HTTP 400 when
// either disagrees with its own runtime values.
func (c *Client) UploadBinary(ctx context.Context, binPath, binSHA256, targetOS, targetArch string) (*UploadBinaryResponse, error) {
	if binSHA256 == "" {
		return nil, errors.New("UploadBinary: sha256 is required")
	}
	if targetOS == "" || targetArch == "" {
		return nil, errors.New("UploadBinary: targetOS and targetArch are required")
	}

	// Uploads can be slow on a busy LAN. Bump the per-request deadline
	// for this one call so a 25 MiB push over WiFi doesn't trip the
	// default 30s cap.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	f, err := os.Open(binPath)
	if err != nil {
		return nil, c.wrap("PUT /v1/self/binary: open "+binPath, err)
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, c.wrap("PUT /v1/self/binary: stat", err)
	}

	req, err := c.authReq(ctx, http.MethodPut, c.BaseURL+"/v1/self/binary", f)
	if err != nil {
		return nil, err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Mooncake-Binary-SHA256", binSHA256)
	req.Header.Set("X-Mooncake-Binary-OS", targetOS)
	req.Header.Set("X-Mooncake-Binary-Arch", targetArch)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("PUT /v1/self/binary", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("PUT /v1/self/binary: read body", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, c.httpErr("PUT /v1/self/binary", resp.StatusCode, body)
	}
	var out UploadBinaryResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, c.wrap("PUT /v1/self/binary: decode", err)
	}
	return &out, nil
}

// MACResponse mirrors the daemon's GET /v1/self/mac body.
type MACResponse struct {
	MAC       string `json:"mac"`
	Interface string `json:"interface"`
}

// GetMAC fetches the peer's hardware address — specifically, the MAC
// of the interface that owns the inbound TCP connection's local IP,
// which is the MAC the controller wants for Wake-on-LAN. Returns the
// daemon's 404 "no_mac" as a wrapped error when the peer has no
// usable non-loopback NIC.
func (c *Client) GetMAC(ctx context.Context) (*MACResponse, error) {
	var out MACResponse
	if err := getJSON(ctx, c, "GET /v1/self/mac", "/v1/self/mac", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ShutdownResponse mirrors the daemon's 202 JSON from
// POST /v1/self/shutdown.
type ShutdownResponse struct {
	DaemonPID      int `json:"daemon_pid"`
	ScheduledInSec int `json:"scheduled_in_sec"`
}

// Shutdown asks the peer to power off. Returns 202 BEFORE the
// shutdown actually runs (mirrors SelfReplace's pre-exec ack pattern)
// — the caller should not expect any liveness signal from the peer
// afterwards. The daemon refuses with 409 runs_in_flight when an
// active run is in progress unless force is true.
func (c *Client) Shutdown(ctx context.Context, force bool) (*ShutdownResponse, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()
	body, err := json.Marshal(struct {
		Force bool `json:"force,omitempty"`
	}{force})
	if err != nil {
		return nil, c.wrap("POST /v1/self/shutdown: marshal", err)
	}
	req, err := c.authReq(ctx, http.MethodPost, c.BaseURL+"/v1/self/shutdown", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("POST /v1/self/shutdown", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("POST /v1/self/shutdown: read body", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, c.httpErr("POST /v1/self/shutdown", resp.StatusCode, respBody)
	}
	var out ShutdownResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, c.wrap("POST /v1/self/shutdown: decode", err)
	}
	return &out, nil
}

// SelfReplaceResponse mirrors the daemon's 202 JSON from
// POST /v1/self/replace.
type SelfReplaceResponse struct {
	OldPID     int    `json:"old_pid"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

// SelfReplace tells the peer to swap its on-disk binary with the
// previously-uploaded staged file and re-exec. The HTTP response
// returns BEFORE the actual exec — the caller is expected to poll
// /v1/version afterwards and confirm `daemon_pid` changed. `force` lets
// the operator override the daemon's "runs in flight" guard.
func (c *Client) SelfReplace(ctx context.Context, stagedPath, sha256 string, force bool) (*SelfReplaceResponse, error) {
	if stagedPath == "" || sha256 == "" {
		return nil, errors.New("SelfReplace: stagedPath and sha256 are required")
	}

	ctx, cancel := withTimeout(ctx)
	defer cancel()

	body, err := json.Marshal(struct {
		StagedPath string `json:"staged_path"`
		SHA256     string `json:"sha256"`
		Force      bool   `json:"force,omitempty"`
	}{stagedPath, sha256, force})
	if err != nil {
		return nil, c.wrap("POST /v1/self/replace: marshal", err)
	}

	req, err := c.authReq(ctx, http.MethodPost, c.BaseURL+"/v1/self/replace", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("POST /v1/self/replace", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("POST /v1/self/replace: read body", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return nil, c.httpErr("POST /v1/self/replace", resp.StatusCode, respBody)
	}
	var out SelfReplaceResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, c.wrap("POST /v1/self/replace: decode", err)
	}
	return &out, nil
}
