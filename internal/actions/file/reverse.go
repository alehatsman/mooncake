package file

import (
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// FileReverseInfo is the pre-apply snapshot the file.write handler
// stashes on Result.ReverseData. Its contents are read by Reverse to
// construct an inverse Step.
//
// Slice A scope: capture stat-level fields only. The pre-write file
// CONTENT is not captured here — slice B (phase 5b) will add a
// Content []byte field plus a size limit so overwrite reverses
// become possible. Until then, this struct's job is to answer one
// question: "did this path exist before we ran?" — enough to reverse
// pure-create cases (the common transaction-rollback path).
type FileReverseInfo struct {
	// State is the step's normalised State at apply time. Carried so
	// Reverse knows what kind of mutation was performed and can pick
	// the right inverse shape.
	State string

	// Existed reports whether the target path was present pre-apply.
	// Drives the create-vs-overwrite split: when false, the step
	// created the path (reverse = delete); when true, the step
	// mutated existing content/state (reverse needs the pre-state
	// payload that slice B will add).
	Existed bool

	// Kind, when Existed, classifies the pre-apply target: "file",
	// "directory", "symlink", "other". Echoes the FileSnapshot.Kind
	// vocabulary so phase 5b can construct precise restore steps.
	Kind string

	// Mode is the pre-apply mode bits. Populated when Existed; zero
	// otherwise. Slice A doesn't use this yet (no mode-restore in
	// the create-only reverse cases), but capturing it now keeps the
	// shape stable for slice B.
	Mode os.FileMode
}

// CaptureReverseInfo reads the current state of path and returns a
// FileReverseInfo suitable for stashing on Result.ReverseData. Pure
// observation — no side effects.
//
// Called from file.write's Run BEFORE any mutation so Reverse later
// has the pre-state. Returns a non-nil pointer always; errors during
// stat degrade to Existed=false (a permission-denied path is opaque
// from the controller's POV, and the runtime mutation will surface
// the real error anyway).
func CaptureReverseInfo(path, state string) *FileReverseInfo {
	info := &FileReverseInfo{State: state}
	st, err := os.Lstat(path)
	if err != nil {
		// errors.Is(ErrNotExist) is the common case; permission
		// denied / broken FS also degrades to "treated as absent
		// for reverse purposes." The runtime EACCES remains the
		// backstop on apply failure.
		return info
	}
	info.Existed = true
	info.Mode = st.Mode().Perm()
	switch {
	case st.Mode()&os.ModeSymlink != 0:
		info.Kind = "symlink"
	case st.IsDir():
		info.Kind = "directory"
	case st.Mode().IsRegular():
		info.Kind = "file"
	default:
		info.Kind = "other"
	}
	return info
}

// Reverse implements actions.Reverser for file.write (spec-22 phase 5a).
//
// Slice A handles the "we created it" case — when Run captured
// Existed=false at apply time, the inverse is a state=absent step
// targeting the same path. That covers the most common transaction-
// rollback scenario: an agent creates N files, the transaction
// fails, all N get deleted.
//
// All other cases return `(nil, error)` — the spec-22 contract for
// "this action declares itself irreversible (at least for now)". The
// error message points at slice B (overwrite reverse via captured
// content) so transaction implementers know what's planned.
//
// Special return semantics:
//   - (step, nil)   → apply this step to undo
//   - (nil, nil)    → no reverse needed (Run captured no mutation;
//                     used here for cases where Reverse is meaningful
//                     but the apply-time path was a noop)
//   - (nil, error)  → cannot reverse — needs human intervention or
//                     more reverse support
func (h *Handler) Reverse(_ actions.Context, step *config.Step, result actions.Result) (*config.Step, error) {
	if step == nil || step.FileWrite == nil {
		return nil, errors.New("file.write Reverse: step has no FileWrite payload")
	}

	info, err := extractReverseInfo(result)
	if err != nil {
		return nil, fmt.Errorf("file.write Reverse: %w", err)
	}

	// Path was absent pre-apply → step created it → reverse by
	// deleting it. Works for both state=file (created file) and
	// state=directory (created dir). state=touch is debatable —
	// touch always changes mtime even on existing files, so we
	// can't tell "we created it" vs "we just bumped mtime" from
	// Existed alone. Handle touch in slice B; for now, conservative
	// irreversibility.
	if !info.Existed {
		switch info.State {
		case "", actionTypeFile, "directory":
			return &config.Step{
				Name: "reverse: delete " + step.FileWrite.Path,
				FileWrite: &config.File{
					Path:  step.FileWrite.Path,
					State: "absent",
				},
			}, nil
		}
	}

	// Path existed pre-apply OR state is one we don't yet reverse.
	// Honest error pointing at slice B.
	switch info.State {
	case "", actionTypeFile:
		return nil, errors.New(
			"file.write Reverse: cannot reverse overwrite of an existing file " +
				"in slice A; phase 5b will add content-snapshot integration")
	case "absent":
		return nil, errors.New(
			"file.write Reverse: cannot reverse a delete in slice A; " +
				"phase 5b will capture pre-delete content")
	case "directory":
		// Existing dir; if we only changed mode we'd want to
		// restore mode. Deferred to slice B.
		return nil, errors.New(
			"file.write Reverse: cannot reverse mode change on existing " +
				"directory in slice A; phase 5b will capture pre-state mode")
	case stateLink, stateHardlink:
		return nil, errors.New(
			"file.write Reverse: link/hardlink reversal not implemented; " +
				"phase 5c will handle the link family")
	case "touch":
		return nil, errors.New(
			"file.write Reverse: touch reverse not implemented; " +
				"phase 5b will capture pre-mtime for accurate restore")
	case "perms":
		return nil, errors.New(
			"file.write Reverse: perms reverse not implemented; " +
				"phase 5b will use captured pre-mode")
	default:
		return nil, fmt.Errorf("file.write Reverse: unknown state %q", info.State)
	}
}

// extractReverseInfo pulls the FileReverseInfo back out of the
// executor.Result.ReverseData slot. Type-asserts twice (interface →
// concrete *executor.Result → ReverseData → *FileReverseInfo). If
// any assert fails, returns an error pointing at the most likely
// cause so a developer hitting it can fix the capture path.
func extractReverseInfo(result actions.Result) (*FileReverseInfo, error) {
	if result == nil {
		return nil, errors.New("nil Result — apply path didn't run, or Reverse was called before Run")
	}
	r, ok := result.(*executor.Result)
	if !ok {
		return nil, fmt.Errorf("expected *executor.Result, got %T", result)
	}
	if r.ReverseData == nil {
		return nil, errors.New("no ReverseData captured — file.write Run must call CaptureReverseInfo before mutating")
	}
	info, ok := r.ReverseData.(*FileReverseInfo)
	if !ok {
		return nil, fmt.Errorf("ReverseData is %T, want *FileReverseInfo", r.ReverseData)
	}
	return info, nil
}

// Compile-time interface check.
var _ actions.Reverser = (*Handler)(nil)
