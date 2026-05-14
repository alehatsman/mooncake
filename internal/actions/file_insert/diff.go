package file_insert

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for text.insert (spec-22 phase 4c).
//
// text.insert inserts content at an anchor-defined position in an
// existing file. The post-mutation content depends on whether the
// anchor matches, AllowMultiple semantics, and Position (before/after).
// Conservative shape: always OpUpdate, Before from current FS, After
// with empty Sha256.
//
// A later phase MAY detect noop when the content-to-insert already
// exists at every match (the runtime idempotency check) and surface
// that as OpNoop, but Diff currently doesn't run the anchor scan.
func (h *Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.TextInsert == nil {
		return actions.Diff{}, errors.New("text.insert Diff: step has no TextInsert payload")
	}
	return filehandler.DiffSinglePathMutation(filehandler.ExpandPath(ctx, step.TextInsert.Path)), nil
}

var _ actions.Differ = (*Handler)(nil)
