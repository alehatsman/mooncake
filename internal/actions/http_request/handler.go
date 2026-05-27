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
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/httputil"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

const (
	actionName = "http.request"

	defaultTimeout          = 30 * time.Second
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
//
// The path advertised here is the unrendered template — spec-44
// doctor only uses it to spot obvious cases (a relative path that
// can't be resolved, a parent dir that doesn't exist on the literal
// path). When the path is fully template-driven the check is a
// no-op; that's expected — doctor catches the static-misuse case,
// the runtime path catches the rest.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{Network: true}
	if step == nil || step.HTTPRequest == nil {
		return ps
	}
	if strings.TrimSpace(step.HTTPRequest.SaveTo) != "" {
		ps.FilesystemWrite = []string{step.HTTPRequest.SaveTo}
	}
	return ps
}

// Validate enforces the structural contract: URL required, one-of body,
// one-of auth, supported method, idempotency contract for POST/PATCH,
// valid retry_on tokens, parseable durations, valid risk. Wave 2:
// probe / reverse sub-blocks are validated recursively.
func (Handler) Validate(step *config.Step) error {
	if step == nil || step.HTTPRequest == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	r := step.HTTPRequest

	// HTTP-with-sudo is nonsense; surface as a clear error rather than
	// silently honoring it.
	if step.AsUser != "" {
		return fmt.Errorf("%s: as_user is not supported (HTTP calls never run as another user)", actionName)
	}

	if err := validateStructural(r, ""); err != nil {
		return err
	}

	method, _ := normalizeMethod(r.Method)
	if err := validateIdempotency(method, r); err != nil {
		return err
	}

	// Wave 2: probe must be a read-method sub-request, and must not
	// nest probe/reverse. Idempotency contract does NOT apply to
	// probe (read-only). Wave 3: probe also must not declare save_to —
	// persisting an inspection-only response confuses the audit story
	// (operators expect save_to to mean "the call I made", not "the
	// pre-flight check").
	if r.Probe != nil {
		if r.Probe.Probe != nil {
			return fmt.Errorf("%s: probe must not nest another probe", actionName)
		}
		if r.Probe.Reverse != nil {
			return fmt.Errorf("%s: probe must not declare reverse", actionName)
		}
		if strings.TrimSpace(r.Probe.SaveTo) != "" {
			return fmt.Errorf("%s: probe must not declare save_to (probes are read-only inspection; declare save_to on the top-level request)", actionName)
		}
		if strings.TrimSpace(r.Probe.ExpectJSONSchema) != "" {
			return fmt.Errorf("%s: probe must not declare expect_json_schema (probes are read-only inspection; declare expect_json_schema on the top-level request)", actionName)
		}
		probeMethod, err := normalizeMethod(r.Probe.Method)
		if err != nil {
			return fmt.Errorf("%s.probe: %w", actionName, err)
		}
		if !readMethods[probeMethod] {
			return fmt.Errorf("%s: probe method must be GET/HEAD/OPTIONS (got %s)", actionName, probeMethod)
		}
		if err := validateStructural(r.Probe, "probe."); err != nil {
			return err
		}
	}

	// Wave 2: reverse is user-owned compensation; structural validity
	// is enforced, but the idempotency contract is NOT — the operator
	// signed up for the verb. No nested probe/reverse. Wave 3: reverse
	// may declare its own save_to — useful when the rollback response
	// also needs to be persisted (e.g., audit logs for compensating
	// transactions).
	if r.Reverse != nil {
		if r.Reverse.Probe != nil {
			return fmt.Errorf("%s: reverse must not declare probe", actionName)
		}
		if r.Reverse.Reverse != nil {
			return fmt.Errorf("%s: reverse must not nest another reverse", actionName)
		}
		if _, err := normalizeMethod(r.Reverse.Method); err != nil {
			return fmt.Errorf("%s.reverse: %w", actionName, err)
		}
		if err := validateStructural(r.Reverse, "reverse."); err != nil {
			return err
		}
	}

	return nil
}

