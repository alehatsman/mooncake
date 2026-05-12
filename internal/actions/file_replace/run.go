package file_replace

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

// Run is the Spec 16 unified entry point. Performs the regex replacement
// in memory once and either reports the predicted change (ModePlan) or
// commits it atomically (ModeExecute). Drift between plan preview and
// real execution is impossible because both modes compute the same
// `newContent` and compare against the same on-disk content.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fr := step.FileReplace

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

	renderedPath, err := ec.PathUtil.ExpandPath(fr.Path, ec.CurrentDir, ctx.GetVariables())
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

	renderedPattern, err := ctx.GetTemplate().Render(fr.Pattern, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render pattern: %w", err)
	}
	renderedReplace, err := ctx.GetTemplate().Render(fr.Replace, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render replacement: %w", err)
	}

	newContent, replacementCount, err := h.performReplace(string(originalContent), renderedPattern, renderedReplace, fr)
	if err != nil {
		return result, err
	}

	// No-change short-circuit shared by both modes.
	if string(originalContent) == newContent {
		if !fr.AllowNoMatch && replacementCount == 0 {
			return result, fmt.Errorf("no matches found for pattern: %s", renderedPattern)
		}
		result.Reason = "pattern not present or already replaced"
		return result, nil
	}

	// Change detected. Plan mode reports the prediction; execute mode
	// performs the atomic write and event emission.
	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would replace %d occurrence(s)", replacementCount)
		return result, nil
	}

	if fr.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Replaced %d occurrence(s) in %s", replacementCount, renderedPath)

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
		"path":         renderedPath,
		"replacements": replacementCount,
		"pattern":      renderedPattern,
	})
	return result, nil
}
