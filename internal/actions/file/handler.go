// Package file implements the file action handler.
//
// The file action manages files, directories, and links with support for:
// - Creating/updating files with content
// - Creating directories
// - Removing files and directories
// - Creating symbolic and hard links
// - Setting permissions and ownership
// - Touch operations (update timestamps)
package file

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// snapshotMaxBytes caps the in-memory head sample captured for event
// payloads. Slightly above artifact_capture's defaultMaxDiffSize (1 MB)
// so consumers with a custom MaxDiffSize between 1 MB and 8 MB still get
// the full content. Files larger than this are SHA-hashed in full but
// only the head sample lands in the event — consumers downstream of
// artifact_capture already truncate to MaxDiffSize, so the cap here
// trades unbounded handler RSS for a documented-truncation contract.
//
// F026: pre-fix the handler called os.ReadFile on the target path,
// allocating len(file) bytes per call (up to 3× per Run on a single
// file.write: pre-snapshot + backup + post-snapshot). A 200 MB target
// peaked at ~600 MB handler RSS; in a memory-constrained container
// (256 MB sidecar) the apply OOM-killed silently.
const snapshotMaxBytes = 8 * 1024 * 1024

// snapshotFile streams `path` once, returning up to snapshotMaxBytes of
// the head, the file's full size, and the SHA-256 of the full content
// (not just the head). Returns (nil, 0, "", err) if the file doesn't
// exist or is unreadable — callers treat that as "no before-state",
// which is the right semantic for create / missing-file paths.
//
// The hash covers the whole file (the bytes are read past the head cap
// for hashing) so downstream consumers comparing checksums still
// observe ground truth.
func snapshotFile(path string) (head []byte, size int64, sum string, err error) {
	f, err := os.Open(path) // #nosec G304 -- path is the action's own target
	if err != nil {
		return nil, 0, "", err
	}
	defer f.Close() //nolint:errcheck

	h := sha256.New()
	headBuf := make([]byte, 0, snapshotMaxBytes)
	buf := make([]byte, 64*1024)
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			size += int64(n)
			h.Write(buf[:n])
			if rem := snapshotMaxBytes - len(headBuf); rem > 0 {
				take := n
				if take > rem {
					take = rem
				}
				headBuf = append(headBuf, buf[:take]...)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, "", readErr
		}
	}
	return headBuf, size, hex.EncodeToString(h.Sum(nil)), nil
}

// copyFileStreaming reads from srcPath and writes to dstPath via
// io.Copy with a bounded buffer. Used by the backup path so a
// large target doesn't pin handler RSS to its full size. Returns
// the write error if any; src-open / dst-create errors get the
// usual wrapping.
func copyFileStreaming(srcPath, dstPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath) // #nosec G304 -- action's own target
	if err != nil {
		return err
	}
	defer src.Close()                                                          //nolint:errcheck
	dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) // #nosec G304 -- action's own backup path
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close() //nolint:errcheck
		return err
	}
	return dst.Close()
}

const (
	defaultFileMode os.FileMode = 0644
	defaultDirMode  os.FileMode = 0755
	actionTypeFile              = "file"
	stateLink                   = "link"
	stateHardlink               = "hardlink"
)

// defaultModeFor returns the mode applied when a file step omits an explicit
// `mode:`. Both Execute and DryRun must consult this so the preview matches
// what is actually performed. See spec-16-unify-dryrun-execute for the
// longer-term plan to remove the parallel paths entirely.
func defaultModeFor(state string) os.FileMode {
	if state == "directory" {
		return defaultDirMode
	}
	return defaultFileMode
}

// Handler implements the Handler interface for file actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
	executor.RegisterReverseDataType("FileReverseInfo", func() any { return &FileReverseInfo{} })
}

// Permissions implements actions.Permitter (spec-22). Declares the
// privileges and FS write target a file.write step needs, so the
// executor can preflight-check before running.
//
// Sudo: true when the destination path is under a system directory
// (/etc, /usr, /var, /opt, ...) per actions.SystemPathPrefixes.
// FilesystemWrite always carries the declared path (or empty if not
// specified) so a future policy layer can allowlist / denylist write
// targets.
//
// Network and RequiredBinaries: unset — file.write is a pure local-FS
// operation with no binary deps.
func (Handler) Permissions(step *config.Step) actions.PermissionSet {
	var ps actions.PermissionSet
	if step == nil || step.FileWrite == nil {
		return ps
	}
	if actions.PathNeedsSudo(step.FileWrite.Path) {
		ps.Sudo = true
	}
	if step.FileWrite.Path != "" {
		ps.FilesystemWrite = []string{step.FileWrite.Path}
	}
	return ps
}

