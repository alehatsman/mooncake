package file

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// FileSnapshot is the typed Before/After payload for actions.Diff when
// the resource kind is ResourceFile. Spec-22 (§"Diff") expects each
// handler family to define its own snapshot shape; consumers learn the
// type from Diff.Resource.Kind.
//
// Kept compact: a Sha256 hex + Size + Mode is enough for a machine
// consumer to know whether content/metadata diverged. The actual line-
// level Diff.Lines breakdown is a separate concern (phase 4b will wire
// the existing effects.ContentDiff into Lines; phase 4a keeps Lines
// empty since the spec calls it optional).
type FileSnapshot struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Kind   string `json:"kind,omitempty"`   // file | directory | symlink | other
	Size   int64  `json:"size,omitempty"`
	Sha256 string `json:"sha256,omitempty"` // lowercase hex; empty for non-regular files
	Mode   string `json:"mode,omitempty"`   // octal e.g. "0644"
	Target string `json:"target,omitempty"` // symlink target when Kind=="symlink"
}

// Diff implements actions.Differ for file.write (spec-22 phase 4).
// Returns a machine-readable structural delta of what this step would
// change. Pure observation — no side effects on the filesystem beyond
// the existing Stat/ReadFile reads that planning already performs.
//
// The returned Diff carries:
//   - Resource: { Kind: ResourceFile, Identifier: <resolved path> }
//   - Operation: create | update | delete | noop
//   - Before: a *FileSnapshot of current filesystem state (may have
//     Exists=false when the path is missing — that's the snapshot,
//     not nil — so the consumer can render a meaningful "before")
//   - After: a *FileSnapshot of the intended post-apply state. nil
//     for OpDelete (nothing will be there); nil for OpNoop (same
//     as Before)
//   - Lines: empty in phase 4a; will be populated by the line-diff
//     helper in phase 4b
//
// Error cases:
//   - step is nil or has no FileWrite payload → returns an error so the
//     caller can surface "this handler isn't applicable to this step"
//   - file content render fails (template error) → propagated, but only
//     for state=file/touch (the only states that need template render)
func (h *Handler) Diff(ctx actions.Context, step *config.Step) (actions.Diff, error) {
	if step == nil || step.FileWrite == nil {
		return actions.Diff{}, errors.New("file.write Diff: step has no FileWrite payload")
	}
	file := step.FileWrite

	renderedPath := resolveDiffPath(ctx, file)
	state := normalizeState(file.State)

	mode := h.parseFileMode(file.Mode, defaultModeFor(state))
	before := SnapshotPath(renderedPath)

	switch state {
	case actionTypeFile:
		return h.diffFile(ctx, file, renderedPath, mode, before)
	case "absent":
		return diffAbsent(renderedPath, before), nil
	case "directory":
		return diffDirectory(renderedPath, mode, before), nil
	case "touch":
		return diffTouch(renderedPath, mode, before), nil
	case stateLink, stateHardlink:
		return h.diffLink(ctx, file, renderedPath, state, before)
	case "perms":
		return diffPerms(renderedPath, mode, before), nil
	default:
		// Unknown / future state: degrade to a coarse update Diff so
		// consumers still get a typed answer instead of an error.
		return actions.Diff{
			Resource:  FileResource(renderedPath),
			Operation: actions.OpUpdate,
			Before:    before,
		}, nil
	}
}

// resolveDiffPath expands the step's Path through the same PathUtil
// flow Run uses, so Diff and Run see the same target. Falls back to
// the raw string when ctx isn't an ExecutionContext (e.g. a test that
// constructs a minimal context) — better to surface a structural Diff
// against the unexpanded path than to error.
func resolveDiffPath(ctx actions.Context, file *config.File) string {
	if ec, ok := ctx.(*executor.ExecutionContext); ok && ec.Svc != nil && ec.Svc.PathUtil != nil {
		if expanded, err := ec.Svc.PathUtil.ExpandPath(file.Path, ec.CurrentDir, ctx.GetVariables()); err == nil {
			return expanded
		}
	}
	return file.Path
}

// normalizeState applies the same empty→file default Run does, so Diff
// and Run agree on dispatch.
func normalizeState(s string) string {
	if s == "" {
		return actionTypeFile
	}
	return s
}

// FileResource constructs an actions.ResourceRef of kind ResourceFile
// pointing at path. Exported so sibling handlers (file.template,
// file.copy, file.download, file.unarchive) reuse one canonical
// ResourceRef shape instead of redeclaring it per package.
func FileResource(path string) actions.ResourceRef {
	return actions.ResourceRef{
		Kind:       actions.ResourceFile,
		Identifier: path,
	}
}

