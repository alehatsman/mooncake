package copy

import (
	"errors"
	"fmt"

	"github.com/alehatsman/mooncake/internal/actions"
	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
	"github.com/alehatsman/mooncake/internal/config"
)

// Reverse implements actions.Reverser for file.copy (spec-22 phase 5
// slice D). The capture happens in Run via filehandler.CaptureReverseInfo
// against the resolved destination path; this method just unpacks
// that capture and builds the inverse Step.
//
// Slice D scope reuses the file.write slice A+B shape verbatim
// because file.copy's net effect on the destination is identical
// to a file.write — read bytes from src, write to dest with mode.
// The only handler-specific bit is which payload to consult on the
// step (FileCopy vs FileWrite). The inverse Step always uses
// FileWrite because file.copy's only mutation lives at the dest,
// and a write-bytes-back step expresses that minimally.
//
//   - !Existed → state=absent on dest (delete what we created)
//   - Existed && Kind=="file" && Content captured → state=file with
//     captured bytes + mode (Force=true)
//   - Existed && Kind=="file" && Content nil → refuse, too large
//   - Existed && Kind!="file" → refuse (would replace a dir/symlink
//     with a regular file; unsafe without multi-step orchestration)
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.FileCopy == nil {
		return nil, errors.New("file.copy Reverse: step has no FileCopy payload")
	}
	info, err := filehandler.ExtractReverseInfo(result)
	if err != nil {
		return nil, fmt.Errorf("file.copy Reverse: %w", err)
	}

	dest := step.FileCopy.Dest
	if !info.Existed {
		return filehandler.DeleteFileStep(dest), nil
	}
	if info.Kind != "file" {
		return nil, fmt.Errorf(
			"file.copy Reverse: cannot reverse a copy that replaced a %s "+
				"at the destination (only regular-file pre-state is "+
				"restorable from a single-step reverse)", info.Kind)
	}
	if info.Content == nil {
		return nil, fmt.Errorf(
			"file.copy Reverse: pre-apply file too large to snapshot "+
				"(> %d bytes); transaction layer must refuse to rollback "+
				"this step rather than partially restore",
			filehandler.MaxReverseCaptureBytes)
	}
	return filehandler.RestoreFileStep(dest, info), nil
}

// Compile-time interface check.
var _ actions.Reverser = (*Handler)(nil)
