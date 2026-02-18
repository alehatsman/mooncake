// Package wait implements the wait action handler.
// Polls a condition until it becomes true or times out.
package wait

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	defaultTimeout  = "30s"
	defaultInterval = "1s"
	minInterval     = 100 * time.Millisecond

	// Condition type constants
	conditionFileExists = "file_exists"
	conditionFileAbsent = "file_absent"
	conditionGitClean   = "git_clean"
	conditionCommand    = "command"
	conditionHTTP       = "http"
	conditionPort       = "port"
)

// Handler implements the wait action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "wait",
		Description:        "Poll a condition until it becomes true or times out",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate validates the wait action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.Wait == nil {
		return fmt.Errorf("wait action requires wait configuration")
	}

	wait := step.Wait

	// Validate condition type
	validConditions := []string{conditionFileExists, conditionFileAbsent, conditionGitClean, conditionCommand, conditionHTTP, conditionPort}
	valid := false
	for _, cond := range validConditions {
		if wait.Condition == cond {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid condition type: %s (must be one of: %s)", wait.Condition, strings.Join(validConditions, ", "))
	}

	// Validate condition-specific fields
	switch wait.Condition {
	case conditionFileExists, conditionFileAbsent:
		if wait.Path == nil {
			return fmt.Errorf("wait.%s requires path field", wait.Condition)
		}
	case conditionGitClean:
		// No required fields
	case conditionCommand:
		if wait.Cmd == nil {
			return fmt.Errorf("wait.command requires cmd field")
		}
	case conditionHTTP:
		if wait.URL == nil {
			return fmt.Errorf("wait.http requires url field")
		}
	case conditionPort:
		if wait.Port == nil {
			return fmt.Errorf("wait.port requires port field")
		}
	}

	return nil
}

// Execute executes the wait action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	wait := step.Wait

	// Parse timeout
	timeout := defaultTimeout
	if wait.Timeout != "" {
		timeout = wait.Timeout
	}
	timeoutDuration, err := time.ParseDuration(timeout)
	if err != nil {
		return nil, fmt.Errorf("invalid timeout duration: %w", err)
	}

	// Parse interval
	interval := defaultInterval
	if wait.Interval != "" {
		interval = wait.Interval
	}
	intervalDuration, err := time.ParseDuration(interval)
	if err != nil {
		return nil, fmt.Errorf("invalid interval duration: %w", err)
	}
	if intervalDuration < minInterval {
		intervalDuration = minInterval
	}

	ec.Logger.Infof("Waiting for condition: %s (timeout: %s, interval: %s)", wait.Condition, timeout, interval)

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeoutDuration)
	defer cancel()

	// Create condition checker
	checker, checkerErr := h.createChecker(wait, ec)
	if checkerErr != nil {
		return nil, checkerErr
	}

	// Poll condition
	startTime := time.Now()
	iterations := 0

	err = h.pollCondition(timeoutCtx, checker, intervalDuration, &iterations, ec)
	elapsed := time.Since(startTime)

	// Create result
	result := executor.NewResult()
	result.Changed = false // Waiting doesn't change state
	result.Data = map[string]interface{}{
		"condition":   wait.Condition,
		"elapsed_ms":  elapsed.Milliseconds(),
		"iterations":  iterations,
		"success":     err == nil,
	}

	if err != nil {
		if err == context.DeadlineExceeded {
			return result, fmt.Errorf("wait timeout after %s (%d iterations)", elapsed.Round(time.Millisecond), iterations)
		}
		return result, err
	}

	ec.Logger.Infof("Condition met after %s (%d iterations)", elapsed.Round(time.Millisecond), iterations)
	return result, nil
}

