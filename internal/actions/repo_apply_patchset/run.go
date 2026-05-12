package repo_apply_patchset

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode parses the
// patchset and applies it in memory against each target file's
// current content. If any file's content would change, reports
// would-change with a count of affected files.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	raps := step.RepoPatch
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	baseDir := ec.CurrentDir
	if raps.BaseDir != "" {
		renderedBaseDir, err := ec.PathUtil.ExpandPath(raps.BaseDir, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("failed to expand base_dir: %w", err)
		}
		baseDir = renderedBaseDir
	}

	patchsetContent := ""
	if raps.Patchset != "" {
		rendered, perr := ctx.GetTemplate().Render(raps.Patchset, ctx.GetVariables())
		if perr != nil {
			return result, fmt.Errorf("failed to render patchset: %w", perr)
		}
		patchsetContent = rendered
	} else {
		renderedFile, ferr := ec.PathUtil.ExpandPath(raps.PatchsetFile, ec.CurrentDir, ctx.GetVariables())
		if ferr != nil {
			return result, fmt.Errorf("failed to expand patchset_file: %w", ferr)
		}
		// #nosec G304 -- patchset file from user config
		b, rerr := os.ReadFile(renderedFile)
		if rerr != nil {
			return result, fmt.Errorf("failed to read patchset file: %w", rerr)
		}
		patchsetContent = string(b)
	}

	filePatches, err := h.parsePatchset(patchsetContent)
	if err != nil {
		return result, fmt.Errorf("failed to parse patchset: %w", err)
	}
	if len(filePatches) == 0 {
		return result, fmt.Errorf("no valid patches found in patchset")
	}

	changedFiles := 0
	totalAppliedHunks := 0
	totalFailedHunks := 0
	for _, fp := range filePatches {
		target := filepath.Join(baseDir, fp.Path)
		// #nosec G304 -- target paths come from the patchset, scoped under baseDir
		content, rerr := os.ReadFile(target)
		if rerr != nil {
			// Missing target file — patch can't predictably apply.
			totalFailedHunks += len(fp.Hunks)
			continue
		}
		newContent, applied, failed := h.applyFilePatch(string(content), fp)
		totalAppliedHunks += applied
		totalFailedHunks += failed
		if newContent != string(content) {
			changedFiles++
		}
	}

	if changedFiles == 0 {
		result.Reason = fmt.Sprintf("patchset already applied (%d/%d hunks)", totalAppliedHunks, totalAppliedHunks+totalFailedHunks)
		return result, nil
	}
	result.WouldChange = true
	result.Reason = fmt.Sprintf("would apply patchset to %d file(s) (%d hunks applied, %d failed)",
		changedFiles, totalAppliedHunks, totalFailedHunks)
	return result, nil
}
