package mooncake

// backend.go — CodingBackend interface + three implementations (#145).
//
// CodingBackend is the swap seam for a coding agent's tool calls. All three
// implementations satisfy one interface; a driver changes backend with zero
// call-site modification.
//
//   MooncakeBackend — kernel-backed; reads go through queries.go (#143),
//                     mutations compile to one-step Apply (#144). Typed,
//                     gated, reversible, audited.
//
//   NativeBackend   — direct OS/exec pass-through; no kernel funnel.
//                     For A/B comparison and migration off legacy agents.
//
//   RemoteBackend   — HTTP round-trips to an agentd peer.  Reads call the
//                     MCP read_file / grep_files / glob_files tools via
//                     POST /v1/mcp; mutations PUT an inline plan YAML and
//                     submit it via POST /v1/runs, then poll for the result.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/config"
)

// CodingBackend is the execution-backend swap seam for a coding agent.
// Read / Grep / Glob are read-only queries; Edit / Write / Exec are
// mutations that carry typed Diff, Reverse capture, Policy gate, and an
// audit event on kernel-backed implementations.
type CodingBackend interface {
	// Read returns the content of path. Offset and Limit from opts are
	// honoured when non-zero.
	Read(ctx context.Context, path string, opts ReadOptions) ([]byte, error)

	// Grep searches for lines matching pattern (RE2) in files under opts.Dir.
	Grep(ctx context.Context, pattern string, opts GrepOptions) ([]Match, error)

	// Glob returns paths matching pattern, optionally rooted at opts.Dir.
	Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error)

	// Edit performs a literal string-replace on path (old → new).
	Edit(ctx context.Context, path, old, new string, opts ApplyOptions) (*ApplyResult, error)

	// Write writes content to path as a file.write step.
	Write(ctx context.Context, path string, content []byte, opts ApplyOptions) (*ApplyResult, error)

	// Exec runs cmd in the default shell.
	Exec(ctx context.Context, cmd string, opts ApplyOptions) (*ApplyResult, error)
}

// ---------------------------------------------------------------------------
// MooncakeBackend — kernel-backed
// ---------------------------------------------------------------------------

// MooncakeBackend routes reads through the SDK's direct query helpers (#143)
// and mutations through the SDK's one-step Apply helpers (#144). This is the
// canonical backend: typed, gated, reversible, audited.
type MooncakeBackend struct{}

// NewMooncakeBackend returns a MooncakeBackend that uses the local mooncake
// kernel for all operations.
func NewMooncakeBackend() *MooncakeBackend { return &MooncakeBackend{} }

func (b *MooncakeBackend) Read(_ context.Context, path string, opts ReadOptions) ([]byte, error) {
	return Read(path, opts)
}

func (b *MooncakeBackend) Grep(_ context.Context, pattern string, opts GrepOptions) ([]Match, error) {
	return Grep(pattern, opts)
}

func (b *MooncakeBackend) Glob(_ context.Context, pattern string, opts GlobOptions) ([]string, error) {
	return Glob(pattern, opts)
}

func (b *MooncakeBackend) Edit(ctx context.Context, path, old, new string, opts ApplyOptions) (*ApplyResult, error) {
	return Edit(ctx, path, old, new, opts)
}

func (b *MooncakeBackend) Write(ctx context.Context, path string, content []byte, opts ApplyOptions) (*ApplyResult, error) {
	return Write(ctx, path, content, opts)
}

func (b *MooncakeBackend) Exec(ctx context.Context, cmd string, opts ApplyOptions) (*ApplyResult, error) {
	return Exec(ctx, cmd, opts)
}

// ---------------------------------------------------------------------------
// NativeBackend — direct OS/exec pass-through
// ---------------------------------------------------------------------------

// NativeBackend is a thin pass-through to raw fs/exec with no kernel funnel.
// It is the A/B baseline: swap it in to compare latency or behavior against
// MooncakeBackend, or to migrate off a legacy agent.
//
// Mutation methods (Edit / Write / Exec) return a synthetic *ApplyResult with
// no Plan, Steps, or Events — just a Summary reflecting success or failure.
type NativeBackend struct{}

// NewNativeBackend returns a NativeBackend.
func NewNativeBackend() *NativeBackend { return &NativeBackend{} }

func (b *NativeBackend) Read(_ context.Context, path string, opts ReadOptions) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 -- intentional
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	if opts.Offset > 0 {
		if _, err := f.Seek(opts.Offset, io.SeekStart); err != nil {
			return nil, err
		}
	}
	if opts.Limit > 0 {
		buf := make([]byte, opts.Limit)
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return nil, err
		}
		return buf[:n], nil
	}
	return io.ReadAll(f)
}

