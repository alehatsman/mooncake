//nolint:revive // package name follows action convention (text_patch)
package text_patch

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file_write"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.patch.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextPatch == nil {
		return nil, errors.New("text.patch Reverse: step has no TextPatch payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextPatch.Path, result, "text.patch")
}

var _ actions.Reverser = (*Handler)(nil)
