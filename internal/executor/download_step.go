package executor

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/utils"
)

// HandleDownload downloads a file from a URL with optional checksum verification.
func HandleDownload(step config.Step, ec *ExecutionContext) error {
	downloadAction := step.Download

	// Validate required fields
	if downloadAction.URL == "" {
		return &StepValidationError{Field: "url", Message: "required for download"}
	}
	if downloadAction.Dest == "" {
		return &StepValidationError{Field: "dest", Message: "required for download"}
	}

	// Render URL with templates
	renderedURL, err := ec.Template.Render(downloadAction.URL, ec.Variables)
	if err != nil {
		return &RenderError{Field: "url", Cause: err}
	}

	// Render destination path
	renderedDest, err := ec.PathUtil.ExpandPath(downloadAction.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "dest path", Cause: err}
	}

	// Create result object
	result := NewResult()
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
			ec.Logger.Debugf("  Unable to verify checksum of existing file: %v", checksumErr)
			needsDownload = true
		} else if matches {
			// File exists with correct checksum, skip download
			needsDownload = false
		} else {
			// File exists but checksum doesn't match, re-download
			needsDownload = true
		}
	}

	mode := parseFileMode(downloadAction.Mode, 0644)

	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		if needsDownload {
			dryRun.LogFileDownload(renderedURL, renderedDest, mode)
		} else {
			dryRun.LogFileDownloadNoChange(renderedURL, renderedDest)
		}
		dryRun.LogRegister(step)
	}) {
		return nil
	}

	if !needsDownload {
		// Already downloaded with correct checksum
		return nil
	}

	result.Changed = true

	// Create backup if requested and dest exists
	if downloadAction.Backup && destExists {
		ec.Logger.Debugf("  Creating backup of: %s", renderedDest)
		backupPath, err := utils.CreateBackup(renderedDest)
		if err != nil {
			markStepFailed(result, step, ec)
			return &FileOperationError{Operation: "backup", Path: renderedDest, Cause: err}
		}
		ec.Logger.Debugf("  Backup created: %s", backupPath)
	}

	// Download file with retries
	ec.Logger.Debugf("  Downloading: %s -> %s", renderedURL, renderedDest)
	maxRetries := downloadAction.Retries
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	var downloadedSize int64
	var downloadErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			ec.Logger.Debugf("  Retry attempt %d/%d", attempt, maxRetries)
		}

		downloadedSize, downloadErr = downloadFile(renderedURL, renderedDest, downloadAction, mode, step, ec)
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
		markStepFailed(result, step, ec)
		return downloadErr
	}

	// Verify checksum if provided
	if downloadAction.Checksum != "" {
		ec.Logger.Debugf("  Verifying checksum: %s", downloadAction.Checksum)
		matches, err := utils.VerifyChecksum(renderedDest, downloadAction.Checksum)
		if err != nil {
			markStepFailed(result, step, ec)
			return &FileOperationError{Operation: "verify", Path: renderedDest, Cause: err}
		}
		if !matches {
			markStepFailed(result, step, ec)
			return &StepValidationError{Field: "checksum", Message: "downloaded file checksum mismatch"}
		}
	}

	// Emit file.downloaded event
	ec.EmitEvent(events.EventFileDownloaded, events.FileDownloadedData{
		URL:       renderedURL,
		Dest:      renderedDest,
		SizeBytes: downloadedSize,
		Mode:      mode.String(),
		Checksum:  downloadAction.Checksum,
		DryRun:    ec.DryRun,
	})

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	return nil
}

// downloadFile downloads a file from URL to destination with atomic write pattern.
func downloadFile(url, dest string, action *config.Download, mode os.FileMode, step config.Step, ec *ExecutionContext) (int64, error) {
	// Create HTTP client with optional timeout
	client := &http.Client{}
	if action.Timeout != "" {
		timeout, err := time.ParseDuration(action.Timeout)
		if err != nil {
			return 0, &SetupError{
				Component: "timeout",
				Issue:     fmt.Sprintf("invalid duration %q", action.Timeout),
				Cause:     err,
			}
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
		return 0, &FileOperationError{Operation: "download", Path: url, Cause: err}
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
		return 0, &FileOperationError{Operation: "download", Path: url, Cause: err}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			ec.Logger.Debugf("Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, &FileOperationError{
			Operation: "download",
			Path:      url,
			Cause:     fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}
	}

	// Create temporary file for atomic write
	tmpFile, err := os.CreateTemp("", "mooncake-download-*")
	if err != nil {
		return 0, &FileOperationError{Operation: "create", Path: "temp file in " + os.TempDir(), Cause: err}
	}
	tmpPath := tmpFile.Name()
	defer func() {
		if closeErr := tmpFile.Close(); closeErr != nil {
			ec.Logger.Debugf("Failed to close temp file: %v", closeErr)
		}
		if removeErr := os.Remove(tmpPath); removeErr != nil {
			ec.Logger.Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
		}
	}()

	// Copy download to temp file
	downloadedSize, err := io.Copy(tmpFile, resp.Body)
	if err != nil {
		return 0, &FileOperationError{Operation: "write", Path: tmpPath, Cause: err}
	}

	// Close temp file before moving
	if err := tmpFile.Close(); err != nil {
		return 0, &FileOperationError{Operation: "write", Path: tmpPath, Cause: err}
	}

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return 0, &FileOperationError{Operation: "chmod", Path: tmpPath, Cause: err}
	}

	// Move temp file to destination (atomic)
	if step.Become {
		// Use sudo for final move
		cmd := fmt.Sprintf("mv %q %q", tmpPath, dest)
		if err := executeSudoCommand(cmd, step, ec); err != nil {
			return 0, &FileOperationError{Operation: "write", Path: dest, Cause: err}
		}
	} else {
		if err := os.Rename(tmpPath, dest); err != nil {
			return 0, &FileOperationError{Operation: "write", Path: dest, Cause: err}
		}
	}

	return downloadedSize, nil
}