// Metadata returns metadata about the file action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:           "file.write",
		Description:    "Manage files, directories, links, and permissions",
		Category:       actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: true,
		EmitsEvents: []string{
			string(events.EventFileCreated),
			string(events.EventFileUpdated),
			string(events.EventFileRemoved),
			string(events.EventDirCreated),
			string(events.EventDirRemoved),
			string(events.EventLinkCreated),
			string(events.EventPermissionsChanged),
		},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on path/ownership operation
		ImplementsCheck:    true,       // Checks existence, permissions, ownership before changes
	}
}

// Validate checks if the file configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.FileWrite == nil {
		return fmt.Errorf("file configuration is nil")
	}

	file := step.FileWrite
	if file.Path == "" {
		// Generate hint from schema
		hint := actions.GetActionHint("file", "path")

		// Add context-aware note if user used 'src' instead of 'path'
		note := ""
		if file.Src != "" {
			note = "\nNote: You provided 'src' but 'path' is required. The 'src' parameter is only used with state='link' or state='hardlink'.\n"
		}

		return fmt.Errorf("file path is empty%s%s", note, hint)
	}

	// Validate state
	validStates := map[string]bool{
		"file": true, "directory": true, "absent": true,
		"touch": true, stateLink: true, stateHardlink: true, "perms": true,
	}
	if file.State != "" && !validStates[file.State] {
		hint := actions.GetActionHint("file", "state")
		return fmt.Errorf("invalid state: %s%s", file.State, hint)
	}

	// Validate link operations require src
	if (file.State == stateLink || file.State == stateHardlink) && file.Src == "" {
		hint := actions.GetActionHint("file", "src")
		return fmt.Errorf("state %s requires src parameter%s", file.State, hint)
	}

	return nil
}

// Execute runs the file action.
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

// Run is the Spec 16 unified entry point. It replaces the legacy Execute,
// DryRun, and Check methods. The handler inspects state once, then either
// performs side effects (ModeApply) or returns a prediction (ModePlan).
//
// All filesystem mutations route through ctx.Effects() so the same
// defaulting and predicate logic decides both the preview and the real
// run. This is the structural fix for the drift class of bugs that
// motivated Spec 16 (see docs-working/specs/done/spec-16-unify-dryrun-execute.md).
// RunRaw signals spec-69 RawRunner participation so user-declared
// `retry:` actually retries this idempotent action via the
// centralized executor loop instead of being silently no-op'd.
func (h *Handler) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return h.Run(ctx, step)
}

func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	file := step.FileWrite

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	renderedPath, err := ec.Svc.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.Target = renderedPath
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	state := file.State
	if state == "" {
		state = actionTypeFile
	}

	// Proposal-01 envelope: Operation reflects what state= maps to.
	// State=absent → OpDelete; everything else → OpUpdate (covers both
	// fresh-create and update — Performer.Effect.Performed is the bool
	// that distinguishes them on disk, the envelope verb stays uniform).
	// Flipped to OpNoop below if Performed=false (file already at target).
	if state == "absent" {
		result.Operation = executor.OpDelete
	} else {
		result.Operation = executor.OpUpdate
	}

	// Capture pre-state for Reverse() (spec-22 phase 5a). Apply mode
	// only — plan mode doesn't mutate, so there is nothing to reverse.
	// Must run BEFORE runState so the recorded snapshot reflects the
	// world pre-mutation.
	if ctx.Mode() == actions.ModeApply {
		result.ReverseData = CaptureReverseInfo(renderedPath, state)
	}

	// Issue #27: capture pre-write bytes for downstream consumers
	// (artifact.capture's size/checksum/content fields). Only meaningful
	// for the file-write case in Apply mode. If the file doesn't exist,
	// snapshotFile returns the not-exist error and beforeHead stays nil
	// — `created` operations then report a 0/empty before-state, which
	// is the correct semantic.
	//
	// F026: stream-snapshot caps in-memory bytes at snapshotMaxBytes
	// while still reading the full file for the SHA-256. The previous
	// os.ReadFile path allocated len(file) bytes regardless of consumer
	// downstream — a 200 MB target ate 200 MB of RSS per Run before
	// the consumer even saw it.
	var (
		beforeHead []byte
		beforeSize int64
		beforeSum  string
	)
	if state == actionTypeFile && ctx.Mode() == actions.ModeApply {
		beforeHead, beforeSize, beforeSum, _ = snapshotFile(renderedPath)
	}

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))
	p := ctx.Effects()
	opts := actions.PerformerOpts{Force: file.Force}

	primary, err := h.runState(ctx, ec, file, step, state, renderedPath, mode, p, opts)
	if err != nil {
		result.Failed = true
		return result, err
	}
	if primary.Err != nil {
		result.Failed = true
		return result, primary.Err
	}

	// Plan mode: surface prediction; skip ownership/event side effects.
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = primary.WouldChange
		result.Reason = primary.Reason
		result.Detail = primary.Detail
		return result, nil
	}

	// Execute mode: real side effects ran.
	if primary.Performed {
		result.Changed = true
	} else if !result.Failed {
		// Idempotent path — file/dir already at target state.
		result.Operation = executor.OpNoop
	}

	// Ownership: run after primary so chown applies to the just-written target.
	if file.Owner != "" || file.Group != "" {
		ownEffect := p.Chown(renderedPath, file.Owner, file.Group, opts)
		if ownEffect.Err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to set ownership: %w", ownEffect.Err)
		}
		if ownEffect.Performed {
			result.Changed = true
		}
	}

	emitFileEvent(ctx, state, primary, renderedPath, mode, h.formatMode, beforeHead, beforeSize, beforeSum)
	return result, nil
}

