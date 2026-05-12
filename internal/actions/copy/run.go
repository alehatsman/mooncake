package copy

import (
	"fmt"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Run is the Spec 16 unified entry point. Reads the source file once,
// optionally verifies its checksum, then routes the write through
// Performer.WriteFile so plan-mode and execute-mode share the same
// idempotency decision (byte-for-byte content + mode match).
//
// Compared to the legacy Execute path:
//   - Idempotency uses content equality rather than size+mtime. This
//     is slower for large files but always correct (the legacy heuristic
//     could miss content-identical files with mismatched mtimes and
//     re-copy needlessly).
//   - Source mtime is no longer preserved on the destination. Nothing
//     else in the codebase depends on this; the freshness check that
//     used it is replaced by Performer's content comparison.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	cp := step.FileCopy

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	src, err := ec.PathUtil.ExpandPath(cp.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}
	dest, err := ec.PathUtil.ExpandPath(cp.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	srcInfo, err := os.Stat(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to stat source: %w", err)
	}
	if srcInfo.IsDir() {
		result.Failed = true
		return result, fmt.Errorf("src is a directory, use recursive copy action instead")
	}

	// Source checksum verification (pre-copy). Failing here is a hard
	// error in both modes — it surfaces config drift early.
	if cp.Checksum != "" {
		ok, cerr := utils.VerifyChecksum(src, cp.Checksum)
		if cerr != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify source checksum: %w", cerr)
		}
		if !ok {
			result.Failed = true
			return result, fmt.Errorf("source checksum mismatch")
		}
	}

	// #nosec G304 — src path is user-supplied by design
	content, err := os.ReadFile(src)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to read source: %w", err)
	}

	// Default to the source file's mode if the user didn't specify one.
	mode := h.parseFileMode(cp.Mode, srcInfo.Mode()&os.ModePerm)

	// Backup before overwrite — only in execute mode.
	if ctx.Mode() == actions.ModeApply && cp.Backup {
		if _, statErr := os.Stat(dest); statErr == nil {
			if backupPath, berr := utils.CreateBackup(dest); berr != nil {
				ctx.GetLogger().Debugf("  Warning: failed to create backup: %v", berr)
			} else {
				ctx.GetLogger().Debugf("  Created backup: %s", backupPath)
			}
		}
	}

	// Force overrides idempotency — always counts as a change.
	if cp.Force {
		// Touch the dest to force WriteFile to overwrite even if
		// content matches. Cheapest: remove and let WriteFile create.
		if ctx.Mode() == actions.ModeApply {
			_ = os.Remove(dest)
		}
	}

	eff := ctx.Effects().WriteFile(dest, content, mode, actions.PerformerOpts{Become: step.ShouldBecome()})
	if eff.Err != nil {
		result.Failed = true
		return result, eff.Err
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = eff.WouldChange
		result.Reason = eff.Reason
		return result, nil
	}

	result.Changed = eff.Performed

	// Ownership after content is in place.
	if cp.Owner != "" || cp.Group != "" {
		own := ctx.Effects().Chown(dest, cp.Owner, cp.Group, actions.PerformerOpts{Become: step.ShouldBecome()})
		if own.Err != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to set ownership: %w", own.Err)
		}
		if own.Performed {
			result.Changed = true
		}
	}

	// Post-copy checksum verification.
	if cp.Checksum != "" {
		ok, verr := utils.VerifyChecksum(dest, cp.Checksum)
		if verr != nil {
			result.Failed = true
			return result, fmt.Errorf("failed to verify destination checksum: %w", verr)
		}
		if !ok {
			result.Failed = true
			return result, fmt.Errorf("destination checksum mismatch after copy")
		}
	}

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileCopied,
			Data: events.FileCopiedData{
				Src:       src,
				Dest:      dest,
				SizeBytes: srcInfo.Size(),
				Mode:      mode.String(),
				Checksum:  cp.Checksum,
				DryRun:    false,
			},
		})
	}

	return result, nil
}
