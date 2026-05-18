package actions

import "os"

// Action names a primitive an Effect represents. Used in plan output,
// logging, and event emission. See Performer.
type Action string

const (
	ActionMkdir     Action = "mkdir"
	ActionWriteFile Action = "write_file"
	ActionCopyFile  Action = "copy_file"
	ActionSymlink   Action = "symlink"
	ActionHardlink  Action = "hardlink"
	ActionTouch     Action = "touch"
	ActionRemove    Action = "remove"
	ActionChmod     Action = "chmod"
	ActionChown     Action = "chown"
	ActionRunCmd    Action = "run_command"
)

// Effect is the result of a Performer call.
//
// Field semantics by mode:
//
//	ModeApply:
//	    Performed   true if a side effect actually happened
//	    AlreadyOk   true if the target was already in desired state (no-op)
//	    WouldChange unused (false)
//	    Err         any error from the underlying syscall / command
//
//	ModePlan:
//	    Performed   false (no side effects in plan mode)
//	    AlreadyOk   true if the target is already in desired state
//	    WouldChange true if applying ModeApply would change state
//	    Err         any error encountered while *inspecting* state
//
// Performed and WouldChange are mutually exclusive; AlreadyOk is set when
// the operation would be a no-op in either mode.
type Effect struct {
	Action      Action
	Path        string
	Reason      string
	Performed   bool
	WouldChange bool
	AlreadyOk   bool
	Err         error
	Detail      any
}

// Changed reports whether this Effect represents a state change — either
// performed (ModeApply) or predicted (ModePlan).
func (e Effect) Changed() bool {
	return e.Performed || e.WouldChange
}

// PerformerOpts carries optional flags that apply to most Performer
// calls. Become/BecomeUser were removed in spec-72 Layer C: the
// step's AsUser is bound onto the Performer at ec.Effects() time and
// drives escalation transparently, so handlers no longer pass a
// per-call become flag.
//
// Force: when set, Symlink and Hardlink replace an existing path that
// is not already of the correct link type (e.g. a directory). Without
// Force, those primitives return an error in that case.
//
// ExplicitMode signals that the caller's `mode` was supplied directly
// (e.g. `file.copy: mode: '0755'`) rather than derived from the source
// file. WriteFile/CopyFile use this to decide whether to enforce the
// requested mode on an existing dest or preserve the dest's current
// mode (the WriteFile-compatible round-trip default).
type PerformerOpts struct {
	Force        bool
	ExplicitMode bool
}

// Performer executes filesystem and command primitives in either
// ModeApply (real side effects) or ModePlan (state inspection only,
// returning a prediction).
//
// Spec 16 introduces Performer so that mutating primitives have exactly
// one site that decides what each operation means in each mode. Handlers
// should call Performer methods (via ctx-supplied accessor) instead of
// calling os.* directly.
type Performer interface {
	// Mode reports the mode the Performer will use for the next call.
	Mode() Mode

	// Mkdir ensures a directory exists at path with the given mode.
	// Parents are created as needed.
	Mkdir(path string, mode os.FileMode, opts PerformerOpts) Effect

	// WriteFile writes content to path with the given mode, creating
	// parent directories as needed. Idempotent: if existing content
	// already matches, AlreadyOk is set.
	//
	// content is held in memory by the implementation; use CopyFile
	// instead when the source is already a file on disk.
	WriteFile(path string, content []byte, mode os.FileMode, opts PerformerOpts) Effect

	// CopyFile streams the file at src to dest with the given mode,
	// creating parent directories as needed. Memory usage is bounded
	// regardless of file size — the source is never loaded into a
	// single []byte (unlike WriteFile + os.ReadFile, which has been
	// the historical shape used by the copy action).
	//
	// Idempotent: if dest already exists with the same size, content
	// (verified by streaming sha256 of both files), and mode, AlreadyOk
	// is set without performing a copy. This keeps Plan-mode honest
	// for large files without loading them into memory.
	//
	// The src file must exist and be a regular file. dest is created
	// or truncated. Symlinks at src are followed; symlinks at dest are
	// replaced.
	CopyFile(src, dest string, mode os.FileMode, opts PerformerOpts) Effect

	// Symlink creates a symbolic link at path pointing to target. If a
	// link already exists with the correct target, AlreadyOk is set.
	Symlink(target, path string, opts PerformerOpts) Effect

	// Hardlink creates a hard link at path pointing to target.
	Hardlink(target, path string, opts PerformerOpts) Effect

	// Touch updates mtime, creating an empty file with the given mode
	// if absent. Not idempotent: WouldChange is always true in ModePlan.
	Touch(path string, mode os.FileMode, opts PerformerOpts) Effect

	// Remove deletes path. If recursive is true, removes directories
	// and their contents.
	Remove(path string, recursive bool, opts PerformerOpts) Effect

	// Chmod sets the permission bits on path.
	Chmod(path string, mode os.FileMode, opts PerformerOpts) Effect

	// Chown sets owner and group on path. Empty owner or group leaves
	// that side unchanged. Owner/group may be names or numeric IDs.
	Chown(path, owner, group string, opts PerformerOpts) Effect
}
