// Package effects provides the default implementation of
// actions.Performer — mode-aware filesystem and command primitives used
// by action handlers.
//
// Spec 16 (docs-working/specs/done/spec-16-unify-dryrun-execute.md) collapses each
// handler's parallel Execute / DryRun / Check methods into a single
// Run(ctx, step) method. Inside Run, handlers call Performer methods
// instead of os.* directly. The Performer consults the current mode and
// either performs the side effect (ModeApply) or returns a prediction
// (ModePlan).
package effects

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/security"
)

// ModeFunc returns the current actions.Mode. The Performer calls it on
// each operation so callers can swap modes between calls (rare;
// typically pinned per execution context).
type ModeFunc func() actions.Mode

// defaultPerformer is the production implementation of actions.Performer.
type defaultPerformer struct {
	modeFn           ModeFunc
	sudoPass         string
	passwordlessSudo bool
	// asUser is the step's bound AsUser, threaded in by
	// ec.Effects() before the handler sees the Performer. Empty
	// means "do not escalate"; "root"/"0" means sudo; "<name>"
	// means sudo + chown the resulting file to <name>'s uid:gid
	// so the named-user file.write / template / copy works as
	// the operator expects. Spec-72 Layer C.
	asUser string
}

// NewPerformer constructs an actions.Performer that performs real
// filesystem operations in ModeApply and inspects state in ModePlan.
// modeFn is called once per primitive to decide the path; sudoPass
// is consulted when escalation is needed; passwordlessSudo lets a
// NOPASSWD operator skip configuring a password — runSudo then uses
// `sudo -n`. asUser is the step's bound AsUser (spec-72 Layer C):
// empty → no escalation, "root"/"0" → sudo to root, "<name>" → sudo
// to root + post-write chown to <name>.
func NewPerformer(modeFn ModeFunc, sudoPass string, passwordlessSudo bool, asUser string) actions.Performer {
	return &defaultPerformer{
		modeFn:           modeFn,
		sudoPass:         sudoPass,
		passwordlessSudo: passwordlessSudo,
		asUser:           asUser,
	}
}

func (p *defaultPerformer) Mode() actions.Mode { return p.modeFn() }

// becomeFallback (spec-69 phase 5b, spec-72 Layer C) is the "try
// direct first, sudo on EACCES" pattern. The gating shifted from
// per-call opts.Become to the per-step bound AsUser: a step that
// didn't declare as_user gets no fallback (direct error propagates),
// matching the "step is the source of truth" invariant.
//
// The pattern still amortises two real cases:
//
//   - tests pointing /etc-shaped paths at a t.TempDir() and asking
//     for a sudo password they don't have configured;
//   - already-root invocations (sudo mooncake apply, or mooncake
//     running under a unit with User=root) paying the cost of a
//     sudo -S wrap for every primitive.
//
// directErr is the result of the bare os.* / direct call; sudoCmd
// is the shell-quoted command to retry under sudo when fallback is
// appropriate.
func (p *defaultPerformer) becomeFallback(_ actions.PerformerOpts, directErr error, sudoCmd string) error {
	if directErr == nil {
		return nil
	}
	if p.asUser == "" || !os.IsPermission(directErr) {
		return directErr
	}
	// Already root: direct failed for a reason sudo can't help with.
	if os.Geteuid() == 0 {
		return directErr
	}
	return p.runSudo(sudoCmd)
}

// chownSpec returns the "uid:gid" string for the bound AsUser (or
// empty when no chown is needed — empty AsUser or root/0). Used by
// the sudo paths in WriteFile / CopyFile / Mkdir / Touch to chown
// the resulting path to the named user after writing as root. Resolved
// once per call (rare enough that caching isn't worth the complexity).
func (p *defaultPerformer) chownSpec() string {
	if p.asUser == "" || p.asUser == "root" || p.asUser == "0" {
		return ""
	}
	u, err := user.Lookup(p.asUser)
	if err != nil {
		// Lookup failure surfaces as an empty spec; the sudo path
		// completes without chown and the file lands owned by root.
		// Operator will see the mis-ownership and can fix the user
		// resolution before re-applying. We prefer this to failing
		// the whole apply because the write itself succeeded.
		return ""
	}
	return u.Uid + ":" + u.Gid
}

