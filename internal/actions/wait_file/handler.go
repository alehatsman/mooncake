// Package wait_file implements the wait.file action: poll a filesystem
// path until it exists (and, optionally, its contents include a required
// substring).
package wait_file

import (
	"context"
	"errors"
	"fmt"
	"os"
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
)

// Handler implements the wait.file action.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "wait.file",
		Description:        "Wait for a file or directory to exist (optionally containing a substring)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{},
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

func (h *Handler) Validate(step *config.Step) error {
	if step.WaitFile == nil {
		return fmt.Errorf("wait.file requires configuration")
	}
	if strings.TrimSpace(step.WaitFile.Path) == "" {
		return fmt.Errorf("wait.file: path is required")
	}
	return nil
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("wait.file: invalid context type")
	}
	w := step.WaitFile

	rendered, err := ctx.GetTemplate().Render(w.Path, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.file.path", Cause: err}
	}
	path, err := ec.Svc.PathUtil.ExpandPath(rendered, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, &executor.FileOperationError{
			Operation: "expand path",
			Path:      rendered,
			Cause:     err,
		}
	}

	contains, err := ctx.GetTemplate().Render(w.Contains, ctx.GetVariables())
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.file.contains", Cause: err}
	}

	if ctx.Mode() == actions.ModePlan {
		r := executor.NewResult()
		r.Checkable = true
		if ok, _ := checkFile(path, contains); ok {
			r.Reason = fmt.Sprintf("file already satisfies wait: %s", path)
			return r, nil
		}
		r.WouldChange = true
		if contains != "" {
			r.Reason = fmt.Sprintf("would wait for file %s to contain %q", path, contains)
		} else {
			r.Reason = "would wait for file: " + path
		}
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

	ctx.GetLogger().Infof("Waiting for file %s (timeout: %s, interval: %s)", path, timeout, interval)

	pollCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	start := time.Now()
	iterations := 0
	check := func() bool {
		iterations++
		ok, _ := checkFile(path, contains)
		return ok
	}

	err = poll(pollCtx, interval, check)
	elapsed := time.Since(start)

	result := executor.NewResult()
	result.Changed = false
	result.Data = map[string]any{
		"path":       path,
		"elapsed_ms": elapsed.Milliseconds(),
		"iterations": iterations,
		"success":    err == nil,
	}

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return result, fmt.Errorf("wait.file timeout after %s (%d checks) waiting for %s",
				elapsed.Round(time.Millisecond), iterations, path)
		}
		return result, err
	}

	ctx.GetLogger().Infof("File %s ready after %s (%d checks)", path, elapsed.Round(time.Millisecond), iterations)
	return result, nil
}

// checkFile returns true if path exists and (when needle != "") its
// contents include needle. Read errors map to false so the poll keeps
// going; we surface only the final timeout.
func checkFile(path, needle string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	if needle == "" {
		return true, nil
	}
	if info.IsDir() {
		return false, nil
	}
	data, err := os.ReadFile(path) //nolint:gosec // Path comes from user config.
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), needle), nil
}

func parseTimings(timeoutStr, intervalStr string) (time.Duration, time.Duration, error) {
	timeout := defaultTimeout
	if timeoutStr != "" {
		d, err := time.ParseDuration(timeoutStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.file: invalid timeout: %w", err)
		}
		timeout = d
	}
	interval := defaultPollInterval
	if intervalStr != "" {
		d, err := time.ParseDuration(intervalStr)
		if err != nil {
			return 0, 0, fmt.Errorf("wait.file: invalid poll_interval: %w", err)
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
