package file_delete_range

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

// Run is the Spec 16 unified entry point. Computes the post-deletion
// content in memory; plan mode reports the prediction, execute mode
// commits the change atomically.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fdr := step.TextDeleteRange

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

	renderedPath, err := ec.PathUtil.ExpandPath(fdr.Path, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}
	if pathErr := pathutil.ValidateNoPathTraversal(renderedPath); pathErr != nil {
		ctx.GetLogger().Debugf("  Path validation warning: %v", pathErr)
	}

	// #nosec G304 -- file path from user config
	originalContent, err := os.ReadFile(renderedPath)
	if err != nil {
		return result, fmt.Errorf("failed to read file %s: %w", renderedPath, err)
	}

	renderedStartAnchor, err := ctx.GetTemplate().Render(fdr.StartAnchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render start_anchor: %w", err)
	}
	renderedEndAnchor, err := ctx.GetTemplate().Render(fdr.EndAnchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render end_anchor: %w", err)
	}

	newContent, deletedLines, err := h.performDeletion(
		string(originalContent),
		renderedStartAnchor,
		renderedEndAnchor,
		fdr.Regex,
		fdr.Inclusive,
	)
	if err != nil {
		return result, err
	}

	if string(originalContent) == newContent {
		result.Reason = "range not found or already deleted"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would delete %d line(s)", deletedLines)
		return result, nil
	}

	if fdr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Deleted %d line(s) from %s", deletedLines, renderedPath)

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
		"deleted_lines": deletedLines,
		"start_anchor":  renderedStartAnchor,
		"end_anchor":    renderedEndAnchor,
	})
	return result, nil
}
