// Package http_request implements the proposal-16 http.request action:
// a first-class HTTP call with body, headers, auth, retries, response
// captured as a registered fact, plan/apply branching, and an
// idempotency contract enforced at Validate-time.
//
// See docs-working/streams/core/proposals/proposal-16-http-request-action.md
// for the full design. Wave 1 ships:
//
//   - all standard methods (GET/HEAD/OPTIONS/POST/PUT/PATCH/DELETE)
//   - body forms: raw `body:`, structured `json:`, urlencoded `form:`,
//     file-bytes `file:` (one-of)
//   - auth forms: bearer / basic / arbitrary header (one-of)
//   - idempotency contract (POST/PATCH require IdempotencyKey,
//     CreatesWhen, or Risk="high")
//   - expect_status validation (default 2xx)
//   - retries with retry_on classification (5xx/4xx/429/timeout/
//     connection_error) and retry_delay
//   - timeout (per-request transfer)
//   - secret redaction for auth headers + opt-in body redaction
//   - response capture as a registered fact (status_code, headers, body,
//     json (auto-parsed), duration_ms, url, attempts)
//   - emits events.EventHTTPRequested (host-only URL, no body)
//   - plan mode: skip network; WouldChange=false for read methods
//     (GET/HEAD/OPTIONS), true for write methods
//
// Waves 2-3 add: plan-mode GET probes (probe:), reverse compensation
// (reverse:), expect_json_schema, save_to.
package http_request

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/httputil"
)

const (
	actionName = "http.request"

	defaultTimeout          = 30 * time.Second
	defaultRetryDelay       = time.Second
	defaultMaxResponseBytes = 1 << 20 // 1 MiB
	defaultFollowRedirects  = 10
)

// readMethods are HTTP verbs that don't (by spec) modify server state.
// In plan mode we report WouldChange=false; risk classification defaults
// to "low".
var readMethods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodOptions: true,
}

// Handler implements the http.request action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata. Network category, supports plan,
// no sudo.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Issue an HTTP request; capture the response as a registered fact",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventHTTPRequested)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    true,
	}
}

// Permissions: Network always. FilesystemWrite only if save_to is set
// (Wave 3). Sudo never; become is rejected by Validate.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{Network: true}
	if step == nil || step.HTTPRequest == nil {
		return ps
	}
	// Wave 3 will add FilesystemWrite=[save_to] here.
	return ps
}

// Validate enforces the structural contract: URL required, one-of body,
// one-of auth, supported method, idempotency contract for POST/PATCH,
// valid retry_on tokens, parseable durations, valid risk.
func (Handler) Validate(step *config.Step) error {
	if step == nil || step.HTTPRequest == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	r := step.HTTPRequest

	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("%s: url is required", actionName)
	}

	method, err := normalizeMethod(r.Method)
	if err != nil {
		return err
	}

	if err := validateBodyOneOf(r); err != nil {
		return err
	}
	if err := validateAuthOneOf(r); err != nil {
		return err
	}

	if err := validateIdempotency(method, r); err != nil {
		return err
	}

	if err := validateRetryOn(r.RetryOn); err != nil {
		return err
	}
	if err := validateDuration("timeout", r.Timeout); err != nil {
		return err
	}
	if err := validateDuration("retry_delay", r.RetryDelay); err != nil {
		return err
	}
	if r.Retries < 0 {
		return fmt.Errorf("%s: retries must be >= 0", actionName)
	}
	for _, code := range r.ExpectStatus {
		if code < 100 || code > 599 {
			return fmt.Errorf("%s: invalid expect_status %d", actionName, code)
		}
	}
	if r.Risk != "" && r.Risk != "low" && r.Risk != "medium" && r.Risk != "high" {
		return fmt.Errorf("%s: risk must be one of low|medium|high (got %q)", actionName, r.Risk)
	}
	if r.MaxResponseBytes < 0 {
		return fmt.Errorf("%s: max_response_bytes must be >= 0", actionName)
	}

	// HTTP-with-sudo is nonsense; surface as a clear error rather than
	// silently honoring it.
	if step.AsUser != "" {
		return fmt.Errorf("%s: as_user is not supported (HTTP calls never run as another user)", actionName)
	}

	return nil
}

// normalizeMethod uppercases + validates the method. Default GET.
func normalizeMethod(in string) (string, error) {
	m := strings.ToUpper(strings.TrimSpace(in))
	if m == "" {
		return http.MethodGet, nil
	}
	switch m {
	case http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return m, nil
	default:
		return "", fmt.Errorf("%s: unsupported method %q", actionName, in)
	}
}