// withChown appends a `&& chown uid:gid <path>` clause to a sudo
// command line when the bound AsUser is a named non-root user.
// Returns the original sudoCmd untouched for AsUser="" / "root" / "0".
// path is the destination the chown should target — the sudo command
// is assumed to have already moved/created the file at that location.
func (p *defaultPerformer) withChown(sudoCmd, path string) string {
	spec := p.chownSpec()
	if spec == "" {
		return sudoCmd
	}
	return sudoCmd + " && chown " + spec + " " + ShellQuote(path)
}

// needSudoForOwnership reports whether a create-style operation must
// skip the direct path and go through sudo to land the resulting
// file owned by the bound AsUser. Direct write/mkdir/touch produces a
// file owned by the current process's uid; if the operator declared
// `as_user: postgres` but mooncake runs as alice, the direct path
// would silently mis-own the file. This check forces sudo (which
// runs as root and bundles a chown clause via withChown) whenever
// the asUser target doesn't match the current process identity.
//
// The check returns false for the no-op cases:
//   - AsUser empty → no escalation, no ownership constraint.
//   - AsUser is "root"/"0" AND mooncake is already root → direct
//     write produces a root-owned file (correct).
//   - AsUser is a name AND the current process's username matches
//     → direct write produces a matching-owned file (correct).
//
// Spec-72 follow-up: closes the documented caveat that direct
// writes silently mis-owned files for named non-current AsUser.
func (p *defaultPerformer) needSudoForOwnership() bool {
	if p.asUser == "" {
		return false
	}
	if p.asUser == "root" || p.asUser == "0" {
		return os.Geteuid() != 0
	}
	current, err := user.Current()
	if err != nil {
		// user.Current shouldn't fail on any supported platform —
		// /etc/passwd would have to be unreadable. Err on the safe
		// side: force sudo so the chown happens via root.
		return true
	}
	return current.Username != p.asUser
}

// ----------------------------------------------------------------------
// Mkdir
// ----------------------------------------------------------------------

