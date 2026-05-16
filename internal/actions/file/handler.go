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
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// sha256Hex returns the hex-encoded SHA256 of b, or "" for nil. Used by
// the file.write event-emit path so downstream consumers (artifact.capture
// with include_checksums:true) get checksums without re-reading the file.
func sha256Hex(b []byte) string {
	if b == nil {
		return ""
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
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
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	state := file.State
	if state == "" {
		state = actionTypeFile
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
	// readErr is non-nil and beforeBytes stays nil — `created` operations
	// then report a 0/empty before-state, which is the correct semantic.
	var beforeBytes []byte
	if state == actionTypeFile && ctx.Mode() == actions.ModeApply {
		beforeBytes, _ = os.ReadFile(renderedPath) // #nosec G304 -- path is the same target runState writes to
	}

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))
	p := ctx.Effects()
	opts := actions.PerformerOpts{Become: step.ShouldBecome(), Force: file.Force}

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

	emitFileEvent(ctx, state, primary, renderedPath, mode, h.formatMode, beforeBytes)
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
		if ctx.Mode() == actions.ModeApply && file.Backup {
			if existing, readErr := os.ReadFile(renderedPath); readErr == nil {
				backupPath := renderedPath + ".bak"
				if writeErr := os.WriteFile(backupPath, existing, 0o600); writeErr != nil {
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
// beforeBytes carries the pre-write file content captured by Run()
// before runState ran. It powers issue-27's downstream artifact.capture
// size/checksum/content fields. Empty for non-file states or when the
// target didn't exist pre-write.
func emitFileEvent(ctx actions.Context, state string, eff actions.Effect, path string, mode os.FileMode, formatMode func(os.FileMode) string, beforeBytes []byte) {
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
		// Read the just-written file so the after-fields reflect ground
		// truth (rather than re-rendering content the caller no longer
		// has). Skipped silently on read error — the event still carries
		// the path/mode/changed bits, and downstream consumers degrade
		// rather than crash.
		afterBytes, _ := os.ReadFile(path) // #nosec G304 -- path is the same target runState just wrote
		fileExistedBefore := beforeBytes != nil
		eventType := events.EventFileCreated
		if fileExistedBefore {
			eventType = events.EventFileUpdated
		} else if eff.Reason != "" && !contains(eff.Reason, "would create") {
			// Belt-and-suspenders for performers that report an update via
			// Effect.Reason without our Run() having captured beforeBytes
			// (e.g. a future planner path bypassing the os.ReadFile probe).
			eventType = events.EventFileUpdated
		}
		var sizeBefore int64
		var checksumBefore string
		if fileExistedBefore {
			sizeBefore = int64(len(beforeBytes))
			checksumBefore = sha256Hex(beforeBytes)
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:           path,
				Mode:           formatMode(mode),
				SizeBytes:      int64(len(afterBytes)),
				SizeBefore:     sizeBefore,
				Changed:        eff.Performed,
				DryRun:         false,
				ChecksumBefore: checksumBefore,
				ChecksumAfter:  sha256Hex(afterBytes),
				ContentBefore:  beforeBytes,
				ContentAfter:   afterBytes,
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
