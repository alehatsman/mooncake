//nolint:revive // package name follows action convention (file_delete_range)
package file_delete_range

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.delete_range.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextDeleteRange == nil {
		return nil, errors.New("text.delete_range Reverse: step has no TextDeleteRange payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextDeleteRange.Path, result, "text.delete_range")
}

var _ actions.Reverser = (*Handler)(nil)