func validateBodyOneOf(r *config.HTTPRequest) error {
	n := 0
	if r.Body != "" {
		n++
	}
	if r.JSON != nil {
		n++
	}
	if len(r.Form) > 0 {
		n++
	}
	if r.File != "" {
		n++
	}
	if n > 1 {
		return fmt.Errorf("%s: set at most one of body/json/form/file", actionName)
	}
	return nil
}

func validateAuthOneOf(r *config.HTTPRequest) error {
	if r.Auth == nil {
		return nil
	}
	n := 0
	if r.Auth.Bearer != "" {
		n++
	}
	if r.Auth.Basic != nil {
		n++
	}
	if r.Auth.Header != nil {
		n++
	}
	if n > 1 {
		return fmt.Errorf("%s: set at most one of auth.bearer/auth.basic/auth.header", actionName)
	}
	if r.Auth.Header != nil && strings.TrimSpace(r.Auth.Header.Name) == "" {
		return fmt.Errorf("%s: auth.header.name is required", actionName)
	}
	if r.Auth.Basic != nil && r.Auth.Basic.User == "" {
		return fmt.Errorf("%s: auth.basic.user is required", actionName)
	}
	return nil
}

// validateIdempotency is the kernel-honest core: POST and PATCH require
// EXACTLY ONE of IdempotencyKey, CreatesWhen, or Risk="high". Without
// one of these, the step would silently re-fire on every apply — the
// exact footgun every other tool ships. GET/HEAD/OPTIONS are read-only;
// PUT/DELETE are idempotent by HTTP spec.
func validateIdempotency(method string, r *config.HTTPRequest) error {
	if method != http.MethodPost && method != http.MethodPatch {
		return nil
	}
	signals := 0
	if r.IdempotencyKey != "" {
		signals++
	}
	if r.CreatesWhen != "" {
		signals++
	}
	if r.Risk == "high" {
		signals++
	}
	if signals == 0 {
		return fmt.Errorf("%s: %s is non-idempotent; set exactly one of "+
			"idempotency_key, creates_when, or risk: high", actionName, method)
	}
	if signals > 1 {
		return fmt.Errorf("%s: set exactly one of idempotency_key, creates_when, risk: high", actionName)
	}
	return nil
}

func validateRetryOn(tokens []string) error {
	for _, t := range tokens {
		switch strings.ToLower(t) {
		case "5xx", "4xx", "429", "timeout", "connection_error":
			// ok
		default:
			return fmt.Errorf("%s: unknown retry_on token %q (accepted: 5xx, 4xx, 429, timeout, connection_error)", actionName, t)
		}
	}
	return nil
}

func validateDuration(field, val string) error {
	if val == "" {
		return nil
	}
	if _, err := time.ParseDuration(val); err != nil {
		return fmt.Errorf("%s: invalid %s duration %q: %w", actionName, field, val, err)
	}
	return nil
}

// renderedRequest is the fully-rendered, template-resolved view of an
// HTTPRequest. All template strings have been resolved against the
// step's variables; bytes are ready to send.
type renderedRequest struct {
	method        string
	url           string
	headers       map[string]string // request headers (auth merged in)
	bodyBytes     []byte
	contentType   string // derived from body form unless overridden by user header
	timeout       time.Duration
	retryDelay    time.Duration
	retries       int
	retryOn       map[string]bool
	expectStatus  []int
	skipTLSVerify bool
	redactBody    bool
	maxRespBytes  int64
	followRedir   int
	// sensitiveHeaders is the set of header names whose values must be
	// redacted in logs/diffs/events. Built from the auth form + the
	// known-sensitive list.
	sensitiveHeaders map[string]bool
}

// Run is the unified Spec-16 entry point. Plan mode skips the network
// (Wave 1); apply mode renders, sends with retries, and captures the
// response as a registered fact.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rr, err := h.render(ctx, step)
	if err != nil {
		return nil, err
	}

	if ctx.Mode() == actions.ModePlan {
		return h.runPlan(ctx, rr)
	}
	return h.runApply(ctx, rr)
}