// DryRun logs what the wait action would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	wait := step.Wait
	timeout := defaultTimeout
	if wait.Timeout != "" {
		timeout = wait.Timeout
	}
	interval := defaultInterval
	if wait.Interval != "" {
		interval = wait.Interval
	}

	ec.Logger.Infof("  [DRY-RUN] Would wait for condition: %s (timeout: %s, interval: %s)", wait.Condition, timeout, interval)

	switch wait.Condition {
	case conditionFileExists:
		ec.Logger.Infof("  [DRY-RUN] Would wait for file to exist: %s", *wait.Path)
	case conditionFileAbsent:
		ec.Logger.Infof("  [DRY-RUN] Would wait for file to be absent: %s", *wait.Path)
	case conditionGitClean:
		if wait.AllowUntracked != nil && *wait.AllowUntracked {
			ec.Logger.Infof("  [DRY-RUN] Would wait for git working tree to be clean (untracked files allowed)")
		} else {
			ec.Logger.Infof("  [DRY-RUN] Would wait for git working tree to be clean (no untracked files)")
		}
	case conditionCommand:
		exitCode := 0
		if wait.ExitCode != nil {
			exitCode = *wait.ExitCode
		}
		ec.Logger.Infof("  [DRY-RUN] Would wait for command success: %s (exit code: %d)", *wait.Cmd, exitCode)
	case conditionHTTP:
		status := 200
		if wait.Status != nil {
			status = *wait.Status
		}
		ec.Logger.Infof("  [DRY-RUN] Would wait for HTTP %s (status: %d)", *wait.URL, status)
	case conditionPort:
		host := "localhost"
		if wait.Host != nil {
			host = *wait.Host
		}
		ec.Logger.Infof("  [DRY-RUN] Would wait for port to be open: %s:%d", host, *wait.Port)
	}

	return nil
}

// createChecker creates a condition checker function.
func (h *Handler) createChecker(wait *config.WaitAction, ec *executor.ExecutionContext) (func() (bool, error), error) {
	switch wait.Condition {
	case conditionFileExists:
		return h.createFileExistsChecker(*wait.Path, ec, true)
	case conditionFileAbsent:
		return h.createFileExistsChecker(*wait.Path, ec, false)
	case conditionGitClean:
		return h.createGitCleanChecker(wait, ec)
	case conditionCommand:
		return h.createCommandChecker(wait, ec)
	case conditionHTTP:
		return h.createHTTPChecker(wait, ec)
	case conditionPort:
		return h.createPortChecker(wait, ec)
	default:
		return nil, fmt.Errorf("unsupported condition type: %s", wait.Condition)
	}
}

// pollCondition polls a condition until it returns true or context times out.
func (h *Handler) pollCondition(ctx context.Context, check func() (bool, error), interval time.Duration, iterations *int, _ *executor.ExecutionContext) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately
	*iterations++
	if ok, err := check(); err != nil {
		return err
	} else if ok {
		return nil
	}

	// Poll until timeout or success
	for {
		select {
		case <-ctx.Done():
			return ctx.Err() // Timeout
		case <-ticker.C:
			*iterations++
			if ok, err := check(); err != nil {
				return err
			} else if ok {
				return nil
			}
		}
	}
}

// createFileExistsChecker creates a file existence checker.
func (h *Handler) createFileExistsChecker(path string, ec *executor.ExecutionContext, shouldExist bool) (func() (bool, error), error) {
	// Render path with variables
	renderedPath, err := ec.Template.Render(path, ec.Variables)
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.path", Cause: err}
	}

	// Expand path
	expandedPath, expandErr := ec.PathUtil.ExpandPath(renderedPath, ec.CurrentDir, ec.Variables)
	if expandErr != nil {
		return nil, &executor.FileOperationError{
			Operation: "expand path",
			Path:      renderedPath,
			Cause:     expandErr,
		}
	}

	return func() (bool, error) {
		_, err := os.Stat(expandedPath)
		exists := err == nil
		if shouldExist {
			return exists, nil
		}
		return !exists, nil
	}, nil
}