func (p *defaultPerformer) Mkdir(path string, mode os.FileMode, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionMkdir, Path: path}

	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir() && modeMatches(info.Mode(), mode):
		e.AlreadyOk = true
		e.Reason = "directory exists with desired mode"
		return e
	case err == nil && info.IsDir():
		e.Reason = fmt.Sprintf("would chmod %s -> %s", info.Mode().Perm(), mode)
	case err == nil && !info.IsDir():
		e.Reason = "path exists but is not a directory"
		e.Err = fmt.Errorf("%s exists and is not a directory", path)
		return e
	case os.IsNotExist(err):
		e.Reason = "would create directory"
	default:
		e.Err = fmt.Errorf("stat %s: %w", path, err)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Spec-72 follow-up: skip direct mkdir when ownership wouldn't
	// match the bound AsUser AND we're actually creating the dir
	// (not adjusting mode on a pre-existing one). Direct mkdir of
	// a NEW dir produces a directory owned by the current uid; for
	// named non-current AsUser, that silently lands the wrong owner.
	// Force sudo so the bundled chown in the sudo command fixes
	// ownership. When the dir already exists, we're just chmod-ing
	// it — ownership is the operator's existing choice and we
	// don't reassign it without explicit intent.
	//
	// Otherwise try direct first; fall back to sudo on EACCES when
	// the bound AsUser indicates escalation is wanted (spec-72 Layer
	// C). The sudo command combines mkdir + chmod + (optionally) chown
	// in one shell so the resulting state is atomic from sudo's
	// perspective.
	sudoCmd := p.withChown(fmt.Sprintf("mkdir -p -m %s %s && chmod %s %s",
		formatMode(mode), ShellQuote(path), formatMode(mode), ShellQuote(path)), path)
	dirAlreadyExists := info != nil && info.IsDir()
	if !dirAlreadyExists && p.needSudoForOwnership() {
		if err := p.runSudo(sudoCmd); err != nil {
			e.Err = fmt.Errorf("mkdir %s: %w", path, err)
			return e
		}
		e.Performed = true
		return e
	}
	mkdirErr := os.MkdirAll(path, mode)
	if err := p.becomeFallback(opts, mkdirErr, sudoCmd); err != nil {
		e.Err = fmt.Errorf("mkdir %s: %w", path, err)
		return e
	}
	// If mkdir went via sudo, the chmod was bundled in. Otherwise do
	// the chmod directly (with the same fallback so the chmod-only
	// EACCES case retries under sudo too).
	if mkdirErr == nil {
		chmodErr := os.Chmod(path, mode)
		if err := p.becomeFallback(opts, chmodErr,
			fmt.Sprintf("chmod %s %s", formatMode(mode), ShellQuote(path)),
		); err != nil {
			e.Err = fmt.Errorf("chmod %s: %w", path, err)
			return e
		}
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// WriteFile
// ----------------------------------------------------------------------

func (p *defaultPerformer) WriteFile(path string, content []byte, mode os.FileMode, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionWriteFile, Path: path}

	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		e.Err = fmt.Errorf("%s exists and is a directory", path)
		return e
	case err == nil:
		existing, readErr := os.ReadFile(path)
		if readErr != nil {
			e.Reason = "existing file unreadable; would overwrite"
		} else if bytes.Equal(existing, content) && modeMatches(info.Mode(), mode) {
			e.AlreadyOk = true
			e.Reason = "file content and mode already match"
			return e
		} else if !bytes.Equal(existing, content) {
			e.Reason = fmt.Sprintf("content differs (%d -> %d bytes)", len(existing), len(content))
			e.Detail = newContentDiff(path, existing, content)
		} else {
			e.Reason = fmt.Sprintf("would chmod %s -> %s", info.Mode().Perm(), mode)
		}
	case os.IsNotExist(err):
		e.Reason = fmt.Sprintf("would create file (%d bytes)", len(content))
	default:
		e.Err = fmt.Errorf("stat %s: %w", path, err)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Create the parent dir when it is not already a directory we can see,
	// routing that through Mkdir so escalation applies: under as_user (or
	// become) a freshly created parent is made via privileged `mkdir -p` +
	// chown, so it is owned by as_user rather than the invoking user
	// (issue #95). The `statErr != nil` arm also covers the original #95
	// case — a missing subdir under a root-owned tree the invoker can't
	// even traverse — since Mkdir runs under the privileged runner instead
	// of EACCESing like a bare os.MkdirAll. A *stattable existing* dir is
	// left completely untouched (never re-chmod'd / re-chown'd), preserving
	// a deliberately-tightened mode such as a 0700 ~/.ssh. With no
	// escalation bound, Mkdir falls through to a plain os.MkdirAll.
	if parent := filepath.Dir(path); parent != "" {
		if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
			if me := p.Mkdir(parent, 0o755, opts); me.Err != nil {
				e.Err = fmt.Errorf("mkdir parent %s: %w", parent, me.Err)
				return e
			}
		}
	}

	// Spec-72 follow-up: skip the direct path when ownership wouldn't
	// match the bound AsUser. Direct write produces a file owned by
	// the current process's uid; for `as_user: postgres` on a host
	// where mooncake runs as alice, that's silently wrong. Force
	// sudo so the bundled chown clause lands the correct owner.
	//
	// Otherwise: try direct first; fall back to sudo on EACCES when
	// the bound AsUser indicates escalation is wanted (spec-72 Layer
	// C). The sudo path stages content in a user-writable tempfile
	// and then mv+chmod under sudo (the user's process can't write
	// directly into /etc/foo, but it CAN write into /tmp and ask sudo
	// to move it).
	if !p.needSudoForOwnership() {
		// #nosec G306 — mode is caller-controlled; this is a provisioning tool.
		directErr := os.WriteFile(path, content, mode)
		if directErr == nil {
			e.Performed = true
			return e
		}
		if p.asUser == "" || !os.IsPermission(directErr) || os.Geteuid() == 0 {
			e.Err = fmt.Errorf("write %s: %w", path, directErr)
			return e
		}
		// Direct EPERM with escalation available → fall through.
	}
	if err := p.sudoWriteFile(path, content, mode); err != nil {
		e.Err = err
		return e
	}
	e.Performed = true
	return e
}

