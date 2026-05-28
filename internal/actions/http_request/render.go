package http_request

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// render resolves an HTTPRequest's templates against vars and packages
// the execution-ready bytes, headers, and auth into a renderedRequest.
// Used by both top-level Run and Wave 2 sub-requests (probe / reverse).
//
// fieldPrefix ("", "probe.", "reverse.") is prepended to render-error
// field names so the operator sees which level rejected.
func (h *Handler) render(ctx actions.Context, r *config.HTTPRequest, vars map[string]interface{}, fieldPrefix string) (*renderedRequest, error) {
	resolved, err := renderConfig(ctx, r, vars, fieldPrefix)
	if err != nil {
		return nil, err
	}
	return buildExec(resolved, fieldPrefix)
}

// renderConfig walks an HTTPRequest and returns a NEW HTTPRequest with
// every template-bearing string resolved against vars. Non-template
// fields (Retries, FollowRedirects, RedactBody, …) are copied by
// value. Probe/Reverse sub-blocks are NOT recursively rendered here —
// they're handled by the caller (plan-mode probe path / apply-time
// reverse path) so each can use its own vars merge.
//
// Output has all templates resolved; safe to pass to buildExec, to
// stash on Result.ReverseData, or to feed back through the executor
// as a Step (the executor's template pass on Run is idempotent on
// already-resolved strings since no `{{` / `{%` remain).
func renderConfig(ctx actions.Context, r *config.HTTPRequest, vars map[string]interface{}, fieldPrefix string) (*config.HTTPRequest, error) {
	tpl := ctx.Template()
	at := func(field string) string { return actionName + "." + fieldPrefix + field }

	out := *r // shallow copy; we'll overwrite the template-bearing fields below

	url, err := tpl.Render(r.URL, vars)
	if err != nil {
		return nil, &executor.RenderError{Field: at("url"), Cause: err}
	}
	out.URL = url

	if len(r.Headers) > 0 {
		hdrs := make(map[string]string, len(r.Headers))
		for k, v := range r.Headers {
			rv, err := tpl.Render(v, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: at("headers." + k), Cause: err}
			}
			hdrs[k] = rv
		}
		out.Headers = hdrs
	}

	body, err := tpl.Render(r.Body, vars)
	if err != nil {
		return nil, &executor.RenderError{Field: at("body"), Cause: err}
	}
	out.Body = body

	if len(r.Form) > 0 {
		form := make(map[string]string, len(r.Form))
		for k, v := range r.Form {
			rv, err := tpl.Render(v, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: at("form." + k), Cause: err}
			}
			form[k] = rv
		}
		out.Form = form
	}

	if r.File != "" {
		rendered, err := tpl.Render(r.File, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("file"), Cause: err}
		}
		out.File = rendered
	}

	if r.Auth != nil {
		authCopy, err := renderAuth(tpl, r.Auth, vars, at)
		if err != nil {
			return nil, err
		}
		out.Auth = authCopy
	}

	if r.IdempotencyKey != "" {
		rv, err := tpl.Render(r.IdempotencyKey, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("idempotency_key"), Cause: err}
		}
		out.IdempotencyKey = rv
	}

	// Probe / Reverse are kept as-is; the caller handles them. JSON
	// is also passed through unchanged — Wave 1 documented that
	// templates inside structured `json:` are not rendered.
	return &out, nil
}

// renderAuth resolves the templated fields of an HTTPAuth and returns
// a new struct.
func renderAuth(tpl interface {
	Render(s string, vars map[string]interface{}) (string, error)
}, in *config.HTTPAuth, vars map[string]interface{}, at func(string) string) (*config.HTTPAuth, error) {
	out := *in
	if in.Bearer != "" {
		rv, err := tpl.Render(in.Bearer, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("auth.bearer"), Cause: err}
		}
		out.Bearer = rv
	}
	if in.Basic != nil {
		u, err := tpl.Render(in.Basic.User, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("auth.basic.user"), Cause: err}
		}
		p, err := tpl.Render(in.Basic.Pass, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("auth.basic.pass"), Cause: err}
		}
		basic := *in.Basic
		basic.User = u
		basic.Pass = p
		out.Basic = &basic
	}
	if in.Header != nil {
		name, err := tpl.Render(in.Header.Name, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("auth.header.name"), Cause: err}
		}
		val, err := tpl.Render(in.Header.Value, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: at("auth.header.value"), Cause: err}
		}
		hdr := *in.Header
		hdr.Name = name
		hdr.Value = val
		out.Header = &hdr
	}
	return &out, nil
}

// buildExec takes a template-resolved HTTPRequest and assembles the
// renderedRequest used by sendOnce. Does NOT call the template
// renderer; assumes renderConfig has already run.
func buildExec(r *config.HTTPRequest, fieldPrefix string) (*renderedRequest, error) {
	method, err := normalizeMethod(r.Method)
	if err != nil {
		return nil, err
	}

	headers := make(map[string]string, len(r.Headers))
	for k, v := range r.Headers {
		headers[k] = v
	}

	bodyBytes, defaultCT, err := buildBody(r, r.Body, r.Form)
	if err != nil {
		return nil, err
	}

	sensitive := defaultSensitiveHeaders()
	if r.Auth != nil {
		switch {
		case r.Auth.Bearer != "":
			headers["Authorization"] = "Bearer " + r.Auth.Bearer
			sensitive["authorization"] = true
		case r.Auth.Basic != nil:
			encoded := base64.StdEncoding.EncodeToString([]byte(r.Auth.Basic.User + ":" + r.Auth.Basic.Pass))
			headers["Authorization"] = "Basic " + encoded
			sensitive["authorization"] = true
		case r.Auth.Header != nil:
			headers[r.Auth.Header.Name] = r.Auth.Header.Value
			sensitive[strings.ToLower(r.Auth.Header.Name)] = true
		}
	}

	if r.IdempotencyKey != "" {
		headers["Idempotency-Key"] = r.IdempotencyKey
	}

	timeout := defaultTimeout
	if r.Timeout != "" {
		timeout, _ = time.ParseDuration(r.Timeout)
	}

	retryOn := make(map[string]bool, len(r.RetryOn))
	for _, t := range r.RetryOn {
		retryOn[strings.ToLower(t)] = true
	}

	maxResp := r.MaxResponseBytes
	if maxResp == 0 {
		maxResp = defaultMaxResponseBytes
	}

	follow := defaultFollowRedirects
	if r.FollowRedirects != nil {
		follow = *r.FollowRedirects
	}

	_ = fieldPrefix // reserved for future error-context use; keep symmetric with render

	return &renderedRequest{
		method:           method,
		url:              r.URL,
		headers:          headers,
		bodyBytes:        bodyBytes,
		contentType:      defaultCT,
		timeout:          timeout,
		retryOn:          retryOn,
		expectStatus:     r.ExpectStatus,
		skipTLSVerify:    r.SkipTLSVerify,
		redactBody:       r.RedactBody,
		maxRespBytes:     maxResp,
		followRedir:      follow,
		sensitiveHeaders: sensitive,
	}, nil
}
