// Package wait_command implements the wait.command action: poll a shell
// command until it exits with the expected code or the timeout elapses.
package wait_command

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	defaultTimeout      = 60 * time.Second
	defaultPollInterval = time.Second
	minPollInterval     = 100 * time.Millisecond
)

// Handler implements the wait.command action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "wait.command",
		Description:        "Wait for a shell command to exit with the expected code",
		Category:           actions.CategoryCommand,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.WaitCommand == nil {
		return fmt.Errorf("wait.command requires configuration")
	}
	if step.WaitCommand.Cmd == "" {
		return fmt.Errorf("wait.command: cmd is required")
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("wait.command: invalid context type")
	}
	w := step.WaitCommand

	cmd, err := ctx.GetTemplate().Render(w.Cmd, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.command.cmd", Cause: err}
	}

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		r.WouldChange = true
		r.Reason = fmt.Sprintf("would wait for command exit %d: %s", w.ExpectExit, cmd)
		return r, nil
	}

	timeout, interval, err := parseTimings(w.Timeout, w.PollInterval)
	if err != nil {
		return nil, err
	}

	ctx.GetLogger().Infof("Waiting for command %q (timeout: %s, interval: %s, expect_exit: %d)",
		cmd, timeout, interval, w.ExpectExit)

	pollCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	iterations := 0
	var lastExit int

	check := func() bool {
		iterations++
		// #nosec G204 -- Shell command from user config is intentional.
		shellCmd := exec.CommandContext(pollCtx, "bash", "-c", cmd)
		shellCmd.Dir = ec.CurrentDir
		runErr := shellCmd.Run()
		if runErr == nil {
			lastExit = 0
			return w.ExpectExit == 0
		}
		if exitErr, isExit := errors.AsType[*exec.ExitError](runErr); isExit {
			lastExit = exitErr.ExitCode()
			return lastExit == w.ExpectExit
		}
		// Command failed to start (e.g. context cancelled, missing binary);
		// don't propagate — keep polling until timeout.
		lastExit = -1
		return false
	}

	err = poll(pollCtx, interval, check)
	elapsed := time.Since(start)

	result := executor.NewResult()
	result.Changed = false
	result.Data = map[string]any{
		"cmd":        cmd,
		"elapsed_ms": elapsed.Milliseconds(),
		"iterations": iterations,
		"last_exit":  lastExit,
		"success":    err == nil,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("wait.command timeout after %s (%d attempts); last exit %d",
				elapsed.Round(time.Millisecond), iterations, lastExit)
		}
		return result, err
	}

	ctx.GetLogger().Infof("Command satisfied after %s (%d attempts)", elapsed.Round(time.Millisecond), iterations)
	return result, nil
}

func parseTimings(timeoutStr, intervalStr string) (time.Duration, time.Duration, error) {
	timeout := defaultTimeout
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.command: invalid timeout: %w", err)
		}
		timeout = d
	}
	interval := defaultPollInterval
	if intervalStr != "" {
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.command: invalid poll_interval: %w", err)
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
