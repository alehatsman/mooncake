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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/effects"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/httputil"
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
		Name:               "file.download",
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

// Permissions implements actions.Permitter (spec-22). file.download
// always declares Network=true — it fetches a URL — and Sudo when Dest
// lands under a known system root. FilesystemWrite=[Dest]. No required
// binaries (the HTTP client is built-in, not an external curl/wget).
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	ps := actions.PermissionSet{Network: true}
	if step == nil || step.FileDownload == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.FileDownload.Dest) {
		ps.Sudo = true
	}
	if step.FileDownload.Dest != "" {
		ps.FilesystemWrite = []string{step.FileDownload.Dest}
	}
	return ps
}

// Validate checks if the download configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileDownload == nil {
		return fmt.Errorf("download configuration is nil")
	}

	downloadAction := step.FileDownload
	if downloadAction.URL == "" {
		hint := actions.GetActionHint("download", "url")
		return fmt.Errorf("url is required%s", hint)
	}

	if downloadAction.Dest == "" {
		hint := actions.GetActionHint("download", "dest")
		return fmt.Errorf("dest is required%s", hint)
	}

	return nil
}

// runApply runs the download action.
func (h *Handler) runApply(ctx actions.Context, step *config.Step) (actions.Result, error) {
	downloadAction := step.FileDownload

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
	renderedDest, err := ec.Svc.PathUtil.ExpandPath(downloadAction.Dest, ec.CurrentDir, ctx.GetVariables())
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

	// Capture pre-state for Reverse() (spec-22 phase 5 slice F).
	// Must run BEFORE the download below — once the file has been
	// fetched, the prior dest content (if any) is gone.
	result.ReverseData = filehandler.CaptureReverseInfo(renderedDest, "")

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

	// Single attempt — retry is owned by the executor's runWithRetry
	// when the user sets a step-level `retry:` block. Every error is
	// retryable by default (no Retryable impl); the action-local
	// `retries:` / `retry_delay:` fields were removed.
	ctx.GetLogger().Debugf("  Downloading: %s -> %s", renderedURL, renderedDest)
	downloadedSize, downloadErr := h.downloadFile(renderedURL, renderedDest, downloadAction, mode, step, ctx)
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
				DryRun:    ctx.Mode() == actions.ModePlan,
			},
		})
	}

	return result, nil
}

// Helper functions

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

