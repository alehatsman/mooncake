package file_patch_apply

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Diff implements actions.Differ for text.patch (spec-22 phase 4c).
//
// text.patch applies a unified-diff-style patch (either inline Patch or
// loaded from PatchFile) to an existing file. Simulating the apply
// requires running the patch engine over the current content; that's
// substantial logic to duplicate in Diff. Conservative shape: always
// OpUpdate, Before from current FS, After with empty Sha256.
//
// Note that PatchFile (when used) is a read-only INPUT — the spec-22
// PermissionSet for text.patch correctly omits it from FilesystemWrite,
// and Diff likewise doesn't include it in the Resource ref. Only Path
// (the mutation target) appears as the diff's Resource.
func (h *Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.TextPatch == nil {
		return actions.Diff{}, errors.New("text.patch Diff: step has no TextPatch payload")
	}
	return filehandler.DiffSinglePathMutation(filehandler.ExpandPath(ctx, step.TextPatch.Path)), nil
}

var _ actions.Differ = (*Handler)(nil)
