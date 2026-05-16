// Package observe_http implements the observe.http action: single-shot
// HTTP GET observation with typed result (spec-59 Phase 3). Network-
// flagged via Permissions{Network:true}.
package observe_http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/httputil"
)

const (
	actionName      = "observe.http"
	defaultMethod   = "GET"
	defaultTimeout  = 3 * time.Second
	bodySampleBytes = 2048
)

// HTTPObservation is the typed Value payload for observe.http.
// Reachable=transport-level success (any 1xx-5xx; not a transport
// failure). StatusCode=0 when Reachable=false.
type HTTPObservation struct {
	URL        string            `json:"url"`
	Method     string            `json:"method"`
	StatusCode int               `json:"status_code"`
	Reachable  bool              `json:"reachable"`
	LatencyMs  int64             `json:"latency_ms"`
	Headers    map[string]string `json:"headers,omitempty"`
	BodySample string            `json:"body_sample,omitempty"`
}

type Handler struct{}

func init() { actions.Register(&Handler{}) }

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               actionName,
		Description:        "Single-shot HTTP GET; returns typed status, latency, headers, body sample",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
		CaptureInPlan:      true,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	o := step.ObserveHTTP
	if o == nil {
		return fmt.Errorf("%s requires configuration", actionName)
	}
	if o.URL == "" {
		return fmt.Errorf("%s: url is required", actionName)
	}
	if o.Timeout != "" {
		if _, err := time.ParseDuration(o.Timeout); err != nil {
			return fmt.Errorf("%s: invalid timeout %q: %w", actionName, o.Timeout, err)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	o := step.ObserveHTTP

	result := executor.NewResult()
	result.Changed = false
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	method := o.Method
	if method == "" {
		method = defaultMethod
	}

	if ctx.Mode() == actions.ModePlan {
		env := actions.PlanDeferred(HTTPObservation{URL: o.URL, Method: method})
		publish(result, env)
		result.Checkable = true
		result.Reason = fmt.Sprintf("would observe %s %s (deferred to apply)", method, o.URL)
		return result, nil
	}

	timeout := defaultTimeout
	if o.Timeout != "" {
		timeout, _ = time.ParseDuration(o.Timeout)
	}

	// F012: base transport inherits httputil's bounded dial / TLS /
	// response-headers timeouts so a stuck remote can't hang the probe
	// past the per-request Timeout. SkipTLSVerify is a user opt-in
	// override that builds a fresh Transport (otherwise the
	// httputil.DefaultTransport's TLSClientConfig would persist across
	// callers — sharing is the point of httputil).
	client := &http.Client{Timeout: timeout, Transport: httputil.DefaultTransport}
	if o.SkipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // user opt-in
			TLSHandshakeTimeout:   30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
		}
	}
	// Issue #18: opt-out for redirect following. Default (nil) matches
	// Go's http.Client default of 10. Explicit 0 disables following so
	// the operator can probe the redirect status itself (canonical:
	// `observe.http --url http://x --expect-status 301`).
	if o.FollowRedirects != nil {
		max := *o.FollowRedirects
		client.CheckRedirect = func(_ *http.Request, via []*http.Request) error {
			if len(via) >= max {
				// Returning ErrUseLastResponse stops the redirect chain
				// and surfaces the response the server gave us (the 3xx)
				// instead of erroring. Matches the "I want to see the
				// redirect" intent.
				return http.ErrUseLastResponse
			}
			return nil
		}
	}

	rendered, err := ctx.GetTemplate().Render(o.URL, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".url", Cause: err}
	}

	obs := HTTPObservation{URL: rendered, Method: method}
	start := time.Now()
	// F012: ctx-aware request via httputil so the canonical UA flows
	// and any future caller ctx (e.g. agentd.Worker via F016) can
	// abort the probe. observe.http has no caller ctx today —
	// Background is bounded by the client.Timeout above.
	req, err := httputil.NewRequest(context.Background(), method, rendered, nil)
	if err != nil {
		obs.LatencyMs = time.Since(start).Milliseconds()
		publish(result, actions.ObserveResult{Value: obs, AsOf: time.Now(), Error: err.Error()})
		return result, nil
	}
	resp, err := client.Do(req)
	obs.LatencyMs = time.Since(start).Milliseconds()
	if err != nil {
		publish(result, actions.ObserveResult{Value: obs, AsOf: time.Now(), Error: err.Error()})
		return result, nil
	}
	defer resp.Body.Close() //nolint:errcheck

	obs.Reachable = true
	obs.StatusCode = resp.StatusCode

	if len(o.CaptureHeaders) > 0 {
		obs.Headers = make(map[string]string, len(o.CaptureHeaders))
		for _, name := range o.CaptureHeaders {
			if v := resp.Header.Get(name); v != "" {
				obs.Headers[name] = v
			}
		}
	}

	// Read up to bodySampleBytes; discard the rest.
	body, _ := io.ReadAll(io.LimitReader(resp.Body, bodySampleBytes))
	obs.BodySample = string(body)

	// Spec-59 G6: ExpectStatus=N sets Found=false when StatusCode != N.
	found := true
	if o.ExpectStatus != 0 && obs.StatusCode != o.ExpectStatus {
		found = false
	}

	publish(result, actions.ObserveResult{Found: found, Value: obs, AsOf: time.Now()})
	return result, nil
}

func publish(r *executor.Result, env actions.ObserveResult) {
	r.SetData(map[string]any{
		"found": env.Found,
		"value": actions.ObserveValueToMap(env.Value),
		"as_of": env.AsOf.Format(time.RFC3339),
		"error": env.Error,
	})
}

// --- Spec-22 ABI no-mutation specialization ---------------------------------

func (h *Handler) Cost(_ actions.Context, _ *config.Step) (actions.CostEstimate, error) {
	return actions.CostEstimate{Resources: 0, Bytes: 0, Reversible: true, Risk: 1}, nil
}

func (h *Handler) Permissions(_ *config.Step) actions.PermissionSet {
	return actions.PermissionSet{
		Network: true,
		Notes:   []string{"read-only observation; opens outbound HTTP"},
	}
}

func (h *Handler) Diff(_ actions.Context, step *config.Step) (actions.Diff, error) {
	o := step.ObserveHTTP
	if o == nil {
		return actions.Diff{}, nil
	}
	method := o.Method
	if method == "" {
		method = defaultMethod
	}
	return actions.Diff{
		Resource: actions.ResourceRef{
			Kind:       actions.ResourceOther,
			Identifier: strings.ToUpper(method) + " " + o.URL,
			Attributes: map[string]string{"observe_kind": "http"},
		},
		Operation: actions.OpNoop,
	}, nil
}

func (h *Handler) Reverse(_ actions.Context, _ *config.Step, _ actions.Result) (*config.Step, error) {
	return nil, nil
}