func (b *NativeBackend) Grep(_ context.Context, pattern string, opts GrepOptions) ([]Match, error) {
	flags := ""
	if opts.CaseInsensitive {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, err
	}

	dir := opts.Dir
	if dir == "" {
		if dir, err = os.Getwd(); err != nil {
			return nil, err
		}
	}

	extSet := make(map[string]struct{}, len(opts.Extensions))
	for _, e := range opts.Extensions {
		extSet["."+strings.TrimPrefix(e, ".")] = struct{}{}
	}

	var matches []Match
	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if len(extSet) > 0 {
			if _, ok := extSet[filepath.Ext(path)]; !ok {
				return nil
			}
		}
		if opts.MaxResults > 0 && len(matches) >= opts.MaxResults {
			return filepath.SkipAll
		}

		f, openErr := os.Open(path) // #nosec G304 -- path from WalkDir
		if openErr != nil {
			return nil
		}
		defer func() { _ = f.Close() }()

		scanner := bufio.NewScanner(f)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			line := scanner.Text()
			if re.MatchString(line) {
				matches = append(matches, Match{Path: path, Line: lineNo, Content: line})
				if opts.MaxResults > 0 && len(matches) >= opts.MaxResults {
					return filepath.SkipAll
				}
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}
	return matches, nil
}

func (b *NativeBackend) Glob(_ context.Context, pattern string, opts GlobOptions) ([]string, error) {
	if opts.Dir != "" {
		pattern = filepath.Join(opts.Dir, pattern)
	}
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if paths == nil {
		return []string{}, nil
	}
	return paths, nil
}

func (b *NativeBackend) Write(_ context.Context, path string, content []byte, _ ApplyOptions) (*ApplyResult, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nativeFailure(err), nil //nolint:nilerr
	}
	if err := os.WriteFile(path, content, 0o644); err != nil { // #nosec G306
		return nativeFailure(err), nil //nolint:nilerr
	}
	return nativeSuccess(), nil
}

func (b *NativeBackend) Edit(_ context.Context, path, old, new string, _ ApplyOptions) (*ApplyResult, error) {
	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		return nativeFailure(err), nil //nolint:nilerr
	}
	replaced := strings.Replace(string(data), old, new, 1)
	if err := os.WriteFile(path, []byte(replaced), 0o644); err != nil { // #nosec G306
		return nativeFailure(err), nil //nolint:nilerr
	}
	return nativeSuccess(), nil
}

func (b *NativeBackend) Exec(_ context.Context, cmd string, _ ApplyOptions) (*ApplyResult, error) {
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/C", cmd) // #nosec G204
	} else {
		c = exec.Command("sh", "-c", cmd) // #nosec G204
	}
	out, err := c.CombinedOutput()
	if err != nil {
		return nativeFailureMsg(fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out)))), nil //nolint:nilerr
	}
	return nativeSuccess(), nil
}

// nativeSuccess returns a synthetic ApplyResult indicating one successful step.
func nativeSuccess() *ApplyResult {
	return &ApplyResult{
		Summary: RunSummary{
			Success:    true,
			TotalSteps: 1,
			Ok:         1,
			Changed:    1,
		},
	}
}

func nativeFailure(err error) *ApplyResult {
	return nativeFailureMsg(err.Error())
}

func nativeFailureMsg(msg string) *ApplyResult {
	return &ApplyResult{
		Summary: RunSummary{
			Success:      false,
			TotalSteps:   1,
			Failed:       1,
			ErrorMessage: msg,
		},
	}
}

// ---------------------------------------------------------------------------
// RemoteBackend — HTTP round-trips to agentd
// ---------------------------------------------------------------------------

// RemoteConfig configures a RemoteBackend.
type RemoteConfig struct {
	// BaseURL is the agentd HTTP base URL, e.g. "http://host:8080".
	BaseURL string
	// Token is the bearer auth token. Empty for unix-socket (auth-exempt)
	// connections.
	Token string
	// PollInterval controls the delay between run-status polls during
	// mutations. Defaults to 250 ms.
	PollInterval time.Duration
}

// RemoteBackend routes all operations to a remote agentd peer over HTTP.
// Reads call the MCP coding tools (read_file / grep_files / glob_files) via
// POST /v1/mcp. Mutations PUT an inline plan YAML via /v1/files, submit it
// with POST /v1/runs, poll until terminal, then fetch the typed KernelResult
// from GET /v1/runs/{id}/result.
type RemoteBackend struct {
	cfg  RemoteConfig
	http *http.Client

	mu         sync.Mutex
	syncedRoot string // cached from GET /v1/version
}

