//nolint:revive // package name follows action convention (file_insert)
package file_insert

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.insert.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextInsert == nil {
		return nil, errors.New("text.insert Reverse: step has no TextInsert payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextInsert.Path, result, "text.insert")
}

var _ actions.Reverser = (*Handler)(nil)