// sudoWriteFile stages content in a user-writable temp file then
// invokes sudo to mv + chmod (+ chown when the bound AsUser is named
// non-root). Extracted from WriteFile so the ownership-forced and
// EACCES-fallback paths share the same implementation.
func (p *defaultPerformer) sudoWriteFile(path string, content []byte, mode os.FileMode) error {
	tmp, terr := os.CreateTemp("", "mooncake-effect-*")
	if terr != nil {
		return fmt.Errorf("create temp file: %w", terr)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
		ShellQuote(tmpPath), ShellQuote(path), formatMode(mode), ShellQuote(path))
	cmd = p.withChown(cmd, path)
	return p.runSudo(cmd)
}

// ContentDiff is a small structured summary attached to Effect.Detail
// for WriteFile when content would change.
type ContentDiff struct {
	OldSize int    `json:"old_size"`
	NewSize int    `json:"new_size"`
	OldHash string `json:"old_hash"`
	NewHash string `json:"new_hash"`
	// UnifiedDiff is the unified diff text. Empty for binary files or new files.
	UnifiedDiff string `json:"unified_diff,omitempty"`
}

func newContentDiff(path string, oldB, newB []byte) ContentDiff {
	return ContentDiff{
		OldSize:     len(oldB),
		NewSize:     len(newB),
		OldHash:     shortHash(oldB),
		NewHash:     shortHash(newB),
		UnifiedDiff: unifiedDiff(path, oldB, newB),
	}
}

func shortHash(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:6])
}

// ----------------------------------------------------------------------
// CopyFile
// ----------------------------------------------------------------------