// SnapshotPath stats the path and returns a *FileSnapshot describing
// what's there. Always returns a non-nil snapshot — when the path
// doesn't exist, Exists=false (the caller treats this as the "Before"
// for an OpCreate). Errors during stat (permission denied etc.) are
// swallowed and reported as Exists=false; the runtime-error path
// remains as the backstop if Apply later tries and fails.
//
// Exported so sibling handlers that write to a single Dest file
// (file.template, file.copy, file.download) reuse the same Before-
// snapshot logic.
func SnapshotPath(path string) *FileSnapshot {
	snap := &FileSnapshot{Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		// errors.Is(ErrNotExist) is the common case; any other error
		// (permission denied, broken FS) also degrades to "looks
		// absent from where we stand" — caller's preflight will
		// surface the real failure.
		_ = err
		return snap
	}
	snap.Exists = true
	snap.Mode = fmt.Sprintf("%#o", info.Mode().Perm())
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		snap.Kind = "symlink"
		if target, err := os.Readlink(path); err == nil {
			snap.Target = target
		}
	case info.IsDir():
		snap.Kind = "directory"
	case info.Mode().IsRegular():
		snap.Kind = "file"
		snap.Size = info.Size()
		// Hash only regular files; reading non-regular paths
		// (devices, FIFOs, sockets) could block or fail. Errors
		// during read also degrade to "no sha256 known".
		if h, err := HashFile(path); err == nil {
			snap.Sha256 = h
		}
	default:
		snap.Kind = "other"
	}
	return snap
}

// HashFile computes lowercase-hex sha256 of the file at path. Returns
// ("", err) on read failure. Exported so sibling handlers can compute
// After.Sha256 from an on-disk source (e.g. file.copy reads Src to
// predict whether content matches Dest).
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path) //nolint:gosec // intentional: we read the target file
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// diffFile handles state=file: write content to a regular file.
func (h *Handler) diffFile(ctx actions.Context, file *config.File, path string, mode os.FileMode, before *FileSnapshot) (actions.Diff, error) {
	rendered, err := ctx.GetTemplate().Render(file.Content, ctx.GetVariables())
	if err != nil {
		return actions.Diff{}, fmt.Errorf("file.write Diff: render content: %w", err)
	}
	desired := []byte(rendered)
	desiredSum := sha256.Sum256(desired)
	desiredSha := hex.EncodeToString(desiredSum[:])

	after := &FileSnapshot{
		Path:   path,
		Exists: true,
		Kind:   "file",
		Size:   int64(len(desired)),
		Sha256: desiredSha,
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case before.Kind == "file" && before.Sha256 == desiredSha && before.Mode == after.Mode:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		After:     after,
	}, nil
}

// diffAbsent handles state=absent: remove path if present.
func diffAbsent(path string, before *FileSnapshot) actions.Diff {
	op := actions.OpDelete
	if !before.Exists {
		op = actions.OpNoop
	}
	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		// After is nil for delete — the path will not exist post-apply.
	}
}

// diffDirectory handles state=directory: ensure a directory with the
// requested mode exists at path.
func diffDirectory(path string, mode os.FileMode, before *FileSnapshot) actions.Diff {
	after := &FileSnapshot{
		Path:   path,
		Exists: true,
		Kind:   "directory",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case before.Kind == "directory" && before.Mode == after.Mode:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		After:     after,
	}
}

// diffTouch handles state=touch: file exists with the right mode;
// modtime gets updated regardless. Treated as create when absent,
// update otherwise — never noop, since touch always bumps mtime.
func diffTouch(path string, mode os.FileMode, before *FileSnapshot) actions.Diff {
	after := &FileSnapshot{
		Path:   path,
		Exists: true,
		Kind:   "file",
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
	}
	op := actions.OpUpdate
	if !before.Exists {
		op = actions.OpCreate
	}
	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		After:     after,
	}
}

// diffLink handles state=link and state=hardlink. Symlinks resolve to
// "create" when absent, "noop" when target matches, "update" otherwise.
// Hardlinks use a coarser create/update since we can't cheaply verify
// inode identity from a Stat call alone.
func (h *Handler) diffLink(ctx actions.Context, file *config.File, path, state string, before *FileSnapshot) (actions.Diff, error) {
	ec, _ := ctx.(*executor.ExecutionContext)
	src, err := h.resolveLinkSrc(ctx, ec, file)
	if err != nil {
		// Resolution failed — emit a Diff with no After so the consumer
		// still sees something structured, plus the error so callers can
		// distinguish "couldn't resolve src" from "no change".
		return actions.Diff{
			Resource:  FileResource(path),
			Operation: actions.OpUpdate,
			Before:    before,
		}, err
	}

	after := &FileSnapshot{
		Path:   path,
		Exists: true,
		Kind:   "symlink",
		Target: src,
	}
	if state == stateHardlink {
		after.Kind = "file" // hardlinks present as regular files in stat
		after.Target = ""
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case state == stateLink && before.Kind == "symlink" && before.Target == src:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		After:     after,
	}, nil
}

// diffPerms handles state=perms: only the mode bits change. Path must
// exist; if it doesn't, mark create (the existing handler errors at
// apply time — we surface a Diff anyway so the user sees what was asked
// for before the error fires).
func diffPerms(path string, mode os.FileMode, before *FileSnapshot) actions.Diff {
	after := &FileSnapshot{
		Path:   path,
		Exists: true,
		Mode:   fmt.Sprintf("%#o", mode.Perm()),
		Kind:   before.Kind,
		Size:   before.Size,
		Sha256: before.Sha256,
		Target: before.Target,
	}

	op := actions.OpUpdate
	switch {
	case !before.Exists:
		op = actions.OpCreate
	case before.Mode == after.Mode:
		op = actions.OpNoop
	}

	return actions.Diff{
		Resource:  FileResource(path),
		Operation: op,
		Before:    before,
		After:     after,
	}
}

// Compile-time interface check: confirm Handler satisfies actions.Differ
// so a future receiver narrowing or method-signature drift breaks the
// build, not the runtime.
var _ actions.Differ = (*Handler)(nil)