func (h *Handler) downloadFile(url, dest string, action *config.Download, mode os.FileMode, step *config.Step, ctx actions.Context) (int64, error) {
	// F012: base transport inherits httputil's bounded dial / TLS /
	// response-headers timeouts. The client.Timeout wraps the whole
	// transfer (kept zero-by-default so large downloads aren't yanked
	// mid-stream); transport-level limits still bound the setup.
	client := &http.Client{Transport: httputil.DefaultTransport}
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

	// F012: ctx-aware request via httputil so canonical UA flows and
	// future caller ctx (step-level deadline, daemon Shutdown via
	// F016) reaches the socket. download doesn't currently thread a
	// real ctx into downloadFile; Background is bounded by the
	// transport-level timeouts above.
	// #nosec G107 -- URL comes from user-provided YAML configuration
	req, err := httputil.NewRequest(context.Background(), http.MethodGet, url, nil)
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
	// Track lifecycle of the temp file so the deferred cleanup only
	// touches state that's still valid:
	//   - tmpFileClosed: true once we've explicitly closed the handle
	//     (the defer must not re-Close → "file already closed" noise).
	//   - tmpFileMoved:  true once the file has been renamed/moved to
	//     dest (the defer must not Remove → "no such file" noise).
	// Both flags get flipped on success paths below.
	tmpFileClosed := false
	tmpFileMoved := false
	defer func() {
		if !tmpFileClosed {
			if closeErr := tmpFile.Close(); closeErr != nil {
				ctx.GetLogger().Debugf("Failed to close temp file: %v", closeErr)
			}
		}
		if !tmpFileMoved {
			if removeErr := os.Remove(tmpPath); removeErr != nil && !os.IsNotExist(removeErr) {
				ctx.GetLogger().Debugf("Failed to remove temp file %s: %v", tmpPath, removeErr)
			}
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
	tmpFileClosed = true

	// Set permissions on temp file
	if err := os.Chmod(tmpPath, mode); err != nil {
		return 0, fmt.Errorf("failed to set permissions: %w", err)
	}

	// MT-14 fix: verify the checksum on the temp file BEFORE we move
	// it to dest. The earlier shape ran VerifyChecksum on dest after
	// the rename, which means a mismatched download still left bytes
	// at dest (and tripped a confusing pair of debug-level cleanup
	// errors when the action did return failure later). Verifying
	// here keeps the atomic-write guarantee: dest is created only if
	// the bytes match.
	if action.Checksum != "" {
		matches, verr := utils.VerifyChecksum(tmpPath, action.Checksum)
		if verr != nil {
			return 0, fmt.Errorf("failed to verify checksum: %w", verr)
		}
		if !matches {
			actual := actualChecksum(tmpPath, action.Checksum)
			return 0, fmt.Errorf(
				"checksum mismatch: declared %s, actual %s (url: %s)",
				action.Checksum, actual, url)
		}
	}

	// Move temp file to destination (atomic)
	if step.ShouldBecome() {
		if !security.IsBecomeSupported() {
			return 0, fmt.Errorf("become not supported on %s", runtime.GOOS)
		}
		// F032: `%q` is Go-string quoting, NOT POSIX-shell quoting —
		// inside double-quotes bash still performs $(…) and backtick
		// substitution. A dest like `/tmp/$(touch /etc/owned)/foo`
		// became a code-execution vector under sudo. Use POSIX-safe
		// single-quote wrapping via effects.ShellQuote.
		cmd := fmt.Sprintf("mv %s %s", effects.ShellQuote(tmpPath), effects.ShellQuote(dest))
		if err := h.executeSudoCommand(ctx, cmd); err != nil {
			return 0, fmt.Errorf("failed to move file with sudo: %w", err)
		}
	} else {
		if err := os.Rename(tmpPath, dest); err != nil {
			return 0, fmt.Errorf("failed to move file: %w", err)
		}
	}
	tmpFileMoved = true

	return downloadedSize, nil
}

// actualChecksum recomputes the file's checksum in the same format
// as the declared one, for inclusion in the mismatch error message.
// Best-effort: if recompute fails we return "<unreadable>" rather
// than overshadowing the original mismatch with a read error.
func actualChecksum(path, declared string) string {
	switch len(declared) {
	case 64: // SHA256
		if s, err := utils.CalculateSHA256(path); err == nil {
			return s
		}
	case 32: // MD5
		if s, err := utils.CalculateMD5(path); err == nil {
			return s
		}
	}
	return "<unreadable>"
}

func (h *Handler) executeSudoCommand(ctx actions.Context, command string) error {
	// Spec-69 phase 5: route through ctx.Privileged() — the same
	// centralized sudo path every other action uses. F005's
	// IsBecomeSupported + SudoPass != "" validation lives inside
	// PrivilegedRunner so this site inherits it for free. Combined
	// output is folded into the error message so the operator sees
	// the failing sh -c invocation verbatim.
	out, err := ctx.Privileged().Run(context.TODO(), "sh", "-c", command)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" {
			return fmt.Errorf("sudo command failed: %w (output: %s)", err, msg)
		}
		return fmt.Errorf("sudo command failed: %w", err)
	}
	return nil
}

// Run is the Spec 16 unified entry point. Plan mode inspects the
// destination file: if it exists with a matching checksum (or
// force=false and no checksum specified), reports already-ok;
// otherwise reports would-download. Apply mode delegates to runApply
// which performs the HTTP fetch as a single attempt — retry, if any,
// is owned by the executor's runWithRetry via the RawRunner hook.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.runApply(ctx, step)
	}

	d := step.FileDownload
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	renderedDest, err := ec.Svc.PathUtil.ExpandPath(d.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand dest path: %w", err)
	}

	_, statErr := os.Stat(renderedDest)
	destExists := statErr == nil

	if !destExists {
		result.WouldChange = true
		result.Reason = "would download (destination missing)"
		return result, nil
	}

	if d.Force {
		result.WouldChange = true
		result.Reason = "would download (force=true)"
		return result, nil
	}

	if d.Checksum != "" {
		matches, cerr := utils.VerifyChecksum(renderedDest, d.Checksum)
		if cerr != nil {
			result.WouldChange = true
			result.Reason = fmt.Sprintf("would download (cannot verify existing checksum: %v)", cerr)
			return result, nil
		}
		if !matches {
			result.WouldChange = true
			result.Reason = "would download (existing file checksum mismatch)"
			return result, nil
		}
		result.Reason = "destination exists with correct checksum"
		return result, nil
	}

	// No checksum, no force, dest exists → legacy Execute also skips,
	// so plan reports already-ok.
	result.Reason = "destination already exists (no checksum / force to re-download)"
	return result, nil
}
