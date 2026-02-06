// Package download implements the download action handler.
//
// The download action downloads files from URLs with:
// - HTTP/HTTPS support
// - Checksum verification (MD5, SHA1, SHA256) for idempotency
// - Custom HTTP headers
// - Timeout and retry support
// - Atomic write pattern (temp file + rename)
package download

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/utils"
)

const (
	defaultFileMode os.FileMode = 0644
)

// Handler implements the Handler interface for download actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the download action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "download",
		Description:        "Download files from URLs with checksum verification",
		Category:           actions.CategoryNetwork,
		SupportsDryRun:     true,
		SupportsBecome:     true,
		EmitsEvents:        []string{string(events.EventFileDownloaded)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on dest path
		ImplementsCheck:    true,       // Checks if file exists and validates checksum
	}
}

// Validate checks if the download configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Download == nil {
		return fmt.Errorf("download configuration is nil")
	}

	downloadAction := step.Download
	if downloadAction.URL == "" {
		return fmt.Errorf("url is required")
	}

	if downloadAction.Dest == "" {
		return fmt.Errorf("dest is required")
	}

	return nil
}

// Execute runs the download action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	downloadAction := step.Download

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Render URL
	renderedURL, err := ctx.GetTemplate().Render(downloadAction.URL, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to render URL: %w", err)
	}

	// Render destination path
	renderedDest, err := ec.PathUtil.ExpandPath(downloadAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Check if destination exists
	_, err = os.Stat(renderedDest)
	destExists := err == nil

	// If destination exists and checksum is provided, check if we need to re-download
	needsDownload := !destExists || downloadAction.Force
	if destExists && !downloadAction.Force && downloadAction.Checksum != "" {
		// Verify existing file checksum for idempotency
		matches, checksumErr := utils.VerifyChecksum(renderedDest, downloadAction.Checksum)
		if checksumErr != nil {
			ctx.GetLogger().Debugf("  Unable to verify checksum of existing file: %v", checksumErr)
			needsDownload = true
		} else if matches {
			// File exists with correct checksum, skip download
			ctx.GetLogger().Debugf("  File already exists with correct checksum: %s", renderedDest)
			needsDownload = false
		} else {
			// File exists but checksum doesn't match, re-download
			needsDownload = true
		}
	}

	mode := h.parseFileMode(downloadAction.Mode, defaultFileMode)

	if !needsDownload {
		// Already downloaded with correct checksum
		return result, nil
	}

	result.Changed = true

	// Create backup if requested and dest exists
	if downloadAction.Backup && destExists {
		ctx.GetLogger().Debugf("  Creating backup of: %s", renderedDest)
		backupPath, err := utils.CreateBackup(renderedDest)
		if err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
		ctx.GetLogger().Debugf("  Backup created: %s", backupPath)
	}

	// Download file with retries
	ctx.GetLogger().Debugf("  Downloading: %s -> %s", renderedURL, renderedDest)
	maxRetries := downloadAction.Retries
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	var downloadedSize int64
	var downloadErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			ctx.GetLogger().Debugf("  Retry attempt %d/%d", attempt, maxRetries)
		}

		downloadedSize, downloadErr = h.downloadFile(renderedURL, renderedDest, downloadAction, mode, step, ec, ctx)
		if downloadErr == nil {
			break // Success
		}

		if attempt < maxRetries {
			// Wait before retry (using step-level retry delay if available)
			if step.RetryDelay != "" {
				if delay, parseErr := time.ParseDuration(step.RetryDelay); parseErr == nil {
					time.Sleep(delay)
				} else {
					time.Sleep(1 * time.Second) // Default 1s delay
				}
			} else {
				time.Sleep(1 * time.Second) // Default 1s delay
			}
		}
	}

	if downloadErr != nil {
		result.Failed = true
		return result, downloadErr
	}

	// Verify checksum if provided
	if downloadAction.Checksum != "" {
		ctx.GetLogger().Debugf("  Verifying checksum: %s", downloadAction.Checksum)
		matches, err := utils.VerifyChecksum(renderedDest, downloadAction.Checksum)
		if err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify checksum: %w", err)
		}
		if !matches {
			result.Failed = true
			return result, fmt.Errorf("downloaded file checksum mismatch")
		}
	}

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventFileDownloaded,
			Data: events.FileDownloadedData{
				URL:       renderedURL,
				Dest:      renderedDest,
				SizeBytes: downloadedSize,
				Mode:      mode.String(),
				Checksum:  downloadAction.Checksum,
				DryRun:    ctx.IsDryRun(),
			},
		})
	}

	return result, nil
}

