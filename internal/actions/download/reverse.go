package download

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for file.download (spec-22
// phase 5 slice F). Net effect on the filesystem is "fetch URL,
// write result to dest" — same shape as file.copy / file.template,
// so the shared filehandler.ReverseInPlaceFileMutation helper
// covers it.
//
// Note: HTTP traffic itself is not reversed — only the destination
// file's filesystem state. If a transaction layer needs to "un-fetch"
// the URL (e.g. revoke an OAuth-protected one-time download), that
// concern belongs above the handler.
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.FileDownload == nil {
		return nil, errors.New("file.download Reverse: step has no FileDownload payload")
	}
	return filehandler.ReverseInPlaceFileMutation(step.FileDownload.Dest, result, "file.download")
}

var _ actions.Reverser = (*Handler)(nil)
