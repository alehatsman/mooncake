// Package notify is a reference Tier-2 action pack built entirely against the
// public mooncake SDK facade (github.com/alehatsman/mooncake/sdk) — the exact
// surface an external consumer compiles its own agent on. It imports no
// internal/ package, so it doubles as a conformance test that the facade is
// self-sufficient for authoring real custom typed actions.
//
// notify.webhook POSTs a (templated) body to a URL — a low-blast-radius
// notification action. A plan reaches it through the generic carrier:
//
//   - action: notify.webhook
//     with:
//     url: https://hooks.example.com/deploy
//     method: POST            # optional; POST/PUT/PATCH, default POST
//     headers:
//     Content-Type: application/json
//     body: '{"event":"deploy","service":"{{ service }}"}'
//
// Register it into a registry and hand that to the agent loop:
//
//	reg := mooncake.DefaultRegistry()
//	_ = reg.Register(notify.WebhookHandler{})
//	_, _ = mooncake.RunLoop(ctx, mooncake.RunOptions{Registry: reg, Goal: "..."})
package notify

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// WebhookHandler implements the notify.webhook custom action against the SDK
// facade. It satisfies the core Handler ABI (Metadata/Validate/Run) plus the
// Permitter and Coster capability interfaces so the typed kernel guarantees —
// permission surfacing at plan time, blast-radius cost — extend to it.
//
// It deliberately does NOT implement Reverser: a sent notification cannot be
// unsent, so the action is irreversible by declaration (the kernel's default
// Reversible=false applies).
type WebhookHandler struct {
	// Client is the HTTP client used for the POST. Nil selects a default
	// client with a short timeout; tests inject one pointed at httptest.
	Client *http.Client
}

// compile-time proof the capability interfaces are satisfied through the facade.
var (
	_ mooncake.Handler   = WebhookHandler{}
	_ mooncake.Permitter = WebhookHandler{}
	_ mooncake.Coster    = WebhookHandler{}
)

func (WebhookHandler) Metadata() mooncake.ActionMetadata {
	return mooncake.ActionMetadata{
		Name:        "notify.webhook",
		Description: "POST a body to a webhook URL (fire-and-forget notification). Not idempotent — fires every run; gate with when: if needed.",
		EmitsEvents: []string{"notify.sent"},
		Version:     "1.0.0",
	}
}

func (WebhookHandler) Validate(step *mooncake.Step) error {
	if strings.TrimSpace(str(step.With["url"])) == "" {
		return fmt.Errorf("notify.webhook: 'url' is required under 'with'")
	}
	if m := str(step.With["method"]); m != "" {
		switch strings.ToUpper(m) {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
		default:
			return fmt.Errorf("notify.webhook: unsupported method %q (use POST, PUT, or PATCH)", m)
		}
	}
	return nil
}

// Permissions declares the action reaches the network so policy/UI surfaces
// see it at plan time rather than as a runtime surprise.
func (WebhookHandler) Permissions(*mooncake.Step) mooncake.PermissionSet {
	return mooncake.PermissionSet{
		Network: true,
		Notes:   []string{"sends an HTTP request to an external URL"},
	}
}

// Cost reports a low, irreversible blast radius: one external resource, no
// local mutation, Risk band 2 (safe-ish side effect).
func (WebhookHandler) Cost(mooncake.Context, *mooncake.Step) (mooncake.CostEstimate, error) {
	return mooncake.CostEstimate{Resources: 1, Bytes: -1, Reversible: false, Risk: 2}, nil
}

func (h WebhookHandler) Run(ctx mooncake.Context, step *mooncake.Step) (mooncake.Result, error) {
	r := mooncake.NewResult()

	// Render templated fields through the kernel renderer so the custom
	// action gets the same {{ var }} treatment as built-ins.
	url, err := ctx.Template().Render(str(step.With["url"]), ctx.Variables())
	if err != nil {
		return fail(r, "notify.webhook: render url: %v", err)
	}
	body, err := ctx.Template().Render(str(step.With["body"]), ctx.Variables())
	if err != nil {
		return fail(r, "notify.webhook: render body: %v", err)
	}
	method := strings.ToUpper(str(step.With["method"]))
	if method == "" {
		method = http.MethodPost
	}

	r.Target = url
	r.Operation = "update" // untyped constant → executor.Operation; sent ≈ an update

	// Plan mode: predict, never touch the network.
	if ctx.Mode() == mooncake.ModePlan {
		r.Changed = true
		r.Stdout = fmt.Sprintf("would %s %s (%d-byte body)", method, url, len(body))
		return r, nil
	}

	req, err := http.NewRequestWithContext(ctx.Ctx(), method, url, bytes.NewBufferString(body))
	if err != nil {
		return fail(r, "notify.webhook: build request: %v", err)
	}
	if hs, ok := step.With["headers"].(map[string]interface{}); ok {
		for k, v := range hs {
			req.Header.Set(k, str(v))
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client().Do(req)
	if err != nil {
		return fail(r, "notify.webhook: %s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return fail(r, "notify.webhook: %s returned HTTP %d", url, resp.StatusCode)
	}
	r.Changed = true
	r.Stdout = fmt.Sprintf("notified %s (HTTP %d)", url, resp.StatusCode)
	return r, nil
}

func (h WebhookHandler) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

// fail populates the result envelope per the proposal-06 contract (Failed=true
// with a non-empty Error) and returns the error so the executor surfaces it.
func fail(r *mooncake.ResultData, format string, args ...interface{}) (mooncake.Result, error) {
	msg := fmt.Sprintf(format, args...)
	r.Failed = true
	r.Rc = 1
	r.Error = msg
	return r, fmt.Errorf("%s", msg)
}

// str coerces a with: value to string without panicking on absent/typed keys.
func str(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