// DryRun logs what would be done without actually doing it.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	downloadAction := step.Download

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render URL
	renderedURL, err := ctx.GetTemplate().Render(downloadAction.URL, ctx.GetVariables())
	if err != nil {
		renderedURL = downloadAction.URL
	}

	// Render destination path
	renderedDest, err := ec.PathUtil.ExpandPath(downloadAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedDest = downloadAction.Dest
	}

	// Check if destination exists
	_, err = os.Stat(renderedDest)
	destExists := err == nil

	// Determine if download is needed
	needsDownload := !destExists || downloadAction.Force
	if destExists && !downloadAction.Force && downloadAction.Checksum != "" {
		matches, checksumErr := utils.VerifyChecksum(renderedDest, downloadAction.Checksum)
		if checksumErr == nil && matches {
			needsDownload = false
		}
	}

	mode := h.parseFileMode(downloadAction.Mode, defaultFileMode)

	if needsDownload {
		ctx.GetLogger().Infof("  [DRY-RUN] Would download: %s -> %s (mode: %s)",
			renderedURL, renderedDest, h.formatMode(mode))
	} else {
		ctx.GetLogger().Infof("  [DRY-RUN] File already downloaded with correct checksum: %s", renderedDest)
	}

	if downloadAction.Checksum != "" {
		ctx.GetLogger().Debugf("  Would verify checksum: %s", downloadAction.Checksum)
	}

	if len(downloadAction.Headers) > 0 {
		ctx.GetLogger().Debugf("  Would use %d custom headers", len(downloadAction.Headers))
	}

	if downloadAction.Timeout != "" {
		ctx.GetLogger().Debugf("  Would use timeout: %s", downloadAction.Timeout)
	}

	if downloadAction.Retries > 0 {
		ctx.GetLogger().Debugf("  Would retry up to %d times", downloadAction.Retries)
	}

	if downloadAction.Backup && destExists && needsDownload {
		ctx.GetLogger().Debugf("  Would create backup before overwrite")
	}

	return nil
}

// Helper functions

func (h *Handler) formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

func (h *Handler) parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

func (h *Handler) downloadFile(url, dest string, action *config.Download, mode os.FileMode, step *config.Step, ec *executor.ExecutionContext, ctx actions.Context) (int64, error) {
	// Create HTTP client with optional timeout
	client := &http.Client{}
	if action.Timeout != "" {
		timeout, err := time.ParseDuration(action.Timeout)
		if err != nil {
			return 0, fmt.Errorf("invalid timeout duration %q: %w", action.Timeout, err)
		}
		client.Timeout = timeout
	} else if step.Timeout != "" {
		// Fall back to step-level timeout if available
		timeout, err := time.ParseDuration(step.Timeout)
		if err == nil {
			client.Timeout = timeout
		}
	}

	// Create HTTP request
	// #nosec G107 -- URL comes from user-provided YAML configuration
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add custom headers if specified
	if action.Headers != nil {
		for key, value := range action.Headers {
			req.Header.Add(key, value)
		}
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to download: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Create temporary file for atomic write
	tmpFile, err := os.CreateTemp("", "mooncake-download-*")
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close temp file: %v", closeErr)
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			ctx.GetLogger().Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
		}
	}()

	// Copy download to temp file
	downloadedSize, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		return 0, fmt.Errorf("failed to write downloaded content: %w", err)
	}

	// Close temp file before moving
	if err := tmpFile.Close(); err != nil {
		return 0, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return 0, fmt.Errorf("failed to set permissions: %w", err)
	}

	// Move temp file to destination (atomic)
	if step.Become {
		if !security.IsBecomeSupported() {
			return 0, fmt.Errorf("become not supported on %s", runtime.GOOS)
		}
		if ec.SudoPass == "" {
			return 0, fmt.Errorf("step requires sudo but no password provided")
		}
		// Use sudo for final move
		cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
		if err := h.executeSudoCommand(cmd, step, ec); err != nil {
			return 0, fmt.Errorf("failed to move file with sudo: %w", err)
		}
	} else {
		if err := os.Rename(tmpPath, dest); err != nil {
			return 0, fmt.Errorf("failed to move file: %w", err)
		}
	}

	return downloadedSize, nil
}

func (h *Handler) executeSudoCommand(command string, _ *config.Step, ec *executor.ExecutionContext) error {
	// #nosec G204 - This is a provisioning tool designed to execute commands
	cmd := exec.Command("sudo", "-S", "sh", "-c", command)
	cmd.Stdin = bytes.NewBufferString(ec.SudoPass + "\n")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}

	return nil
}
