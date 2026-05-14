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
	"strings"
	"time"
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
	Version    string `json:"version"`
	DaemonPID  int    `json:"daemon_pid"`
	Hostname   string `json:"hostname"`
	SyncedRoot string `json:"synced_root"`
	SystemMode bool   `json:"system_mode"`
}

// GetVersion fetches /v1/version. Used by the controller to learn the
// peer's SyncedRoot (needed to compute the plan_path it later submits) and
// for liveness checks in spec-42's fleet status.
func (c *Client) GetVersion(ctx context.Context) (*Version, error) {
	ctx, cancel := withTimeout(ctx)
	defer cancel()

	req, err := c.authReq(ctx, http.MethodGet, c.BaseURL+"/v1/version", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, c.wrap("GET /v1/version", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := readSmallBody(resp)
	if err != nil {
		return nil, c.wrap("GET /v1/version: read body", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, c.httpErr("GET /v1/version", resp.StatusCode, body)
	}
	var v Version
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, c.wrap("GET /v1/version: decode", err)
	}
	return &v, nil
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

// SubmitRequest mirrors the daemon's submitRequest JSON shape. PlanPath
// must be an absolute path on the daemon's filesystem (typically under
// `<synced_root>/<scope>/...`); same for VarsFiles and BaseDir.
type SubmitRequest struct {
	PlanPath  string   `json:"plan_path"`
	VarsFiles []string `json:"vars_files,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Goal      string   `json:"goal,omitempty"`
	BaseDir   string   `json:"base_dir,omitempty"`
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
	return fmt.Errorf("peer %s: %s: %w", c.Name, op, err)
}