// NewRemoteBackend returns a RemoteBackend targeting the given agentd peer.
func NewRemoteBackend(cfg RemoteConfig) *RemoteBackend {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 250 * time.Millisecond
	}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext
	return &RemoteBackend{
		cfg:  cfg,
		http: &http.Client{Transport: t},
	}
}

// ---------------------------------------------------------------------------
// RemoteBackend reads
// ---------------------------------------------------------------------------

func (b *RemoteBackend) Read(ctx context.Context, path string, opts ReadOptions) ([]byte, error) {
	args := map[string]any{"path": path}
	if opts.Offset > 0 {
		args["offset"] = opts.Offset
	}
	if opts.Limit > 0 {
		args["limit"] = opts.Limit
	}
	text, err := b.callMCP(ctx, "read_file", args)
	if err != nil {
		return nil, fmt.Errorf("remote Read: %w", err)
	}
	var envelope struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, fmt.Errorf("remote Read: parse response: %w", err)
	}
	if envelope.Encoding != "base64" {
		return nil, fmt.Errorf("remote Read: unexpected encoding %q", envelope.Encoding)
	}
	data, err := base64.StdEncoding.DecodeString(envelope.Content)
	if err != nil {
		return nil, fmt.Errorf("remote Read: decode base64: %w", err)
	}
	return data, nil
}