// runPlan reports what would happen without touching the network.
// Wave 2 will optionally execute a `probe:` GET in plan mode to
// evaluate CreatesWhen against current state.
func (h *Handler) runPlan(ctx actions.Context, rr *renderedRequest) (actions.Result, error) {
	res := executor.NewResult()
	res.Checkable = true
	bodyHint := ""
	if len(rr.bodyBytes) > 0 {
		if rr.redactBody {
			bodyHint = ", body=<redacted>"
		} else {
			bodyHint = fmt.Sprintf(", body=%d bytes", len(rr.bodyBytes))
		}
	}
	if readMethods[rr.method] {
		res.WouldChange = false
		res.Reason = fmt.Sprintf("would call %s %s (read-only%s)", rr.method, rr.url, bodyHint)
	} else {
		res.WouldChange = true
		res.Reason = fmt.Sprintf("would call %s %s%s", rr.method, rr.url, bodyHint)
	}
	ctx.GetLogger().Infof("  [plan] %s", res.Reason)
	return res, nil
}

// runApply sends the request with retry classification, captures the
// response into the result.Data, and emits the audit event.
func (h *Handler) runApply(ctx actions.Context, rr *renderedRequest) (actions.Result, error) {
	res := executor.NewResult()
	res.StartTime = time.Now()
	defer func() {
		res.EndTime = time.Now()
		res.Duration = res.EndTime.Sub(res.StartTime)
	}()

	client := buildClient(rr)

	var (
		got     *httpAttempt
		attempt int
		lastErr error
	)
	maxAttempts := rr.retries + 1
	for attempt = 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			ctx.GetLogger().Debugf("  retry %d/%d after %s", attempt-1, rr.retries, rr.retryDelay)
			time.Sleep(rr.retryDelay)
		}
		got, lastErr = h.sendOnce(client, rr)
		if !shouldRetry(got, lastErr, rr) {
			break
		}
		got = nil
		if attempt == maxAttempts {
			break
		}
	}

	statusCode := 0
	if got != nil {
		statusCode = got.statusCode
	}

	// Build the fact regardless of success/failure so the user can
	// inspect what happened on partial failures.
	data := map[string]interface{}{
		"method":      rr.method,
		"url":         rr.url,
		"status_code": statusCode,
		"attempts":    attempt,
		"duration_ms": time.Since(res.StartTime).Milliseconds(),
		"success":     false,
	}
	if got != nil {
		data["headers"] = flattenResponseHeaders(got.header, rr.sensitiveHeaders)
		body := got.body
		truncated := false
		if rr.maxRespBytes > 0 && int64(len(body)) > rr.maxRespBytes {
			body = body[:rr.maxRespBytes]
			truncated = true
		}
		if rr.redactBody {
			data["body"] = "<redacted>"
		} else {
			data["body"] = string(body)
			if parsed, ok := tryParseJSON(got.contentType, body); ok {
				data["json"] = parsed
			}
		}
		data["truncated"] = truncated
		data["final_url"] = got.finalURL
	}

	// Publish event (host + path only; no query, no body).
	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventHTTPRequested,
			Data: events.HTTPRequestedData{
				Method:     rr.method,
				URL:        sanitizeURLForEvent(rr.url),
				StatusCode: statusCode,
				DurationMs: time.Since(res.StartTime).Milliseconds(),
				Attempts:   attempt,
				DryRun:     false,
			},
		})
	}

	if lastErr != nil {
		res.Failed = true
		res.Data = data
		return res, fmt.Errorf("%s: %s %s: %w", actionName, rr.method, rr.url, lastErr)
	}

	if !statusAccepted(statusCode, rr.expectStatus) {
		res.Failed = true
		res.Data = data
		return res, fmt.Errorf("%s: %s %s returned %d (expected %v)",
			actionName, rr.method, rr.url, statusCode, rr.expectStatus)
	}

	data["success"] = true
	res.Data = data
	// Write methods are reported as Changed=true (the action made a
	// remote-side change, even if we don't know exactly what); read
	// methods don't count as changes.
	res.Changed = !readMethods[rr.method]
	ctx.GetLogger().Infof("  HTTP %s %s -> %d (%dms, %d attempt(s))",
		rr.method, rr.url, statusCode, data["duration_ms"], attempt)
	return res, nil
}

// httpAttempt captures one round-trip's result. We surface this rather
// than the raw *http.Response so the caller never holds an open Body;
// fields are copied out before the response is closed.
type httpAttempt struct {
	statusCode  int
	header      http.Header
	contentType string
	finalURL    string
	body        []byte
}

