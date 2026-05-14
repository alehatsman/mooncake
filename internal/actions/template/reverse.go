package template

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for file.template (spec-22
// phase 5 slice D). Net effect on the destination is identical to
// file.write / file.copy — read source, render, write to dest —
// so the inverse Step has the same shape: either delete the
// created dest (create case) or restore the pre-apply bytes +
// mode (overwrite case).
//
// The inverse Step always uses FileWrite because file.template's
// only externally observable mutation is at the dest path; the
// source template is never modified, so there's nothing on the
// template side that needs unwinding.
//
//   - !Existed → state=absent on dest
//   - Existed && Kind=="file" && Content captured → state=file
//     with captured bytes + mode (Force=true)
//   - Existed && Kind=="file" && Content nil → refuse, too large
//   - Existed && Kind!="file" → refuse (would need multi-step
//     replacement of a dir/symlink)
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.FileTemplate == nil {
		return nil, errors.New("file.template Reverse: step has no FileTemplate payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.FileTemplate.Dest, result, "file.template")
}

// Compile-time interface check.
var _ actions.Reverser = (*Handler)(nil)