// runState dispatches to the per-state Effect call, returning the
// primary Effect. Returns (Effect, error) where error covers pre-Effect
// validation failures (e.g. template render errors); Effect.Err covers
// failures from the Performer itself.
func (h *Handler) runState(
	ctx actions.Context,
	ec *executor.ExecutionContext,
	file *config.File,
	_ *config.Step,
	state, renderedPath string,
	mode os.FileMode,
	p actions.Performer,
	opts actions.PerformerOpts,
) (actions.Effect, error) {
	switch state {
	case "directory":
		return p.Mkdir(renderedPath, mode, opts), nil

	case actionTypeFile:
		rendered, err := ctx.GetTemplate().Render(file.Content, ctx.GetVariables())
		if err != nil {
			return actions.Effect{}, fmt.Errorf("failed to render content: %w", err)
		}
		content := []byte(rendered)

		// Backup before overwriting — only in execute mode and only if
		// the file currently exists.
		//
		// F026: stream the existing file to the backup path instead of
		// buffering its contents in RAM. The previous os.ReadFile +
		// os.WriteFile pair allocated len(file) bytes; copyFileStreaming
		// is bounded by a 64 KB internal buffer.
		if ctx.Mode() == actions.ModeApply && file.Backup {
			if _, statErr := os.Stat(renderedPath); statErr == nil {
				backupPath := renderedPath + ".bak"
				if writeErr := copyFileStreaming(renderedPath, backupPath, 0o600); writeErr != nil {
					ctx.GetLogger().Debugf("  Warning: failed to create backup: %v", writeErr)
				} else {
					ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
				}
			}
		}
		return p.WriteFile(renderedPath, content, mode, opts), nil

	case "absent":
		return p.Remove(renderedPath, true, opts), nil

	case "touch":
		return p.Touch(renderedPath, mode, opts), nil

	case stateLink:
		expandedSrc, err := h.resolveLinkSrc(ctx, ec, file)
		if err != nil {
			return actions.Effect{}, err
		}
		if err := h.checkLinkForce(renderedPath, expandedSrc, file.Force); err != nil {
			return actions.Effect{}, err
		}
		return p.Symlink(expandedSrc, renderedPath, opts), nil

	case stateHardlink:
		expandedSrc, err := h.resolveLinkSrc(ctx, ec, file)
		if err != nil {
			return actions.Effect{}, err
		}
		if err := h.checkHardlinkForce(renderedPath, expandedSrc, file.Force); err != nil {
			return actions.Effect{}, err
		}
		return p.Hardlink(expandedSrc, renderedPath, opts), nil

	case "perms":
		return p.Chmod(renderedPath, mode, opts), nil

	default:
		return actions.Effect{}, fmt.Errorf("unknown file state: %s", state)
	}
}

// resolveLinkSrc renders and path-expands the link source. Returns the
// fully-resolved path or an error if rendering/expansion fails.
func (h *Handler) resolveLinkSrc(ctx actions.Context, ec *executor.ExecutionContext, file *config.File) (string, error) {
	rendered, err := ctx.GetTemplate().Render(file.Src, ctx.GetVariables())
	if err != nil {
		return "", fmt.Errorf("failed to render src: %w", err)
	}
	expanded, err := ec.Svc.PathUtil.ExpandPath(rendered, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return "", fmt.Errorf("failed to expand src path: %w", err)
	}
	return expanded, nil
}

