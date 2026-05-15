package actions

import "os"

// Action names a primitive an Effect represents. Used in plan output,
// logging, and event emission. See Performer.
type Action string

const (
	ActionMkdir     Action = "mkdir"
	ActionWriteFile Action = "write_file"
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

// PerformerOpts carries optional flags that apply to most Performer calls.
//
// Become: when set, the underlying primitive runs via sudo (with the
// password supplied to the Performer implementation). Handlers read
// Step.Become and pass it through; they should not shell out to sudo
// themselves.
//
// Force: when set, Symlink and Hardlink replace an existing path that
// is not already of the correct link type (e.g. a directory). Without
// Force, those primitives return an error in that case.
type PerformerOpts struct {
	Become     bool
	BecomeUser string
	Force      bool
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
	WriteFile(path string, content []byte, mode os.FileMode, opts PerformerOpts) Effect

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
