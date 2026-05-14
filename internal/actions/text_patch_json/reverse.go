//nolint:revive // package name follows action convention
package text_patch_json

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.patch.json.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextPatchJSON == nil {
		return nil, errors.New("text.patch.json Reverse: step has no TextPatchJSON payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextPatchJSON.Path, result, "text.patch.json")
}

var _ actions.Reverser = (*Handler)(nil)
