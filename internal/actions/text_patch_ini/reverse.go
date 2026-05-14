//nolint:revive // package name follows action convention
package text_patch_ini

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.patch.ini.
// See filehandler.ReverseInPlaceFileMutation for the shared
// reverse-step construction logic.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextPatchINI == nil {
		return nil, errors.New("text.patch.ini Reverse: step has no TextPatchINI payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextPatchINI.Path, result, "text.patch.ini")
}

var _ actions.Reverser = (*Handler)(nil)