// checkLinkForce errors out when a symlink exists with a different
// target and force is not set. This matches the legacy contract.
func (h *Handler) checkLinkForce(path, desiredTarget string, force bool) error {
	if linkTarget, err := os.Readlink(path); err == nil {
		if linkTarget != desiredTarget && !force {
			return errors.New("link exists with different target (use force: true to overwrite)")
		}
	}
	return nil
}

// checkHardlinkForce errors out when a regular file or wrong-inode
// hardlink exists and force is not set.
func (h *Handler) checkHardlinkForce(path, desiredTarget string, force bool) error {
	dstInfo, err := os.Stat(path)
	if err != nil {
		return nil // doesn't exist; safe to create
	}
	srcInfo, srcErr := os.Stat(desiredTarget)
	if srcErr == nil && os.SameFile(srcInfo, dstInfo) {
		return nil // already correct
	}
	if !force {
		return errors.New("file exists (use force: true to overwrite)")
	}
	return nil
}

// emitFileEvent publishes the appropriate event for the state and
// effect outcome. Only called in ModeApply. Matches the events the
// legacy Execute path emitted.
//
// beforeHead carries up to snapshotMaxBytes of the pre-write file's
// head bytes (full content for small files, head sample for big ones).
// beforeSize / beforeSum are the FULL file size and SHA-256, computed
// streaming so a 10 GB target doesn't allocate 10 GB. The event's
// ContentBefore/After fields carry the head sample (artifact_capture
// consumers truncate to MaxDiffSize anyway); SizeBefore/After carry
// ground truth. F026.
func emitFileEvent(ctx actions.Context, state string, eff actions.Effect, path string, mode os.FileMode, formatMode func(os.FileMode) string, beforeHead []byte, beforeSize int64, beforeSum string) {
	pub := ctx.GetEventPublisher()
	if pub == nil {
		return
	}

	switch state {
	case "directory":
		pub.Publish(events.Event{
			Type: events.EventDirCreated,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case actionTypeFile:
		// F026: stream-snapshot the just-written file so we get a
		// bounded head sample + full size + full SHA-256 without
		// allocating len(file) bytes. Same pattern as the pre-write
		// snapshot above the call to runState.
		afterHead, afterSize, afterSum, _ := snapshotFile(path)
		// fileExistedBefore: pre-fix we used beforeBytes != nil, which
		// is equivalent to beforeSize > 0 OR the file existed but was
		// empty. snapshotFile returns size==0 and a non-error for an
		// empty pre-existing file, so check via beforeSum instead —
		// non-empty hash means we successfully read at least the
		// header, which only happens when the file existed.
		fileExistedBefore := beforeSum != ""
		eventType := events.EventFileCreated
		if fileExistedBefore {
			eventType = events.EventFileUpdated
		} else if eff.Reason != "" && !contains(eff.Reason, "would create") {
			// Belt-and-suspenders for performers that report an update via
			// Effect.Reason without our Run() having captured a
			// pre-write snapshot (e.g. a future planner path bypassing
			// the snapshotFile probe).
			eventType = events.EventFileUpdated
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:           path,
				Mode:           formatMode(mode),
				SizeBytes:      afterSize,
				SizeBefore:     beforeSize,
				Changed:        eff.Performed,
				DryRun:         false,
				ChecksumBefore: beforeSum,
				ChecksumAfter:  afterSum,
				ContentBefore:  beforeHead,
				ContentAfter:   afterHead,
			},
		})
	case "absent":
		// Distinguish file vs dir for the event type.
		eventType := events.EventFileRemoved
		if eff.Reason != "" && contains(eff.Reason, "directory") {
			eventType = events.EventDirRemoved
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:    path,
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case "touch":
		pub.Publish(events.Event{
			Type: events.EventFileCreated,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	case stateLink, stateHardlink:
		linkType := "symlink"
		if state == stateHardlink {
			linkType = stateHardlink
		}
		pub.Publish(events.Event{
			Type: events.EventLinkCreated,
			Data: events.LinkCreatedData{
				Src:    "", // path expansion happens earlier; Effect doesn't carry src today
				Dest:   path,
				Type:   linkType,
				DryRun: false,
			},
		})
	case "perms":
		pub.Publish(events.Event{
			Type: events.EventPermissionsChanged,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
			},
		})
	}
}

// contains is a tiny strings.Contains replacement to avoid a dependency
// import for one call.
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure path is treated; keep filepath import live for future use
// (parent-dir resolution may move into runState).
var _ = filepath.Dir