// CopyFile streams src to dest. Memory usage is bounded regardless of
// file size — F026.
//
// Plan-mode idempotency check is a two-stage walk:
//   - First compare Stat sizes (cheap, settles 99% of "obviously
//     different" cases).
//   - Only when sizes match, stream-sha256 both sides to confirm
//     content equality. Streaming hash keeps RAM bounded; the
//     historical os.ReadFile + bytes.Equal shape would have loaded
//     both into memory.
//
// Apply-mode performs the copy via os.Open + os.CreateTemp +
// io.Copy + os.Rename — atomic at the rename step, so a crashed
// daemon won't leave a half-written dest in place.
func (p *defaultPerformer) CopyFile(src, dest string, mode os.FileMode, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionCopyFile, Path: dest}

	srcInfo, err := os.Stat(src)
	if err != nil {
		e.Err = fmt.Errorf("stat source %s: %w", src, err)
		return e
	}
	if !srcInfo.Mode().IsRegular() {
		e.Err = fmt.Errorf("%s is not a regular file", src)
		return e
	}

	destInfo, derr := os.Stat(dest)
	switch {
	case derr == nil && destInfo.IsDir():
		e.Err = fmt.Errorf("%s exists and is a directory", dest)
		return e
	case derr == nil:
		if srcInfo.Size() == destInfo.Size() {
			srcHash, sherr := streamFileSHA256(src)
			if sherr == nil {
				destHash, dherr := streamFileSHA256(dest)
				if dherr == nil && srcHash == destHash && modeMatches(destInfo.Mode(), mode) {
					e.AlreadyOk = true
					e.Reason = "file content and mode already match"
					return e
				}
			}
		}
		e.Reason = fmt.Sprintf("content differs (%d -> %d bytes)", destInfo.Size(), srcInfo.Size())
	case os.IsNotExist(derr):
		e.Reason = fmt.Sprintf("would create file (%d bytes)", srcInfo.Size())
	default:
		e.Err = fmt.Errorf("stat dest %s: %w", dest, derr)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Create the parent dir when it is not already a directory we can see,
	// routing that through Mkdir so escalation applies: under as_user (or
	// become) a freshly created parent is made via privileged `mkdir -p` +
	// chown, so it is owned by as_user rather than the invoking user
	// (issue #95). The `statErr != nil` arm also covers a missing subdir
	// under a root-owned tree the invoker can't traverse. A stattable
	// existing dir is left untouched (never re-chmod'd / re-chown'd). With
	// no escalation bound, Mkdir falls through to a plain os.MkdirAll.
	if parent := filepath.Dir(dest); parent != "" {
		if info, statErr := os.Stat(parent); statErr != nil || !info.IsDir() {
			if me := p.Mkdir(parent, 0o755, opts); me.Err != nil {
				e.Err = fmt.Errorf("mkdir parent %s: %w", parent, me.Err)
				return e
			}
		}
	}

	tmpFile, err := streamSrcToTemp(src, filepath.Dir(dest))
	if err != nil {
		e.Err = err
		return e
	}
	defer func() { _ = os.Remove(tmpFile) }() // no-op after a successful rename

	// Mode-preservation quirk: WriteFile's apply path
	// (os.WriteFile in non-become mode) does NOT chmod an existing
	// file. CopyFile mirrors that for the default-mode case so
	// copy → reverse → restore round-trips keep the pre-apply mode
	// when the caller didn't explicitly request one. When the caller
	// did pass opts.ExplicitMode (e.g. file.copy: mode: '0755'),
	// honor it — silently preserving the dest's drifted mode would
	// contradict both the idempotency check above (which already
	// treats a mode mismatch as a reason to re-copy) and the
	// declarative intent of an explicit mode field.
	finalMode := mode
	if !opts.ExplicitMode && destInfo != nil && !destInfo.IsDir() {
		finalMode = destInfo.Mode().Perm()
	}

	if p.needSudoForOwnership() {
		// Stage the file under a path the unprivileged process can
		// write, then sudo mv into place + chmod (+ chown for named
		// users). Matches WriteFile's become path. Spec-72 follow-up:
		// gate on needSudoForOwnership so AsUser="" and AsUser=current
		// both skip the unnecessary sudo wrap.
		cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
			ShellQuote(tmpFile), ShellQuote(dest), formatMode(finalMode), ShellQuote(dest))
		cmd = p.withChown(cmd, dest)
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else {
		// #nosec G302 — mode is caller-controlled; this is a provisioning tool
		if err := os.Chmod(tmpFile, finalMode); err != nil {
			e.Err = fmt.Errorf("chmod temp file: %w", err)
			return e
		}
		if err := os.Rename(tmpFile, dest); err != nil {
			e.Err = fmt.Errorf("rename %s -> %s: %w", tmpFile, dest, err)
			return e
		}
	}
	e.Performed = true
	return e
}