// sendOnce issues a single request, reads + closes the body, and
// returns the fields the caller needs as a value. Returns (nil, err)
// on transport failure.
func (h *Handler) sendOnce(client *http.Client, rr *renderedRequest) (*httpAttempt, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rr.timeout)
	defer cancel()

	var body io.Reader
	if len(rr.bodyBytes) > 0 {
		body = bytes.NewReader(rr.bodyBytes)
	}

	req, err := httputil.NewRequest(ctx, rr.method, rr.url, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if rr.contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", rr.contentType)
	}
	for k, v := range rr.headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	reader := io.Reader(resp.Body)
	if rr.maxRespBytes > 0 {
		// Cap reads at maxRespBytes+1 so we can detect truncation
		// (the extra byte tells us "there was more").
		reader = io.LimitReader(resp.Body, rr.maxRespBytes+1)
	}
	data, err := io.ReadAll(reader)
	out := &httpAttempt{
		statusCode:  resp.StatusCode,
		header:      resp.Header.Clone(),
		contentType: resp.Header.Get("Content-Type"),
		finalURL:    resp.Request.URL.String(),
		body:        data,
	}
	if err != nil {
		return out, fmt.Errorf("read response body: %w", err)
	}
	return out, nil
}

// shouldRetry inspects an attempt + error and decides whether to retry.
func shouldRetry(got *httpAttempt, err error, rr *renderedRequest) bool {
	if len(rr.retryOn) == 0 {
		return false
	}
	if err != nil {
		if isTimeoutErr(err) && rr.retryOn["timeout"] {
			return true
		}
		if rr.retryOn["connection_error"] {
			return true
		}
		return false
	}
	if got == nil {
		return false
	}
	if rr.retryOn["5xx"] && got.statusCode >= 500 && got.statusCode < 600 {
		return true
	}
	if rr.retryOn["4xx"] && got.statusCode >= 400 && got.statusCode < 500 {
		return true
	}
	if rr.retryOn["429"] && got.statusCode == http.StatusTooManyRequests {
		return true
	}
	return false
}

func isTimeoutErr(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var ne interface{ Timeout() bool }
	if errors.As(err, &ne) {
		return ne.Timeout()
	}
	return false
}

// statusAccepted reports whether code is in the expected set. If
// expectStatus is empty, the default is "any 2xx".
func statusAccepted(code int, expect []int) bool {
	if len(expect) == 0 {
		return code >= 200 && code < 300
	}
	return slices.Contains(expect, code)
}

// buildClient constructs the HTTP client for one rendered request.
// Reuses the project-wide DefaultTransport for connection pooling unless
// SkipTLSVerify or a custom redirect bound forces a per-request override.
func buildClient(rr *renderedRequest) *http.Client {
	transport := httputil.DefaultTransport
	if rr.skipTLSVerify {
		// Clone so we don't mutate the shared transport. The clone
		// shares the connection pool with the parent (Go's
		// (*http.Transport).Clone semantics) only if we don't change
		// TLSClientConfig — which we do — so this request gets its own
		// pool. Acceptable: skip_tls_verify is rare and dev-only.
		clone := transport.Clone()
		if clone.TLSClientConfig == nil {
			clone.TLSClientConfig = &tls.Config{} //nolint:gosec // user-opted
		}
		// #nosec G402 -- explicit user opt-in via skip_tls_verify
		clone.TLSClientConfig.InsecureSkipVerify = true
		transport = clone
	}
	client := &http.Client{Transport: transport}
	limit := rr.followRedir
	client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
		if len(via) >= limit {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return client
}

// tryParseJSON attempts to parse body as JSON when Content-Type
// announces it. Returns (value, true) on success; (nil, false) otherwise.
func tryParseJSON(contentType string, body []byte) (interface{}, bool) {
	ct := strings.ToLower(contentType)
	if !strings.Contains(ct, "application/json") && !strings.HasSuffix(ct, "+json") {
		return nil, false
	}
	var v interface{}
	if err := decodeJSON(body, &v); err != nil {
		return nil, false
	}
	return v, true
}

// flattenResponseHeaders converts http.Header (map[string][]string) into
// a flat map[string]string for ergonomic templating. Multi-value headers
// are comma-joined per RFC 7230 §3.2.2. Sensitive header values are
// redacted.
func flattenResponseHeaders(h http.Header, sensitive map[string]bool) map[string]string {
	out := make(map[string]string, len(h))
	for k, vs := range h {
		if sensitive[strings.ToLower(k)] {
			out[k] = "<redacted>"
			continue
		}
		out[k] = strings.Join(vs, ", ")
	}
	return out
}

// sanitizeURLForEvent strips the query string and fragment. Path is
// kept; the trade-off is "audit logs benefit from path-level info"
// vs. "paths sometimes leak secrets" — the latter is rarer and is
// the user's responsibility to avoid (don't put secrets in URLs).
func sanitizeURLForEvent(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// readFile is the indirection that lets tests stub file reads. Honours
// the same template-rendered path as everything else.
var readFile = os.ReadFile
