package text_line

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for text.line (spec-22 phase
// 5 slice E). text.line's net effect on the filesystem is "mutate
// content at this path" — same shape as file.copy / file.template,
// so the canonical filehandler.ReverseInPlaceFileMutation helper
// produces the right inverse Step.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.TextLine == nil {
		return nil, errors.New("text.line Reverse: step has no TextLine payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.TextLine.Path, result, "text.line")
}

var _ actions.Reverser = (*Handler)(nil)