// createGitCleanChecker creates a git clean status checker.
func (h *Handler) createGitCleanChecker(wait *config.WaitAction, ec *executor.ExecutionContext) (func() (bool, error), error) {
	allowUntracked := false
	if wait.AllowUntracked != nil {
		allowUntracked = *wait.AllowUntracked
	}

	return func() (bool, error) {
		// Check if we're in a git repository
		// #nosec G204 -- Git command is controlled and safe
		checkCmd := exec.Command("git", "rev-parse", "--git-dir")
		checkCmd.Dir = ec.CurrentDir
		if err := checkCmd.Run(); err != nil {
			return false, fmt.Errorf("not a git repository: %s", ec.CurrentDir)
		}

		// Get git status
		// #nosec G204 -- Git command is controlled and safe
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = ec.CurrentDir
		output, err := statusCmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("git status failed: %w", err)
		}

		statusOutput := strings.TrimSpace(string(output))

		// If allow_untracked is true, filter out untracked files
		if allowUntracked && statusOutput != "" {
			lines := strings.Split(statusOutput, "\n")
			var trackedChanges []string
			for _, line := range lines {
				if len(line) >= 2 && line[0:2] != "??" {
					trackedChanges = append(trackedChanges, line)
				}
			}
			statusOutput = strings.Join(trackedChanges, "\n")
		}

		// Clean if no output
		return statusOutput == "", nil
	}, nil
}

// createCommandChecker creates a command success checker.
func (h *Handler) createCommandChecker(wait *config.WaitAction, ec *executor.ExecutionContext) (func() (bool, error), error) {
	// Render command with variables
	cmd, err := ec.Template.Render(*wait.Cmd, ec.Variables)
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.cmd", Cause: err}
	}

	expectedExitCode := 0
	if wait.ExitCode != nil {
		expectedExitCode = *wait.ExitCode
	}

	return func() (bool, error) {
		// Execute command
		// #nosec G204 -- Command from user config is intentional functionality
		shellCmd := exec.Command("bash", "-c", cmd)
		shellCmd.Dir = ec.CurrentDir

		err := shellCmd.Run()
		exitCode := 0
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				exitCode = exitError.ExitCode()
			} else {
				// Command failed to start
				return false, nil // Don't error, just return false to continue polling
			}
		}

		return exitCode == expectedExitCode, nil
	}, nil
}

// createHTTPChecker creates an HTTP endpoint checker.
func (h *Handler) createHTTPChecker(wait *config.WaitAction, ec *executor.ExecutionContext) (func() (bool, error), error) {
	// Render URL with variables
	url, err := ec.Template.Render(*wait.URL, ec.Variables)
	if err != nil {
		return nil, &executor.RenderError{Field: "wait.url", Cause: err}
	}

	expectedStatus := 200
	if wait.Status != nil {
		expectedStatus = *wait.Status
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	return func() (bool, error) {
		resp, err := client.Get(url) //nolint:noctx,gosec // Context not available in checker, URL from config is intentional
		if err != nil {
			return false, nil // Don't error, just return false to continue polling
		}
		defer func() {
			_ = resp.Body.Close() // Best effort close
		}()

		return resp.StatusCode == expectedStatus, nil
	}, nil
}

// createPortChecker creates a TCP port connectivity checker.
func (h *Handler) createPortChecker(wait *config.WaitAction, ec *executor.ExecutionContext) (func() (bool, error), error) {
	host := "localhost"
	if wait.Host != nil {
		// Render host with variables
		renderedHost, err := ec.Template.Render(*wait.Host, ec.Variables)
		if err != nil {
			return nil, &executor.RenderError{Field: "wait.host", Cause: err}
		}
		host = renderedHost
	}

	port := *wait.Port
	address := net.JoinHostPort(host, fmt.Sprintf("%d", port))

	return func() (bool, error) {
		conn, err := net.DialTimeout("tcp", address, 2*time.Second)
		if err != nil {
			return false, nil // Don't error, just return false to continue polling
		}
		_ = conn.Close() // Best effort close
		return true, nil
	}, nil
}