// validateStructural runs the field-level checks that apply identically
// to the top-level request and to nested probe/reverse blocks: URL
// non-empty, body/auth one-of, retry_on tokens, parseable durations,
// non-negative retries / max-response-bytes, valid expect_status, valid
// risk. The fieldPrefix ("", "probe.", "reverse.") is prepended to
// error messages so the operator sees which level rejected.
func validateStructural(r *config.HTTPRequest, fieldPrefix string) error {
	at := func(field string) string { return fieldPrefix + field }
	if strings.TrimSpace(r.URL) == "" {
		return fmt.Errorf("%s: %s is required", actionName, at("url"))
	}
	if _, err := normalizeMethod(r.Method); err != nil {
		return err
	}
	if err := validateBodyOneOf(r); err != nil {
		return err
	}
	if err := validateAuthOneOf(r); err != nil {
		return err
	}
	if err := validateRetryOn(r.RetryOn); err != nil {
		return err
	}
	if err := validateDuration(at("timeout"), r.Timeout); err != nil {
		return err
	}
	for _, code := range r.ExpectStatus {
		if code < 100 || code > 599 {
			return fmt.Errorf("%s: invalid %s %d", actionName, at("expect_status"), code)
		}
	}
	if r.Risk != "" && r.Risk != "low" && r.Risk != "medium" && r.Risk != "high" {
		return fmt.Errorf("%s: %s must be one of low|medium|high (got %q)", actionName, at("risk"), r.Risk)
	}
	if r.MaxResponseBytes < 0 {
		return fmt.Errorf("%s: %s must be >= 0", actionName, at("max_response_bytes"))
	}
	if r.ExpectJSONSchema != "" && strings.TrimSpace(r.ExpectJSONSchema) == "" {
		return fmt.Errorf("%s: %s must not be whitespace-only", actionName, at("expect_json_schema"))
	}
	for i, k := range r.ExpectJSONKeys {
		if strings.TrimSpace(k) == "" {
			return fmt.Errorf("%s: %s[%d] is empty", actionName, at("expect_json_keys"), i)
		}
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
//
// Retry attempts + delay live on the step (RetryPolicy) so the
// executor's runWithRetry can drive the loop uniformly; only the
// HTTP-specific retry-on classifier travels with the request.
type renderedRequest struct {
	method        string
	url           string
	headers       map[string]string // request headers (auth merged in)
	bodyBytes     []byte
	contentType   string // derived from body form unless overridden by user header
	timeout       time.Duration
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

// Run is the unified Spec-16 entry point. Plan mode optionally
// executes a `probe:` read-request to evaluate `creates_when:` against
// current state (Wave 2); apply mode renders, sends a single request,
// and captures the response as a registered fact. Retry is owned by
// the executor's runWithRetry — see RunRaw + IsRetryable below.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rr, err := h.render(ctx, step.HTTPRequest, ctx.GetVariables(), "")
	if err != nil {
		return nil, err
	}

	if ctx.Mode() == actions.ModePlan {
		return h.runPlan(ctx, step, rr)
	}
	return h.runApply(ctx, step, rr)
}

// RunRaw is the spec-69 RawRunner entry point: a single send attempt,
// no retry loop. The executor wraps this in runWithRetry when the
// user sets a step-level retry policy; IsRetryable below classifies
// each attempt's outcome against the RetryOn tokens.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

// IsRetryable classifies a failed attempt against the step's RetryOn
// tokens. Called by the executor's retry loop only when err != nil —
// runApply returns a non-nil err for transport failures AND for
// status codes outside expectStatus, so this method sees every
// retryable signal. RetryOn is HTTP-specific and stays on the action
// config; attempts + delay live on step.Retry.
func (h *Handler) IsRetryable(result actions.Result, err error, step *config.Step) bool {
	if step == nil || step.HTTPRequest == nil || len(step.HTTPRequest.RetryOn) == 0 {
		return false
	}
	retryOn := make(map[string]bool, len(step.HTTPRequest.RetryOn))
	for _, t := range step.HTTPRequest.RetryOn {
		retryOn[strings.ToLower(t)] = true
	}
	// Transport-layer failures (no response): timeout vs. generic
	// connection error are the two buckets we classify.
	r, ok := result.(*executor.Result)
	if !ok || r == nil || r.Data == nil {
		if err == nil {
			return false
		}
		if isTimeoutErr(err) {
			return retryOn["timeout"]
		}
		return retryOn["connection_error"]
	}
	// Response-level failures: pull the recorded status code from
	// the fact map. status_code 0 means no response made it back —
	// fall back to the transport-error classifier above.
	status, _ := r.Data["status_code"].(int)
	if status == 0 {
		if err == nil {
			return false
		}
		if isTimeoutErr(err) {
			return retryOn["timeout"]
		}
		return retryOn["connection_error"]
	}
	if retryOn["5xx"] && status >= 500 && status < 600 {
		return true
	}
	if retryOn["4xx"] && status >= 400 && status < 500 {
		return true
	}
	if retryOn["429"] && status == http.StatusTooManyRequests {
		return true
	}
	return false
}

// runPlan reports what would happen without touching the network for
// the main call. Wave 2 honors a `probe:` block (executed as a read
// request) plus `creates_when:` (boolean predicate) so the operator
// sees "would create" vs. "already exists" at plan time.
func (h *Handler) runPlan(ctx actions.Context, step *config.Step, rr *renderedRequest) (actions.Result, error) {
	res := executor.NewResult()
	res.Checkable = true
	res.Operation = executor.OpUpdate
	res.Target = rr.url
	r := step.HTTPRequest

	bodyHint := bodyHintFor(rr)

	// Wave 2: probe + creates_when path.
	if r.Probe != nil || r.CreatesWhen != "" {
		probeFact, probeErr := h.executeProbe(ctx, r.Probe)
		if probeErr != nil {
			// Probe failure is informational, not fatal: a transient
			// network blip during plan shouldn't gate the entire run.
			// Fall through with probeFact=nil so creates_when (if any)
			// evaluates against the existing scope only.
			ctx.GetLogger().Debugf("  probe failed: %v", probeErr)
			res.Reason = fmt.Sprintf("probe failed: %v; would call %s %s%s", probeErr, rr.method, rr.url, bodyHint)
			res.WouldChange = !readMethods[rr.method]
			return res, nil
		}
		if r.CreatesWhen != "" {
			would, err := evalCreatesWhen(ctx, r.CreatesWhen, probeFact)
			if err != nil {
				return nil, err
			}
			res.WouldChange = would
			if would {
				res.Reason = fmt.Sprintf("would call %s %s (creates_when: true%s)", rr.method, rr.url, bodyHint)
			} else {
				res.Reason = fmt.Sprintf("skip %s %s (creates_when: false — state already matches)", rr.method, rr.url)
			}
			ctx.GetLogger().Infof("  [plan] %s", res.Reason)
			return res, nil
		}
		// Probe ran but no creates_when to evaluate; surface the probe
		// result and fall back to the Wave-1 default for WouldChange.
		res.Data = map[string]interface{}{"probe": probeFact}
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

// bodyHintFor describes the body in plan-mode reason strings without
// leaking content. Redacted bodies report "<redacted>" instead of size.
func bodyHintFor(rr *renderedRequest) string {
	if len(rr.bodyBytes) == 0 {
		return ""
	}
	if rr.redactBody {
		return ", body=<redacted>"
	}
	return fmt.Sprintf(", body=%d bytes", len(rr.bodyBytes))
}

// executeProbe runs the probe sub-request in plan mode. Single attempt,
// short timeout. Returns the response fact map (status_code, headers,
// body, json, …) or an error if the probe is nil OR the network call
// itself fails. nil probe + nil error means "no probe to run."
func (h *Handler) executeProbe(ctx actions.Context, probe *config.HTTPRequest) (map[string]interface{}, error) {
	if probe == nil {
		return nil, nil
	}
	pr, err := h.render(ctx, probe, ctx.GetVariables(), "probe.")
	if err != nil {
		return nil, err
	}
	// Probe has its own retry budget; we don't override it here. The
	// user's probe config wins. Most probes will be retries=0 single
	// shots, which is fine.
	client := buildClient(pr)
	got, err := h.sendOnce(client, pr)
	if err != nil {
		return nil, fmt.Errorf("probe request: %w", err)
	}
	return buildFact(got, pr), nil
}

// buildFact converts an httpAttempt into the map[string]interface{}
// shape used everywhere we surface response state (runApply result.Data,
// probe results). Single source of truth for the fact schema.
// Callers always pass a non-nil rr; got may be nil if the transport
// never produced a response (network error).
func buildFact(got *httpAttempt, rr *renderedRequest) map[string]interface{} {
	if got == nil {
		return map[string]interface{}{"status_code": 0, "success": false}
	}
	body := got.body
	truncated := false
	if rr.maxRespBytes > 0 && int64(len(body)) > rr.maxRespBytes {
		body = body[:rr.maxRespBytes]
		truncated = true
	}
	out := map[string]interface{}{
		"status_code": got.statusCode,
		"headers":     flattenResponseHeaders(got.header, rr.sensitiveHeaders),
		"truncated":   truncated,
		"final_url":   got.finalURL,
	}
	if rr.redactBody {
		out["body"] = "<redacted>"
	} else {
		out["body"] = string(body)
		if parsed, ok := tryParseJSON(got.contentType, body); ok {
			out["json"] = parsed
		}
	}
	return out
}

// evalCreatesWhen renders + evaluates the creates_when predicate with
// `probe` merged into scope when set. Returns the predicate's bool.
func evalCreatesWhen(ctx actions.Context, expr string, probeFact map[string]interface{}) (bool, error) {
	base := ctx.GetVariables()
	merged := make(map[string]interface{}, len(base)+1)
	for k, v := range base {
		merged[k] = v
	}
	if probeFact != nil {
		merged["probe"] = probeFact
	}
	rendered, err := ctx.GetTemplate().Render(expr, merged)
	if err != nil {
		return false, fmt.Errorf("%s.creates_when render: %w", actionName, err)
	}
	out, err := ctx.GetEvaluator().Evaluate(rendered, merged)
	if err != nil {
		return false, fmt.Errorf("%s.creates_when evaluate: %w", actionName, err)
	}
	b, ok := out.(bool)
	if !ok {
		return false, fmt.Errorf("%s.creates_when must evaluate to bool, got %T", actionName, out)
	}
	return b, nil
}

// runApply sends ONE request, captures the response, and emits the
// audit event. Retry is the executor's job — when the user sets a
// step-level retry: block, runWithRetry calls this multiple times and
// IsRetryable classifies each failure. data["attempts"] is fixed up
// by the executor post-loop with the cross-attempt count.
func (h *Handler) runApply(ctx actions.Context, step *config.Step, rr *renderedRequest) (actions.Result, error) {
	res := executor.NewResult()
	res.Operation = executor.OpUpdate
	res.Target = rr.url
	res.StartTime = time.Now()
	defer func() {
		res.EndTime = time.Now()
		res.Duration = res.EndTime.Sub(res.StartTime)
	}()

	client := buildClient(rr)
	got, transportErr := h.sendOnce(client, rr)

	statusCode := 0
	if got != nil {
		statusCode = got.statusCode
	}

	// Build the fact regardless of success/failure so the user can
	// inspect what happened on partial failures.
	data := buildFact(got, rr)
	data["method"] = rr.method
	data["url"] = rr.url
	// Per-attempt count; the executor overwrites this with the final
	// cross-attempt count when retries are configured.
	data["attempts"] = 1
	data["duration_ms"] = time.Since(res.StartTime).Milliseconds()
	data["success"] = false

	// Publish event (host + path only; no query, no body). Emitted
	// per-attempt — if retries fire, you'll see multiple events with
	// the same step ID, each carrying its own status_code.
	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventHTTPRequested,
			Data: events.HTTPRequestedData{
				Method:     rr.method,
				URL:        sanitizeURLForEvent(rr.url),
				StatusCode: statusCode,
				DurationMs: time.Since(res.StartTime).Milliseconds(),
				Attempts:   1,
				DryRun:     false,
			},
		})
	}

	if transportErr != nil {
		res.Failed = true
		res.Data = data
		return res, fmt.Errorf("%s: %s %s: %w", actionName, rr.method, rr.url, transportErr)
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

	// Narrow wave-3 piece: assert response.json contains the
	// operator's declared top-level keys. Independent from the
	// (deferred) full JSON-schema validation — this is the common
	// "prove the API returned an id" assertion. The check happens
	// AFTER status validation so a 200-with-malformed-body fails
	// with a clear "missing keys" message rather than a generic
	// status-mismatch.
	if len(step.HTTPRequest.ExpectJSONKeys) > 0 {
		if err := checkExpectJSONKeys(step.HTTPRequest.ExpectJSONKeys, data); err != nil {
			res.Failed = true
			return res, fmt.Errorf("%s: %w", actionName, err)
		}
	}

	// Wave 3 / expect_json_schema: validate response.json against a
	// JSON-Schema (draft-07) file. Path is Node-style — resolved
	// against ec.CurrentDir (the dir of the YAML file declaring the
	// step). Composes with expect_json_keys (keys above already
	// passed); runs second because it's more expensive.
	if strings.TrimSpace(step.HTTPRequest.ExpectJSONSchema) != "" {
		if err := checkExpectJSONSchema(ctx, step.HTTPRequest.ExpectJSONSchema, data); err != nil {
			res.Failed = true
			return res, fmt.Errorf("%s: %w", actionName, err)
		}
	}

	// Wave 3: persist the response body to save_to when set. The path
	// is template-rendered so callers can interpolate response facts
	// (e.g. `save_to: "/var/cache/hooks/{{ response.json.id }}.json"`).
	// Parent directories are created (mkdir -p). A write failure fails
	// the step — the user asked for the file; silent loss would
	// surprise them. data["saved_to"] holds the rendered path so
	// downstream steps can read the actual location.
	if strings.TrimSpace(step.HTTPRequest.SaveTo) != "" {
		rendered, err := writeResponseBody(ctx, step.HTTPRequest.SaveTo, got.body, data)
		if err != nil {
			res.Failed = true
			return res, fmt.Errorf("%s: save_to: %w", actionName, err)
		}
		data["saved_to"] = rendered
	}

	// Wave 2: snapshot the reverse block with response fact merged in,
	// so the eventual Reverse() call returns a Step with already-
	// resolved URLs/headers/auth that reference the actual resource
	// the apply created (e.g. `hook.json.id`).
	if step.HTTPRequest.Reverse != nil {
		snap, err := h.snapshotReverse(ctx, step, data)
		if err != nil {
			// Render-error on reverse block is informational: the
			// apply itself succeeded; we just can't snapshot a clean
			// reverse. Surface it so Reverse() fails loudly later.
			ctx.GetLogger().Errorf("  reverse-block snapshot failed: %v", err)
		} else {
			res.ReverseData = snap
		}
	}

	// One log line per RunRaw call. The executor's retry loop logs its
	// own "retry N/M" line between attempts; consumers tracking total
	// trips should read response.attempts from the registered fact,
	// which the executor fixes up post-loop with the cross-attempt count.
	ctx.GetLogger().Infof("  HTTP %s %s -> %d (%dms)",
		rr.method, rr.url, statusCode, data["duration_ms"])
	return res, nil
}

// snapshotReverse renders the reverse block's templates against vars
// that include the just-completed response under both `response` (a
// stable name) and step.As (the user's chosen register name, if any).
// Returns a *config.HTTPRequest with no remaining templates — Reverse()
// wraps it in a Step that the transaction layer can re-run as a normal
// http.request invocation.
func (h *Handler) snapshotReverse(ctx actions.Context, step *config.Step, responseFact map[string]interface{}) (*config.HTTPRequest, error) {
	base := ctx.GetVariables()
	merged := make(map[string]interface{}, len(base)+2)
	for k, v := range base {
		merged[k] = v
	}
	merged["response"] = responseFact
	if step.As != "" {
		merged[step.As] = responseFact
	}
	return renderConfig(ctx, step.HTTPRequest.Reverse, merged, "reverse.")
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

// shouldRetry was deleted with the in-handler retry loop; the
// equivalent classifier now lives on *Handler.IsRetryable above and
// is called by the executor's runWithRetry between attempts.

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

// writeResponseBody persists the response bytes to a template-
// rendered path. Parent directories are created with 0o755; the file
// itself is written 0o644. Wave 3 / save_to.
//
// The render context exposes the response fact under `response` (a
// stable name) and step-As (if set) — matches snapshotReverse's
// convention so callers can interpolate response.json.id / .status_code
// into the path.
//
// Errors propagate: a write failure fails the step, since the user
// asked for the file and silent loss would surprise them. mkdir -p
// is best-effort idempotent — re-running the same plan twice creates
// the parent dir once.
func writeResponseBody(ctx actions.Context, pathTemplate string, body []byte, responseFact map[string]interface{}) (string, error) {
	base := ctx.GetVariables()
	merged := make(map[string]interface{}, len(base)+1)
	for k, v := range base {
		merged[k] = v
	}
	merged["response"] = responseFact
	rendered, err := ctx.GetTemplate().Render(pathTemplate, merged)
	if err != nil {
		return "", fmt.Errorf("render path %q: %w", pathTemplate, err)
	}
	rendered = strings.TrimSpace(rendered)
	if rendered == "" {
		return "", fmt.Errorf("save_to rendered to empty path (template %q)", pathTemplate)
	}
	if dir := filepath.Dir(rendered); dir != "" && dir != "." {
		// #nosec G301 -- 0755 is the standard "parent dir for files
		// the daemon writes"; the file itself is 0644.
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("mkdir -p %s: %w", dir, err)
		}
	}
	// #nosec G306 -- 0644 is the standard "file the operator can
	// read" mode; sensitive responses should declare redact_body and
	// not be persisted via save_to.
	if err := os.WriteFile(rendered, body, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", rendered, err)
	}
	return rendered, nil
}

// checkExpectJSONKeys verifies the parsed-JSON response fact has all
// listed top-level keys. data["json"] is the auto-parsed payload —
// when the Content-Type wasn't application/json, this stays nil and
// the check fails with a clear "response was not JSON" message
// rather than a confusing "missing key X" on a string body.
//
// Missing keys are reported as a deterministic sorted list so the
// error message diffs cleanly across reruns.
func checkExpectJSONKeys(want []string, data map[string]interface{}) error {
	raw, ok := data["json"]
	if !ok || raw == nil {
		return fmt.Errorf("expect_json_keys: response was not JSON (Content-Type did not auto-parse); declare a JSON-returning endpoint or drop expect_json_keys")
	}
	obj, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expect_json_keys: response JSON is %T, want object", raw)
	}
	var missing []string
	for _, k := range want {
		if _, present := obj[k]; !present {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("expect_json_keys: missing key(s) %v in response.json", missing)
	}
	return nil
}

// checkExpectJSONSchema validates the parsed-JSON response fact
// against a JSON Schema (draft-07) loaded from disk. The schema path
// is resolved Node-style against the step's source-file dir
// (ec.CurrentDir) — `./schema.json` is next to the YAML declaring the
// step; `../schemas/x.json` walks one up; absolute paths honored.
//
// Failure modes, all before the response is accepted:
//   - response was not JSON (Content-Type didn't auto-parse)
//   - schema file is missing / unreadable
//   - schema is malformed (compile error)
//   - response.json fails validation — first violation is reported
//     with its JSON-pointer location
//
// Compilation runs at apply time (not Validate) because the path may
// reference vars. The compiled schema is local to this call; no
// process-wide cache (each step is reborn per apply, and schemas are
// cheap to compile relative to a network round-trip).
func checkExpectJSONSchema(ctx actions.Context, schemaPathTemplate string, data map[string]interface{}) error {
	raw, ok := data["json"]
	if !ok || raw == nil {
		return fmt.Errorf("expect_json_schema: response was not JSON (Content-Type did not auto-parse); declare a JSON-returning endpoint or drop expect_json_schema")
	}

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("expect_json_schema: context is not an ExecutionContext")
	}
	resolved, err := ec.Svc.PathUtil.ExpandPath(schemaPathTemplate, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return fmt.Errorf("expect_json_schema: resolve %q: %w", schemaPathTemplate, err)
	}
	resolved = strings.TrimSpace(resolved)
	if resolved == "" {
		return fmt.Errorf("expect_json_schema: path %q rendered to empty string", schemaPathTemplate)
	}

	schemaBytes, err := readFile(resolved)
	if err != nil {
		return fmt.Errorf("expect_json_schema: read %s: %w", resolved, err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7
	const schemaURL = "mooncake://expect_json_schema"
	if err := compiler.AddResource(schemaURL, bytes.NewReader(schemaBytes)); err != nil {
		return fmt.Errorf("expect_json_schema: add schema %s: %w", resolved, err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return fmt.Errorf("expect_json_schema: compile %s: %w", resolved, err)
	}

	if err := schema.Validate(raw); err != nil {
		// jsonschema returns a *ValidationError tree; the leaf-most
		// cause is the most actionable. Walk to the deepest leaf and
		// report it with its InstanceLocation (JSON pointer).
		if vErr, ok := err.(*jsonschema.ValidationError); ok {
			leaf := deepestValidationCause(vErr)
			loc := leaf.InstanceLocation
			if loc == "" {
				loc = "/"
			}
			return fmt.Errorf("expect_json_schema: response failed validation at %s: %s", loc, leaf.Message)
		}
		return fmt.Errorf("expect_json_schema: response failed validation: %w", err)
	}
	return nil
}

// deepestValidationCause walks the *jsonschema.ValidationError tree
// to the most specific leaf cause — the one with no further causes
// of its own. That's the actionable "this field, this rule" message;
// the outer errors are just "oneOf failed" envelopes.
func deepestValidationCause(v *jsonschema.ValidationError) *jsonschema.ValidationError {
	cur := v
	for len(cur.Causes) > 0 {
		cur = cur.Causes[0]
	}
	return cur
}
