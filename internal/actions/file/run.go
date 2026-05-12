package file

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. It replaces the legacy Execute,
// DryRun, and Check methods. The handler inspects state once, then either
// performs side effects (ModeApply) or returns a prediction (ModePlan).
//
// All filesystem mutations route through ctx.Effects() so the same
// defaulting and predicate logic decides both the preview and the real
// run. This is the structural fix for the drift class of bugs that
// motivated Spec 16 (see docs-working/spec-16-unify-dryrun-execute.md).
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	file := step.File

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	renderedPath, err := ec.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables())
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

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))
	p := ctx.Effects()
	opts := actions.PerformerOpts{Become: step.Become}

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
		// AlreadyOk in plan mode is still a "no-change" outcome; surface it
		// as a non-changing successful result.
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

	emitFileEvent(ctx, state, primary, renderedPath, mode, h.formatMode)
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
	step *config.Step,
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
	expanded, err := ec.PathUtil.ExpandPath(rendered, ec.CurrentDir, ctx.GetVariables())
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
func emitFileEvent(ctx actions.Context, state string, eff actions.Effect, path string, mode os.FileMode, formatMode func(os.FileMode) string) {
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
		eventType := events.EventFileCreated
		// Heuristic: if the Effect's Reason indicates an existing file
		// (contains "differs" or "chmod" or "match"), it's an update.
		// Cleaner long-term would be for Effect to carry a "was-existing"
		// bit, but this matches today's observable behavior.
		if eff.Reason != "" && !contains(eff.Reason, "would create") {
			eventType = events.EventFileUpdated
		}
		pub.Publish(events.Event{
			Type: eventType,
			Data: events.FileOperationData{
				Path:    path,
				Mode:    formatMode(mode),
				Changed: eff.Performed,
				DryRun:  false,
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
