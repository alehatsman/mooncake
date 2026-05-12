package file_insert

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

// Run is the Spec 16 unified entry point. Computes the would-be content
// in memory once; plan mode reports the prediction, execute mode
// commits the change.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	fi := step.FileInsert

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

	renderedPath, err := ec.PathUtil.ExpandPath(fi.Path, ec.CurrentDir, ctx.GetVariables())
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

	renderedAnchor, err := ctx.GetTemplate().Render(fi.Anchor, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render anchor: %w", err)
	}
	renderedContent, err := ctx.GetTemplate().Render(fi.Content, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render content: %w", err)
	}

	newContent, insertionCount, err := h.performInsertion(
		string(originalContent),
		renderedAnchor,
		renderedContent,
		fi.Position,
		fi.Regex,
		fi.AllowMultiple,
	)
	if err != nil {
		return result, err
	}

	if string(originalContent) == newContent {
		result.Reason = "anchor not found or content already present"
		return result, nil
	}

	if ctx.Mode() == actions.ModePlan {
		result.WouldChange = true
		result.Reason = fmt.Sprintf("would insert content at %d location(s)", insertionCount)
		return result, nil
	}

	if fi.Backup {
		backupPath := renderedPath + ".bak"
		if err := os.WriteFile(backupPath, originalContent, 0o600); err != nil {
			return result, fmt.Errorf("failed to create backup: %w", err)
		}
	}

	if err := h.writeAtomic(renderedPath, newContent); err != nil {
		return result, fmt.Errorf("failed to write file: %w", err)
	}

	result.Changed = true
	ctx.GetLogger().Infof("  Inserted content at %d location(s) in %s", insertionCount, renderedPath)

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
		"path":       renderedPath,
		"insertions": insertionCount,
		"anchor":     renderedAnchor,
		"position":   fi.Position,
	})
	return result, nil
}
