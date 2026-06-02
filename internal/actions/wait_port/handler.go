// Package wait_port implements the wait.port action: poll a TCP
// endpoint until it accepts connections or the timeout elapses.
package wait_port

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	defaultTimeout      = 60 * time.Second
	defaultPollInterval = time.Second
	minPollInterval     = 100 * time.Millisecond
	dialTimeout         = 2 * time.Second
)

// Handler implements the wait.port action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "wait.port",
		Description:        "Wait for a TCP port to accept connections",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.WaitPort == nil {
		return fmt.Errorf("wait.port requires configuration")
	}
	if step.WaitPort.Port <= 0 || step.WaitPort.Port > 65535 {
		return fmt.Errorf("wait.port: port must be 1..65535, got %d", step.WaitPort.Port)
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	w := step.WaitPort
	host, err := resolveHost(ctx, w.Host)
	if err != nil {
		return nil, err
	}
	address := net.JoinHostPort(host, strconv.Itoa(w.Port))

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true
		r.Reason = "would wait for TCP port: " + address
		return r, nil
	}

	// MT-42: accept `interval:` as an alias for `poll_interval:`.
	intervalField := w.PollInterval
	if intervalField == "" {
		intervalField = w.Interval
	}
	timeout, interval, err := parseTimings(w.Timeout, intervalField)
	if err != nil {
		return nil, err
	}

	ctx.Logger().Infof("Waiting for TCP port %s (timeout: %s, interval: %s)", address, timeout, interval)

	// Derive the poll deadline from the run-wide context so a
	// run-wide cancellation (Ctrl-C, fleet kill, MCP shutdown) aborts
	// the wait, not just the action's own timeout.
	pollCtx, cancel := context.WithTimeout(ctx.Ctx(), timeout)
	defer cancel()

	start := time.Now()
	iterations := 0
	check := func() bool {
		iterations++
		conn, dialErr := net.DialTimeout("tcp", address, dialTimeout)
		if dialErr != nil {
			return false
		}
		_ = conn.Close()
		return true
	}

	err = poll(pollCtx, interval, check)
	elapsed := time.Since(start)

	result := executor.NewResult()
	result.Operation = executor.OpQuery
	result.Target = address
	result.Changed = false
	result.Data = map[string]any{
		"address":    address,
		"elapsed_ms": elapsed.Milliseconds(),
		"iterations": iterations,
		"success":    err == nil,
	}

	if err != nil {
		if err == context.DeadlineExceeded {
			return result, fmt.Errorf("wait.port timeout after %s (%d attempts) waiting for %s",
				elapsed.Round(time.Millisecond), iterations, address)
		}
		return result, err
	}

	ctx.Logger().Infof("Port %s open after %s (%d attempts)", address, elapsed.Round(time.Millisecond), iterations)
	return result, nil
}

func resolveHost(ctx actions.Context, host string) (string, error) {
	if host == "" {
		return "localhost", nil
	}
	rendered, err := ctx.Template().Render(host, ctx.Variables())
	if err != nil {
		return "", &executor.RenderError{Field: "wait.port.host", Cause: err}
	}
	return rendered, nil
}

func parseTimings(timeoutStr, intervalStr string) (time.Duration, time.Duration, error) {
	timeout := defaultTimeout
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.port: invalid timeout: %w", err)
		}
		timeout = d
	}
	interval := defaultPollInterval
	if intervalStr != "" {
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.port: invalid poll_interval: %w", err)
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
