package artifact_capture

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode inspects whether
// the artifact output directory already exists. If it does, reports
// already-captured (heuristic — recapturing would still re-run the
// inner steps, but the artifact dir is the visible marker). If it
// doesn't, reports would-capture. Deep inspection of the inner steps'
// effects in plan mode is a follow-up.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	c := step.ArtifactCapture
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	result := executor.NewResult()
	result.Checkable = true

	outputDir := c.OutputDir
	if outputDir == "" {
		outputDir = defaultOutputDir
	}

	renderedOutput, err := ec.PathUtil.ExpandPath(outputDir, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand output_dir: %w", err)
	}
	artifactDir := filepath.Join(renderedOutput, c.Name)

	if info, statErr := os.Stat(artifactDir); statErr == nil && info.IsDir() {
		result.Reason = fmt.Sprintf("artifact %q already captured at %s", c.Name, artifactDir)
		return result, nil
	}

	result.WouldChange = true
	result.Reason = fmt.Sprintf("would capture artifact %q (%d inner step(s))", c.Name, len(c.Steps))
	return result, nil
}