func (b *RemoteBackend) Grep(ctx context.Context, pattern string, opts GrepOptions) ([]Match, error) {
	args := map[string]any{"pattern": pattern}
	if opts.Dir != "" {
		args["dir"] = opts.Dir
	}
	if len(opts.Extensions) > 0 {
		args["extensions"] = opts.Extensions
	}
	if opts.MaxResults > 0 {
		args["max_results"] = opts.MaxResults
	}
	if opts.CaseInsensitive {
		args["case_insensitive"] = true
	}
	text, err := b.callMCP(ctx, "grep_files", args)
	if err != nil {
		return nil, fmt.Errorf("remote Grep: %w", err)
	}
	var envelope struct {
		Matches []struct {
			Path    string `json:"path"`
			Line    int    `json:"line"`
			Content string `json:"content"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, fmt.Errorf("remote Grep: parse response: %w", err)
	}
	matches := make([]Match, len(envelope.Matches))
	for i, m := range envelope.Matches {
		matches[i] = Match{Path: m.Path, Line: m.Line, Content: m.Content}
	}
	return matches, nil
}

func (b *RemoteBackend) Glob(ctx context.Context, pattern string, opts GlobOptions) ([]string, error) {
	args := map[string]any{"pattern": pattern}
	if opts.Dir != "" {
		args["dir"] = opts.Dir
	}
	text, err := b.callMCP(ctx, "glob_files", args)
	if err != nil {
		return nil, fmt.Errorf("remote Glob: %w", err)
	}
	var envelope struct {
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal([]byte(text), &envelope); err != nil {
		return nil, fmt.Errorf("remote Glob: parse response: %w", err)
	}
	if envelope.Paths == nil {
		return []string{}, nil
	}
	return envelope.Paths, nil
}

// ---------------------------------------------------------------------------
// RemoteBackend mutations
// ---------------------------------------------------------------------------

func (b *RemoteBackend) Write(ctx context.Context, path string, content []byte, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name:      "write " + path,
		FileWrite: newFileWriteStep(path, string(content)),
	}
	return b.runStep(ctx, step, opts)
}

func (b *RemoteBackend) Edit(ctx context.Context, path, old, new string, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name:        "edit " + path,
		TextReplace: newTextReplaceStep(path, old, new),
	}
	return b.runStep(ctx, step, opts)
}

func (b *RemoteBackend) Exec(ctx context.Context, cmd string, opts ApplyOptions) (*ApplyResult, error) {
	step := Step{
		Name:  "exec",
		Shell: newShellStep(cmd),
	}
	return b.runStep(ctx, step, opts)
}

// runStep serializes one Step as a JSON plan, PUTs it to the daemon,
// submits a run, polls until terminal, and returns the typed KernelResult.
func (b *RemoteBackend) runStep(ctx context.Context, step Step, opts ApplyOptions) (*ApplyResult, error) {
	planJSON, err := b.marshalPlan(step)
	if err != nil {
		return nil, fmt.Errorf("remote runStep: marshal plan: %w", err)
	}

	syncedRoot, err := b.getSyncedRoot(ctx)
	if err != nil {
		return nil, fmt.Errorf("remote runStep: get synced root: %w", err)
	}

	scope := "sdk-remote"
	planName := fmt.Sprintf("sdk-%d.json", time.Now().UnixNano())
	if err := b.putBytes(ctx, scope, planName, planJSON); err != nil {
		return nil, fmt.Errorf("remote runStep: upload plan: %w", err)
	}
	planPath := filepath.Join(syncedRoot, scope, planName)

	runID, err := b.submitRun(ctx, planPath, opts)
	if err != nil {
		return nil, fmt.Errorf("remote runStep: submit: %w", err)
	}

	if err := b.pollUntilTerminal(ctx, runID); err != nil {
		return nil, fmt.Errorf("remote runStep: poll: %w", err)
	}

	result, err := b.getRunResult(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("remote runStep: get result: %w", err)
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// RemoteBackend HTTP helpers
// ---------------------------------------------------------------------------

const (
	remoteSmallBodyCap   int64 = 1 << 20  // 1 MiB — JSON responses
	remoteMediumBodyCap  int64 = 16 << 20 // 16 MiB — run results
	remoteRequestTimeout       = 30 * time.Second
)

// callMCP dispatches a single tools/call JSON-RPC request to POST /v1/mcp
// and returns the text content of the first content element.
func (b *RemoteBackend) callMCP(ctx context.Context, toolName string, toolArgs map[string]any) (string, error) {
	argsJSON, err := json.Marshal(toolArgs)
	if err != nil {
		return "", fmt.Errorf("callMCP %s: marshal args: %w", toolName, err)
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name":      toolName,
			"arguments": json.RawMessage(argsJSON),
		},
	}
	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("callMCP %s: marshal request: %w", toolName, err)
	}

	ctx, cancel := b.withTimeout(ctx)
	defer cancel()

	body, status, err := b.postJSON(ctx, "/v1/mcp", reqBody)
	if err != nil {
		return "", fmt.Errorf("callMCP %s: POST /v1/mcp: %w", toolName, err)
	}
	if status != http.StatusOK && status != http.StatusNoContent {
		return "", fmt.Errorf("callMCP %s: HTTP %d: %s", toolName, status, shortSnippet(body))
	}

	var rpcResp struct {
		Result *struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return "", fmt.Errorf("callMCP %s: parse JSON-RPC response: %w", toolName, err)
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("callMCP %s: RPC error %d: %s", toolName, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	if rpcResp.Result == nil || len(rpcResp.Result.Content) == 0 {
		return "", fmt.Errorf("callMCP %s: empty result", toolName)
	}
	return rpcResp.Result.Content[0].Text, nil
}

// getSyncedRoot fetches and caches the daemon's synced_root from GET /v1/version.
func (b *RemoteBackend) getSyncedRoot(ctx context.Context) (string, error) {
	b.mu.Lock()
	cached := b.syncedRoot
	b.mu.Unlock()
	if cached != "" {
		return cached, nil
	}

	ctx2, cancel := b.withTimeout(ctx)
	defer cancel()

	body, status, err := b.getJSON(ctx2, "/v1/version")
	if err != nil {
		return "", fmt.Errorf("GET /v1/version: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("GET /v1/version: HTTP %d: %s", status, shortSnippet(body))
	}
	var v struct {
		SyncedRoot string `json:"synced_root"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return "", fmt.Errorf("GET /v1/version: decode: %w", err)
	}
	if v.SyncedRoot == "" {
		return "", fmt.Errorf("GET /v1/version: empty synced_root")
	}
	b.mu.Lock()
	b.syncedRoot = v.SyncedRoot
	b.mu.Unlock()
	return v.SyncedRoot, nil
}

// putBytes uploads data to PUT /v1/files under (scope, relPath).
func (b *RemoteBackend) putBytes(ctx context.Context, scope, relPath string, data []byte) error {
	u := b.cfg.BaseURL + "/v1/files?" + url.Values{
		"scope": {scope},
		"path":  {relPath},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.ContentLength = int64(len(data))
	req.Header.Set("Content-Type", "application/octet-stream")
	b.setAuth(req)

	resp, err := b.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("PUT /v1/files: HTTP %d", resp.StatusCode)
	}
	return nil
}

type remoteSubmitReq struct {
	PlanPath string `json:"plan_path"`
}

type remoteSubmitResp struct {
	RunID string `json:"run_id"`
}

// submitRun sends a plan path to POST /v1/runs and returns the run ID.
func (b *RemoteBackend) submitRun(ctx context.Context, planPath string, _ ApplyOptions) (string, error) {
	ctx2, cancel := b.withTimeout(ctx)
	defer cancel()

	payload, _ := json.Marshal(remoteSubmitReq{PlanPath: planPath})
	body, status, err := b.postJSON(ctx2, "/v1/runs", payload)
	if err != nil {
		return "", fmt.Errorf("POST /v1/runs: %w", err)
	}
	if status != http.StatusAccepted {
		return "", fmt.Errorf("POST /v1/runs: HTTP %d: %s", status, shortSnippet(body))
	}
	var sr remoteSubmitResp
	if err := json.Unmarshal(body, &sr); err != nil {
		return "", fmt.Errorf("POST /v1/runs: decode: %w", err)
	}
	if sr.RunID == "" {
		return "", fmt.Errorf("POST /v1/runs: empty run_id")
	}
	return sr.RunID, nil
}

// pollUntilTerminal polls GET /v1/runs/{id} until the run reaches a terminal
// state or ctx is cancelled.
func (b *RemoteBackend) pollUntilTerminal(ctx context.Context, runID string) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(b.cfg.PollInterval):
		}

		pctx, cancel := b.withTimeout(ctx)
		body, status, err := b.getJSON(pctx, "/v1/runs/"+runID)
		cancel()
		if err != nil {
			return fmt.Errorf("GET /v1/runs/%s: %w", runID, err)
		}
		if status != http.StatusOK {
			return fmt.Errorf("GET /v1/runs/%s: HTTP %d: %s", runID, status, shortSnippet(body))
		}
		var rec struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(body, &rec); err != nil {
			return fmt.Errorf("GET /v1/runs/%s: decode: %w", runID, err)
		}
		if isTerminalStatus(rec.Status) {
			return nil
		}
	}
}