// streamFileSHA256 returns the hex sha256 of the file at path, streamed
// in chunks so the file's bytes never live in memory all at once.
func streamFileSHA256(path string) (string, error) {
	// #nosec G304 — path comes from a Performer caller that has already
	// validated it; this helper is internal-package.
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// streamSrcToTemp creates a temp file in dir, copies src into it via
// io.Copy, and returns the temp file's path. The caller is responsible
// for removing the temp file (or for renaming it onto the final dest,
// which makes the Remove a no-op).
func streamSrcToTemp(src, dir string) (string, error) {
	// #nosec G304 — src path is mooncake-config-controlled
	srcF, err := os.Open(src)
	if err != nil {
		return "", fmt.Errorf("open source %s: %w", src, err)
	}
	defer func() { _ = srcF.Close() }()

	tmp, err := os.CreateTemp(dir, "mooncake-copy-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, srcF); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("stream copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return tmpPath, nil
}

// ----------------------------------------------------------------------
// Symlink / Hardlink
// ----------------------------------------------------------------------

func (p *defaultPerformer) Symlink(target, path string, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionSymlink, Path: path}

	info, err := os.Lstat(path)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		existing, readErr := os.Readlink(path)
		if readErr == nil && existing == target {
			e.AlreadyOk = true
			e.Reason = "symlink already points to " + target
			return e
		}
		e.Reason = fmt.Sprintf("symlink target differs (%s -> %s)", existing, target)
	case err == nil:
		if opts.Force {
			kind := describePathKind(info)
			e.Reason = fmt.Sprintf("would replace %s with symlink -> %s", kind, target)
			if p.modeFn() == actions.ModePlan {
				e.WouldChange = true
				return e
			}
			// apply mode: fall through to the remove-then-create block below
		} else {
			e.Reason = "path exists and is not a symlink"
			e.Err = fmt.Errorf("%s exists and is not a symlink (use force: true to replace)", path)
			return e
		}
	case os.IsNotExist(err):
		e.Reason = "would create symlink -> " + target
	default:
		e.Err = fmt.Errorf("lstat %s: %w", path, err)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	if _, statErr := os.Lstat(path); statErr == nil {
		if p.needSudoForOwnership() {
			if err := p.runSudo("rm -f " + ShellQuote(path)); err != nil {
				e.Err = err
				return e
			}
		} else if err := os.Remove(path); err != nil {
			e.Err = fmt.Errorf("remove existing %s: %w", path, err)
			return e
		}
	}

	if p.needSudoForOwnership() {
		// `chown -h` (no-dereference) is the symlink-aware form so the
		// link itself is chowned, not the target. Bundled into the same
		// sudo invocation when AsUser is a named non-root user.
		cmd := fmt.Sprintf("ln -s %s %s", ShellQuote(target), ShellQuote(path))
		if spec := p.chownSpec(); spec != "" {
			cmd += " && chown -h " + spec + " " + ShellQuote(path)
		}
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else if err := os.Symlink(target, path); err != nil {
		e.Err = fmt.Errorf("symlink %s -> %s: %w", path, target, err)
		return e
	}
	e.Performed = true
	return e
}

func (p *defaultPerformer) Hardlink(target, path string, _ actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionHardlink, Path: path}

	targetInfo, err := os.Stat(target)
	if err != nil {
		e.Err = fmt.Errorf("stat target %s: %w", target, err)
		return e
	}

	pathInfo, err := os.Lstat(path)
	switch {
	case err == nil && os.SameFile(targetInfo, pathInfo):
		e.AlreadyOk = true
		e.Reason = "hardlink already points to target"
		return e
	case err == nil:
		e.Reason = "path exists; would replace with hardlink"
	case os.IsNotExist(err):
		e.Reason = "would create hardlink -> " + target
	default:
		e.Err = fmt.Errorf("lstat %s: %w", path, err)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	if _, statErr := os.Lstat(path); statErr == nil {
		if p.needSudoForOwnership() {
			if err := p.runSudo("rm -f " + ShellQuote(path)); err != nil {
				e.Err = err
				return e
			}
		} else if err := os.Remove(path); err != nil {
			e.Err = fmt.Errorf("remove existing %s: %w", path, err)
			return e
		}
	}

	if p.needSudoForOwnership() {
		cmd := fmt.Sprintf("ln %s %s", ShellQuote(target), ShellQuote(path))
		cmd = p.withChown(cmd, path)
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else if err := os.Link(target, path); err != nil {
		e.Err = fmt.Errorf("hardlink %s -> %s: %w", path, target, err)
		return e
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// Touch
// ----------------------------------------------------------------------

func (p *defaultPerformer) Touch(path string, mode os.FileMode, _ actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionTouch, Path: path}

	_, statErr := os.Stat(path)
	switch {
	case statErr == nil:
		e.Reason = "would update mtime"
	case os.IsNotExist(statErr):
		e.Reason = "would create file"
	default:
		e.Err = fmt.Errorf("stat %s: %w", path, statErr)
		return e
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	if p.needSudoForOwnership() {
		cmd := fmt.Sprintf("touch %s && chmod %s %s",
			ShellQuote(path), formatMode(mode), ShellQuote(path))
		cmd = p.withChown(cmd, path)
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else {
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, mode)
		if err != nil {
			e.Err = fmt.Errorf("touch %s: %w", path, err)
			return e
		}
		_ = f.Close()
		now := time.Now()
		if err := os.Chtimes(path, now, now); err != nil {
			e.Err = fmt.Errorf("chtimes %s: %w", path, err)
			return e
		}
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// Remove
// ----------------------------------------------------------------------

func (p *defaultPerformer) Remove(path string, recursive bool, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionRemove, Path: path}

	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		e.AlreadyOk = true
		e.Reason = "already absent"
		return e
	case err != nil:
		e.Err = fmt.Errorf("lstat %s: %w", path, err)
		return e
	}

	switch {
	case info.IsDir() && !recursive:
		e.Reason = "would remove directory (non-recursive)"
	case info.IsDir():
		e.Reason = "would remove directory recursively"
	default:
		e.Reason = "would remove"
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Try direct first; fall back to sudo on EACCES when Become is set.
	var rmErr error
	if recursive {
		rmErr = os.RemoveAll(path)
	} else {
		rmErr = os.Remove(path)
	}
	var sudoCmd string
	switch {
	case info.IsDir() && recursive:
		sudoCmd = "rm -rf " + ShellQuote(path)
	case info.IsDir():
		sudoCmd = "rmdir " + ShellQuote(path)
	default:
		sudoCmd = "rm -f " + ShellQuote(path)
	}
	if err := p.becomeFallback(opts, rmErr, sudoCmd); err != nil {
		e.Err = fmt.Errorf("remove %s: %w", path, err)
		return e
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// Chmod
// ----------------------------------------------------------------------

func (p *defaultPerformer) Chmod(path string, mode os.FileMode, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionChmod, Path: path}

	info, err := os.Stat(path)
	switch {
	case os.IsNotExist(err):
		e.Err = fmt.Errorf("%s does not exist", path)
		return e
	case err != nil:
		e.Err = fmt.Errorf("stat %s: %w", path, err)
		return e
	case modeMatches(info.Mode(), mode):
		e.AlreadyOk = true
		e.Reason = "mode already " + mode.String()
		return e
	default:
		e.Reason = fmt.Sprintf("would chmod %s -> %s", info.Mode().Perm(), mode)
	}

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Try direct first; fall back to sudo on EACCES when Become is set.
	directErr := os.Chmod(path, mode)
	if err := p.becomeFallback(opts, directErr,
		fmt.Sprintf("chmod %s %s", formatMode(mode), ShellQuote(path)),
	); err != nil {
		e.Err = fmt.Errorf("chmod %s: %w", path, err)
		return e
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// Chown
// ----------------------------------------------------------------------

func (p *defaultPerformer) Chown(path, owner, group string, opts actions.PerformerOpts) actions.Effect {
	e := actions.Effect{Action: actions.ActionChown, Path: path}

	if owner == "" && group == "" {
		e.AlreadyOk = true
		e.Reason = "no owner or group specified"
		return e
	}

	uid, gid := -1, -1
	if owner != "" {
		resolved, err := lookupUID(owner)
		if err != nil {
			e.Err = err
			return e
		}
		uid = resolved
	}
	if group != "" {
		resolved, err := lookupGID(group)
		if err != nil {
			e.Err = err
			return e
		}
		gid = resolved
	}

	info, err := os.Stat(path)
	if err != nil {
		e.Err = fmt.Errorf("stat %s: %w", path, err)
		return e
	}
	curUID, curGID, ok := statOwner(info)
	if ok && (uid == -1 || curUID == uid) && (gid == -1 || curGID == gid) {
		e.AlreadyOk = true
		e.Reason = "ownership already matches"
		return e
	}
	e.Reason = fmt.Sprintf("would chown to %d:%d", uid, gid)

	if p.modeFn() == actions.ModePlan {
		e.WouldChange = true
		return e
	}

	// Try direct first; fall back to sudo on EACCES when Become is set.
	spec := ""
	switch {
	case owner != "" && group != "":
		spec = owner + ":" + group
	case owner != "":
		spec = owner
	case group != "":
		spec = ":" + group
	}
	directErr := os.Chown(path, uid, gid)
	if err := p.becomeFallback(opts, directErr,
		fmt.Sprintf("chown %s %s", spec, ShellQuote(path)),
	); err != nil {
		e.Err = fmt.Errorf("chown %s: %w", path, err)
		return e
	}
	e.Performed = true
	return e
}

// ----------------------------------------------------------------------
// Helpers
// ----------------------------------------------------------------------

// runSudo executes a shell command via sudo using the password supplied
// to NewPerformer. Errors include captured stderr.
//
// F005: implementation now delegates to security.BecomeRunner so the
// "become unsupported" and "no SudoPass configured" validation
// matches every other call site project-wide. Pre-fix the latter
// case (SudoPass="") was not caught — the runner happily wrote
// `"\n"` to sudo's stdin and let sudo hang on its password prompt.
func (p *defaultPerformer) runSudo(command string) error {
	runner := security.BecomeRunner{SudoPass: p.sudoPass, PasswordlessSudo: p.passwordlessSudo}
	cmd, err := runner.Command(true, "sh", "-c", command)
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo command failed: %w (stderr: %s)", err, stderr.String())
	}
	return nil
}

func formatMode(m os.FileMode) string { return fmt.Sprintf("%04o", m.Perm()) }

// describePathKind returns a human-readable noun for the file type of info.
func describePathKind(info os.FileInfo) string {
	m := info.Mode()
	switch {
	case m.IsDir():
		return "directory"
	case m&os.ModeSymlink != 0:
		return "symlink"
	case m&os.ModeDevice != 0:
		return "device"
	case m&os.ModeNamedPipe != 0:
		return "pipe"
	case m&os.ModeSocket != 0:
		return "socket"
	default:
		return "file"
	}
}

// ShellQuote single-quotes a string for safe POSIX-shell interpolation.
// Embedded single quotes are escaped via the standard `'\”` idiom.
// Exported so handlers reaching for `sudo sh -c <cmd>` can construct
// safe commands without re-implementing the quoting (F032).
//
// Go's `%q` verb is NOT a substitute — it escapes for Go-string syntax,
// not POSIX-shell syntax, and leaves $(...) / backtick substitution
// active inside double quotes.
func ShellQuote(s string) string {
	return "'" + replaceAll(s, "'", `'\''`) + "'"
}

func replaceAll(s, old, replacement string) string {
	out := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			out = append(out, replacement...)
			i += len(old)
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}

func lookupUID(name string) (int, error) {
	if n, err := strconv.Atoi(name); err == nil {
		return n, nil
	}
	u, err := user.Lookup(name)
	if err != nil {
		return 0, fmt.Errorf("lookup user %q: %w", name, err)
	}
	n, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, fmt.Errorf("parse uid for %q: %w", name, err)
	}
	return n, nil
}

func lookupGID(name string) (int, error) {
	if n, err := strconv.Atoi(name); err == nil {
		return n, nil
	}
	g, err := user.LookupGroup(name)
	if err != nil {
		return 0, fmt.Errorf("lookup group %q: %w", name, err)
	}
	n, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("parse gid for %q: %w", name, err)
	}
	return n, nil
}
