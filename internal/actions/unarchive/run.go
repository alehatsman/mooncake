package unarchive

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Run is the Spec 16 unified entry point. Plan mode inspects:
//   - if `creates:` is set and the path exists, the extraction would
//     be skipped (already-ok)
//   - if `creates:` is unset or the path is missing, the extraction
//     would run (would-change)
//
// Execute mode delegates to the legacy Execute path which does the
// actual archive walking and writing.
//
// Limitation: when `creates:` is not configured, plan mode cannot
// tell whether the archive contents already match the destination
// tree without opening the archive and walking it (which is
// format-specific: tar, tar.gz, zip, etc). Users who want accurate
// plan output should set `creates:` to a path that exists after
// successful extraction — the standard idempotency pattern for this
// action.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	ua := step.Unarchive
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	result := executor.NewResult()
	result.Checkable = true

	if ua.Creates != "" {
		renderedCreates, err := ec.PathUtil.ExpandPath(ua.Creates, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("failed to expand creates path: %w", err)
		}
		if _, statErr := os.Stat(renderedCreates); statErr == nil {
			result.Reason = "creates path already exists"
			return result, nil
		}
		result.WouldChange = true
		result.Reason = "would extract (creates path missing)"
		return result, nil
	}

	// No creates marker — the legacy Execute path extracts unconditionally,
	// so plan mode reports would-change.
	result.WouldChange = true
	result.Reason = "would extract (no creates marker)"
	return result, nil
}
