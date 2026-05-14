package file

import (
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// MaxReverseCaptureBytes caps how much pre-apply content slice B
// snapshots into FileReverseInfo.Content. Set deliberately small:
// reverse is for config files and small artefacts, not for binary
// blobs or large logs. Above this threshold Content stays nil and
// Reverse refuses with an explicit "too large to snapshot" error
// so a transaction layer fails loudly rather than rolling back
// partially.
const MaxReverseCaptureBytes = 4 * 1024 * 1024

// FileReverseInfo is the pre-apply snapshot the file.write handler
// stashes on Result.ReverseData. Its contents are read by Reverse to
// construct an inverse Step.
//
// Slice A captured stat-level fields; slice B adds Content so
// overwrites and deletes of regular files become reversible. The
// "did this path exist before we ran?" question is still the
// primary axis — Existed is the gate that picks the create branch
// vs the overwrite/delete branch in Reverse.
type FileReverseInfo struct {
	// State is the step's normalised State at apply time. Carried so
	// Reverse knows what kind of mutation was performed and can pick
	// the right inverse shape.
	State string

	// Existed reports whether the target path was present pre-apply.
	// Drives the create-vs-overwrite split: when false, the step
	// created the path (reverse = delete); when true, the step
	// mutated existing content/state and Reverse needs Content +
	// Mode to reconstruct the original.
	Existed bool

	// Kind, when Existed, classifies the pre-apply target: "file",
	// "directory", "symlink", "other". Echoes the FileSnapshot.Kind
	// vocabulary so reverse can pick the right restore shape.
	Kind string

	// Mode is the pre-apply mode bits. Populated when Existed; zero
	// otherwise. Slice B uses this for overwrite/delete restore
	// (so the reverse step writes back the original permissions)
	// and for state=perms reverse (where mode IS the change).
	Mode os.FileMode

	// Content holds the pre-apply bytes of the file when Kind=="file"
	// and size <= MaxReverseCaptureBytes. nil otherwise. nil for a
	// regular file means "exceeded the size cap" — Reverse uses that
	// distinction to refuse explicitly rather than silently mangle
	// the rollback.
	//
	// Templating note: the reverse step puts Content into
	// config.File.Content as a string, which runs through Pongo2
	// rendering on apply. Pre-rendered content that happens to
	// contain `{{` or `{%` literals would be re-rendered on the way
	// back — vanishingly rare for typical config-file payloads, but
	// a known sharp edge until we add a "raw content" path in a
	// future slice.
	Content []byte
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

	// Slice B: snapshot bytes for regular files under the size cap.
	// Directories have no inherent content; symlinks have a target
	// (slice C will use that); files over the cap stay nil and
	// Reverse refuses with an explicit size error.
	//
	// Read errors degrade to nil — same rationale as the Lstat
	// branch above: the runtime mutation will surface the real
	// error, no point double-reporting here.
	if info.Kind == "file" && st.Size() <= MaxReverseCaptureBytes {
		if data, readErr := os.ReadFile(path); readErr == nil {
			info.Content = data
		}
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

	path := step.FileWrite.Path

	// Path was absent pre-apply → step created it → reverse by
	// deleting it. Works for both state=file (created file) and
	// state=directory (created dir). state=touch is debatable —
	// touch always changes mtime even on existing files, so we
	// can't tell "we created it" vs "we just bumped mtime" from
	// Existed alone — defer to a future slice.
	if !info.Existed {
		switch info.State {
		case "", actionTypeFile, "directory":
			return &config.Step{
				Name: "reverse: delete " + path,
				FileWrite: &config.File{
					Path:  path,
					State: "absent",
				},
			}, nil
		}
	}

	// Path existed pre-apply. Slice B restores from the captured
	// content + mode for regular-file overwrites and deletes, and
	// from mode alone for state=perms.
	switch info.State {
	case "", actionTypeFile:
		// Overwrite reverse: rewrite the pre-apply bytes with the
		// pre-apply mode. Only works when the original was a
		// regular file we managed to snapshot.
		if info.Kind != "file" {
			return nil, fmt.Errorf(
				"file.write Reverse: cannot reverse overwrite of a %s "+
					"(only regular-file pre-state can be restored)", info.Kind)
		}
		if info.Content == nil {
			return nil, fmt.Errorf(
				"file.write Reverse: pre-apply file too large to snapshot "+
					"(> %d bytes); transaction layer must refuse to rollback "+
					"this step rather than partially restore", MaxReverseCaptureBytes)
		}
		return restoreFileStep(path, info), nil

	case "absent":
		// Delete reverse: re-create with pre-apply bytes + mode.
		// Directory deletions deferred — recreating a tree needs a
		// listing of its prior contents, which slice E will add.
		if info.Kind != "file" {
			return nil, fmt.Errorf(
				"file.write Reverse: cannot un-delete a %s in slice B; "+
					"directory and link recreate paths land in later slices", info.Kind)
		}
		if info.Content == nil {
			return nil, fmt.Errorf(
				"file.write Reverse: pre-delete file too large to snapshot "+
					"(> %d bytes); cannot reconstruct", MaxReverseCaptureBytes)
		}
		return restoreFileStep(path, info), nil

	case "perms":
		// Mode change on an existing path. Reverse step rewrites
		// the original mode bits — no content touched. Works
		// uniformly for files and directories.
		return &config.Step{
			Name: "reverse: restore perms on " + path,
			FileWrite: &config.File{
				Path:  path,
				State: "perms",
				Mode:  formatReverseMode(info.Mode),
			},
		}, nil

	case "directory":
		// Existing dir target with state=directory means a mode
		// change at most (mkdir is a no-op on existing dirs). Use
		// a state=perms reverse step to restore the original mode.
		return &config.Step{
			Name: "reverse: restore dir perms on " + path,
			FileWrite: &config.File{
				Path:  path,
				State: "perms",
				Mode:  formatReverseMode(info.Mode),
			},
		}, nil

	case stateLink, stateHardlink:
		return nil, errors.New(
			"file.write Reverse: link/hardlink reversal not implemented; " +
				"phase 5 slice C will handle the link family")
	case "touch":
		return nil, errors.New(
			"file.write Reverse: touch reverse not implemented; " +
				"future slice will capture pre-mtime for accurate restore")
	default:
		return nil, fmt.Errorf("file.write Reverse: unknown state %q", info.State)
	}
}

// restoreFileStep builds the inverse Step for an overwrite or
// delete reverse: write the captured bytes back at the captured
// mode. Shared so the two callers stay textually identical.
func restoreFileStep(path string, info *FileReverseInfo) *config.Step {
	return &config.Step{
		Name: "reverse: restore " + path,
		FileWrite: &config.File{
			Path:    path,
			State:   actionTypeFile,
			Content: string(info.Content),
			Mode:    formatReverseMode(info.Mode),
			Force:   true, // overwrite whatever the failing apply left behind
		},
	}
}

// formatReverseMode renders an os.FileMode in the octal form
// config.File.Mode expects ("0644", "0755", …). Returns the
// empty string for zero — callers can decide whether to omit the
// field or pass it through.
func formatReverseMode(m os.FileMode) string {
	if m == 0 {
		return ""
	}
	return fmt.Sprintf("%#o", m)
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
