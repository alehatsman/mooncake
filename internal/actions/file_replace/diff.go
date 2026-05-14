package file_replace

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for text.replace (spec-22 phase 4c).
//
// text.replace performs an in-place regex substitution on an existing
// file. The post-mutation content depends on the pattern + replace +
// flags + count combination interacting with the current file content;
// simulating that in Diff would duplicate substantial regex logic for
// little gain. So phase 4c is conservative: Operation is always
// OpUpdate, Before is a real snapshot, After signals "the file still
// exists, kind=file, mode preserved" with Sha256 left empty (unknown).
//
// A later phase MAY upgrade specific predictable cases (e.g. pattern
// doesn't match → OpNoop) by running the regex against the existing
// content. The conservative shape stays correct in the meantime.
func (h *Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.TextReplace == nil {
		return actions.Diff{}, errors.New("text.replace Diff: step has no TextReplace payload")
	}
	return filehandler.DiffSinglePathMutation(filehandler.ExpandPath(ctx, step.TextReplace.Path)), nil
}

// Compile-time interface check.
var _ actions.Differ = (*Handler)(nil)
