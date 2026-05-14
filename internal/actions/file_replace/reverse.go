//nolint:revive // package name follows action convention (file_replace)
package file_replace

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.replace.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextReplace == nil {
		return nil, errors.New("text.replace Reverse: step has no TextReplace payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextReplace.Path, result, "text.replace")
}

var _ actions.Reverser = (*Handler)(nil)
