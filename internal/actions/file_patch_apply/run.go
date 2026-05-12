package file_patch_apply

import (
	"fmt"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

// Run is the Spec 16 unified entry point. Applies the patch in memory
// to predict the result; plan mode reports the prediction, execute
// mode commits the atomic write.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fpa := step.TextPatch

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true
	result.StartTime = time.Now()
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	renderedPath, err := ec.PathUtil.ExpandPath(fpa.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}
	if pathErr := pathutil.ValidateNoPathTraversal(renderedPath); pathErr != nil {
		ctx.GetLogger().Debugf("  Path validation warning: %v", pathErr)
	}

	//nolint:dupl // patch-load idiom shared with handler.go; trivial helper not worth the indirection.
	patchContent := ""
	if fpa.Patch != "" {
		rendered, perr := ctx.GetTemplate().Render(fpa.Patch, ctx.GetVariables())
		if perr != nil {
			return result, fmt.Errorf("failed to render patch: %w", perr)
		}
		patchContent = rendered
	} else {
		renderedPatchFile, pferr := ec.PathUtil.ExpandPath(fpa.PatchFile, ec.CurrentDir, ctx.GetVariables())
		if pferr != nil {
			return result, fmt.Errorf("failed to expand patch_file path: %w", pferr)
		}
		// #nosec G304 -- patch file from user config
		patchBytes, rerr := os.ReadFile(renderedPatchFile)
		if rerr != nil {
			return result, fmt.Errorf("failed to read patch file %s: %w", renderedPatchFile, rerr)
		}
		patchContent = string(patchBytes)
	}

	// #nosec G304 -- target file from user config
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	patch, err := h.parsePatch(patchContent)
	if err != nil {
		return result, fmt.Errorf("failed to parse patch: %w", err)
	}

	contextLines := 3
	if fpa.ContextLines != nil {
		contextLines = *fpa.ContextLines
	}

	newContent, appliedHunks, failedHunks := h.applyPatch(string(originalContent), patch, contextLines)

	if fpa.Strict && failedHunks > 0 {
		return result, fmt.Errorf("patch application failed: %d hunk(s) failed in strict mode", failedHunks)
	}

	if string(originalContent) == newContent {
		result.Reason = "patch already applied"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would apply %d hunk(s) (%d failed)", appliedHunks, failedHunks)
		return result, nil
	}

	if fpa.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Applied patch to %s (%d hunks succeeded, %d failed)", renderedPath, appliedHunks, failedHunks)

	if pub := ctx.GetEventPublisher(); pub != nil {
		pub.Publish(events.Event{
			Type: events.EventFileUpdated,
			Data: events.FileOperationData{
				Path:    renderedPath,
				Changed: true,
				DryRun:  false,
			},
		})
	}

	result.SetData(map[string]interface{}{
		"path":          renderedPath,
		"applied_hunks": appliedHunks,
		"failed_hunks":  failedHunks,
		"total_hunks":   len(patch.Hunks),
	})
	return result, nil
}
