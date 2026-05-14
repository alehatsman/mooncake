package file_delete_range

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for text.delete_range (spec-22
// phase 4c).
//
// text.delete_range removes content between StartAnchor and EndAnchor
// inclusive/exclusive of the anchor lines. The After content depends on
// whether the anchors match at all, where they match, and the inclusive
// flag — too much branching to simulate cheaply here. Conservative
// shape: always OpUpdate, Before from current FS, After with empty
// Sha256.
func (h *Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.TextDeleteRange == nil {
		return actions.Diff{}, errors.New("text.delete_range Diff: step has no TextDeleteRange payload")
	}
	return filehandler.DiffSinglePathMutation(filehandler.ExpandPath(ctx, step.TextDeleteRange.Path)), nil
}

var _ actions.Differ = (*Handler)(nil)
