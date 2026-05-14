//nolint:revive // package name follows action convention
package text_patch_yaml

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.patch.yaml.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextPatchYAML == nil {
		return nil, errors.New("text.patch.yaml Reverse: step has no TextPatchYAML payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextPatchYAML.Path, result, "text.patch.yaml")
}

var _ actions.Reverser = (*Handler)(nil)
