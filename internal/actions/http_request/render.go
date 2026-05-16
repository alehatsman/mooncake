package http_request

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// render walks an HTTPRequest, resolves every template-bearing string
// against ctx variables, builds the body bytes, merges auth into
// headers, and packages everything into a renderedRequest ready for
// runPlan / runApply to consume. Field rendering errors are returned
// as executor.RenderError so cmd-side messages carry the field name.
func (h *Handler) render(ctx actions.Context, step *config.Step) (*renderedRequest, error) {
	r := step.HTTPRequest
	tpl := ctx.GetTemplate()
	vars := ctx.GetVariables()

	method, err := normalizeMethod(r.Method)
	if err != nil {
		return nil, err
	}

	renderedURL, err := tpl.Render(r.URL, vars)
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".url", Cause: err}
	}

	headers := make(map[string]string, len(r.Headers))
	for k, v := range r.Headers {
		rv, err := tpl.Render(v, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: actionName + ".headers." + k, Cause: err}
		}
		headers[k] = rv
	}

	body, err := tpl.Render(r.Body, vars)
	if err != nil {
		return nil, &executor.RenderError{Field: actionName + ".body", Cause: err}
	}

	form := make(map[string]string, len(r.Form))
	for k, v := range r.Form {
		rv, err := tpl.Render(v, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: actionName + ".form." + k, Cause: err}
		}
		form[k] = rv
	}

	// File path is rendered (but not expanded — keeping Wave 1 simple;
	// file: today is best-paired with an absolute path or one already
	// resolved via vars).
	if r.File != "" {
		rendered, err := tpl.Render(r.File, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: actionName + ".file", Cause: err}
		}
		// Swap into a temporary copy so we don't mutate the user's
		// declared struct.
		rCopy := *r
		rCopy.File = rendered
		r = &rCopy
	}

	bodyBytes, defaultCT, err := buildBody(r, body, form)
	if err != nil {
		return nil, err
	}

	// Auth → headers. Validate() guaranteed at most one form set.
	sensitive := defaultSensitiveHeaders()
	if r.Auth != nil {
		switch {
		case r.Auth.Bearer != "":
			val, err := tpl.Render(r.Auth.Bearer, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: actionName + ".auth.bearer", Cause: err}
			}
			headers["Authorization"] = "Bearer " + val
			sensitive["authorization"] = true
		case r.Auth.Basic != nil:
			u, err := tpl.Render(r.Auth.Basic.User, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: actionName + ".auth.basic.user", Cause: err}
			}
			p, err := tpl.Render(r.Auth.Basic.Pass, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: actionName + ".auth.basic.pass", Cause: err}
			}
			encoded := base64.StdEncoding.EncodeToString([]byte(u + ":" + p))
			headers["Authorization"] = "Basic " + encoded
			sensitive["authorization"] = true
		case r.Auth.Header != nil:
			name, err := tpl.Render(r.Auth.Header.Name, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: actionName + ".auth.header.name", Cause: err}
			}
			val, err := tpl.Render(r.Auth.Header.Value, vars)
			if err != nil {
				return nil, &executor.RenderError{Field: actionName + ".auth.header.value", Cause: err}
			}
			headers[name] = val
			sensitive[strings.ToLower(name)] = true
		}
	}

	// IdempotencyKey, if set, ships as a standard header. Render so
	// users can interpolate run IDs etc.
	if r.IdempotencyKey != "" {
		val, err := tpl.Render(r.IdempotencyKey, vars)
		if err != nil {
			return nil, &executor.RenderError{Field: actionName + ".idempotency_key", Cause: err}
		}
		headers["Idempotency-Key"] = val
	}

	timeout := defaultTimeout
	if r.Timeout != "" {
		// Already validated in Validate().
		timeout, _ = time.ParseDuration(r.Timeout)
	}
	retryDelay := defaultRetryDelay
	if r.RetryDelay != "" {
		retryDelay, _ = time.ParseDuration(r.RetryDelay)
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

	rr := &renderedRequest{
		method:           method,
		url:              renderedURL,
		headers:          headers,
		bodyBytes:        bodyBytes,
		contentType:      defaultCT,
		timeout:          timeout,
		retryDelay:       retryDelay,
		retries:          r.Retries,
		retryOn:          retryOn,
		expectStatus:     r.ExpectStatus,
		skipTLSVerify:    r.SkipTLSVerify,
		redactBody:       r.RedactBody,
		maxRespBytes:     maxResp,
		followRedir:      follow,
		sensitiveHeaders: sensitive,
	}
	return rr, nil
}