// isTerminalStatus reports whether a run status string is terminal.
func isTerminalStatus(s string) bool {
	switch s {
	case "success", "failed", "interrupted":
		return true
	}
	return false
}

// getRunResult fetches the full KernelResult from GET /v1/runs/{id}/result.
func (b *RemoteBackend) getRunResult(ctx context.Context, runID string) (*ApplyResult, error) {
	ctx2, cancel := b.withTimeout(ctx)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx2, http.MethodGet,
		b.cfg.BaseURL+"/v1/runs/"+runID+"/result", nil)
	if err != nil {
		return nil, err
	}
	b.setAuth(req)

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET /v1/runs/%s/result: %w", runID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, remoteMediumBodyCap))
	if err != nil {
		return nil, fmt.Errorf("GET /v1/runs/%s/result: read body: %w", runID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET /v1/runs/%s/result: HTTP %d: %s", runID, resp.StatusCode, shortSnippet(body))
	}
	var result apply.KernelResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("GET /v1/runs/%s/result: decode: %w", runID, err)
	}
	return &result, nil
}

// marshalPlan serialises one step as a single-step JSON plan that the daemon
// can compile. Returns the raw JSON bytes.
func (b *RemoteBackend) marshalPlan(step Step) ([]byte, error) {
	cfg := &Config{
		Version: "1.0",
		Steps:   []Step{step},
	}
	return json.Marshal(cfg)
}

// postJSON does POST base + path with body and returns (responseBody, statusCode, error).
func (b *RemoteBackend) postJSON(ctx context.Context, path string, body []byte) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		b.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.ContentLength = int64(len(body))
	b.setAuth(req)

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, remoteSmallBodyCap+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return respBody, resp.StatusCode, nil
}

// getJSON does GET base + path and returns (responseBody, statusCode, error).
func (b *RemoteBackend) getJSON(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.cfg.BaseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	b.setAuth(req)

	resp, err := b.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, remoteSmallBodyCap+1))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (b *RemoteBackend) setAuth(req *http.Request) {
	if b.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.cfg.Token)
	}
}

func (b *RemoteBackend) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, remoteRequestTimeout)
}

func shortSnippet(body []byte) string {
	s := strings.TrimSpace(string(body))
	if len(s) > 256 {
		return s[:256] + "…"
	}
	return s
}

// ---------------------------------------------------------------------------
// Step construction helpers — shared by RemoteBackend and NativeBackend
// ---------------------------------------------------------------------------

func newFileWriteStep(path, content string) *config.File {
	return &config.File{Path: path, State: "file", Content: content}
}

func newTextReplaceStep(path, old, new string) *config.FileReplace {
	noRegex := false
	return &config.FileReplace{
		Path:    path,
		Pattern: old,
		Replace: new,
		Flags:   &config.ReplaceFlags{Regex: noRegex},
	}
}

func newShellStep(cmd string) *config.ShellAction {
	return &config.ShellAction{Cmd: cmd}
}
