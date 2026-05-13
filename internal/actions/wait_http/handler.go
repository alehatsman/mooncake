// Package wait_http implements the wait.http action: poll an HTTP
// endpoint until it returns one of the accepted statuses (and, optionally,
// the body contains a required substring).
package wait_http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	defaultTimeout      = 60 * time.Second
	defaultPollInterval = time.Second
	minPollInterval     = 100 * time.Millisecond
	requestTimeout      = 5 * time.Second
)

// Handler implements the wait.http action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "wait.http",
		Description:        "Wait for an HTTP endpoint to return an accepted status",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.WaitHTTP == nil {
		return fmt.Errorf("wait.http requires configuration")
	}
	if strings.TrimSpace(step.WaitHTTP.URL) == "" {
		return fmt.Errorf("wait.http: url is required")
	}
	for _, s := range step.WaitHTTP.Status {
		if s < 100 || s > 599 {
			return fmt.Errorf("wait.http: invalid status code %d", s)
		}
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	w := step.WaitHTTP
	url, err := ctx.GetTemplate().Render(w.URL, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.http.url", Cause: err}
	}

	method := strings.ToUpper(strings.TrimSpace(w.Method))
	if method == "" {
		method = http.MethodGet
	}

	accepted := w.Status
	if len(accepted) == 0 {
		accepted = []int{http.StatusOK}
	}

	bodyContains, err := ctx.GetTemplate().Render(w.BodyContains, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.http.body_contains", Cause: err}
	}

	headers, err := renderHeaders(ctx, w.Headers)
	if err != nil {
		return nil, err
	}

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would wait for %s %s (status %v)", method, url, accepted)
		return r, nil
	}

	timeout, interval, err := parseTimings(w.Timeout, w.PollInterval)
	if err != nil {
		return nil, err
	}

	ctx.GetLogger().Infof("Waiting for HTTP %s %s (timeout: %s, interval: %s)", method, url, timeout, interval)

	client := &http.Client{Timeout: requestTimeout}
	pollCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	iterations := 0
	var lastStatus int

	check := func() bool {
		iterations++
		req, reqErr := http.NewRequestWithContext(pollCtx, method, url, http.NoBody)
		if reqErr != nil {
			return false
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, doErr := client.Do(req)
		if doErr != nil {
			return false
		}
		defer func() { _ = resp.Body.Close() }()

		lastStatus = resp.StatusCode
		if !slices.Contains(accepted, resp.StatusCode) {
			return false
		}
		if bodyContains == "" {
			return true
		}
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return false
		}
		return strings.Contains(string(body), bodyContains)
	}

	err = poll(pollCtx, interval, check)
	elapsed := time.Since(start)

	result := executor.NewResult()
	result.Changed = false
	result.Data = map[string]any{
		"url":         url,
		"method":      method,
		"elapsed_ms":  elapsed.Milliseconds(),
		"iterations":  iterations,
		"last_status": lastStatus,
		"success":     err == nil,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("wait.http timeout after %s (%d attempts) for %s; last status %d",
				elapsed.Round(time.Millisecond), iterations, url, lastStatus)
		}
		return result, err
	}

	ctx.GetLogger().Infof("HTTP %s ready after %s (%d attempts)", url, elapsed.Round(time.Millisecond), iterations)
	return result, nil
}

func renderHeaders(ctx actions.Context, in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		rendered, err := ctx.GetTemplate().Render(v, ctx.GetVariables())
		if err != nil {
			return nil, &executor.RenderError{Field: "wait.http.headers." + k, Cause: err}
		}
		out[k] = rendered
	}
	return out, nil
}

func parseTimings(timeoutStr, intervalStr string) (time.Duration, time.Duration, error) {
	timeout := defaultTimeout
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.http: invalid timeout: %w", err)
		}
		timeout = d
	}
	interval := defaultPollInterval
	if intervalStr != "" {
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.http: invalid poll_interval: %w", err)
		}
		interval = d
	}
	if interval < minPollInterval {
		interval = minPollInterval
	}
	return timeout, interval, nil
}

func poll(ctx context.Context, interval time.Duration, check func() bool) error {
	if check() {
		return nil
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if check() {
				return nil
			}
		}
	}
}
