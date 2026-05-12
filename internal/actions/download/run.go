package download

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/utils"
)

// Run is the Spec 16 unified entry point. Plan mode inspects the
// destination file: if it exists with a matching checksum (or
// force=false and no checksum specified), reports already-ok;
// otherwise reports would-download. Execute mode delegates to the
// legacy Execute path which performs the HTTP fetch.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	d := step.FileDownload
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	renderedDest, err := ec.PathUtil.ExpandPath(d.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand dest path: %w", err)
	}

	_, statErr := os.Stat(renderedDest)
	destExists := statErr == nil

	if !destExists {
		result.WouldChange = true
		result.Reason = "would download (destination missing)"
		return result, nil
	}

	if d.Force {
		result.WouldChange = true
		result.Reason = "would download (force=true)"
		return result, nil
	}

	if d.Checksum != "" {
		matches, cerr := utils.VerifyChecksum(renderedDest, d.Checksum)
		if cerr != nil {
			result.WouldChange = true
			result.Reason = fmt.Sprintf("would download (cannot verify existing checksum: %v)", cerr)
			return result, nil
		}
		if !matches {
			result.WouldChange = true
			result.Reason = "would download (existing file checksum mismatch)"
			return result, nil
		}
		result.Reason = "destination exists with correct checksum"
		return result, nil
	}

	// No checksum, no force, dest exists → legacy Execute also skips,
	// so plan reports already-ok.
	result.Reason = "destination already exists (no checksum / force to re-download)"
	return result, nil
}
