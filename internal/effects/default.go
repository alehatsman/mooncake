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
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	modeFn   ModeFunc
	sudoPass string
}

// NewPerformer constructs an actions.Performer that performs real
// filesystem operations in ModeApply and inspects state in ModePlan.
// modeFn is called once per primitive to decide the path; sudoPass is
// consulted when opts.Become is true.
func NewPerformer(modeFn ModeFunc, sudoPass string) actions.Performer {
	return &defaultPerformer{modeFn: modeFn, sudoPass: sudoPass}
}

func (p *defaultPerformer) Mode() actions.Mode { return p.modeFn() }

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

	if opts.Become {
		cmd := fmt.Sprintf("mkdir -p -m %s %s && chmod %s %s",
			formatMode(mode), shellQuote(path), formatMode(mode), shellQuote(path))
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else {
		if err := os.MkdirAll(path, mode); err != nil {
			e.Err = fmt.Errorf("mkdir %s: %w", path, err)
			return e
		}
		if err := os.Chmod(path, mode); err != nil {
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

	if parent := filepath.Dir(path); parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			e.Err = fmt.Errorf("mkdir parent %s: %w", parent, err)
			return e
		}
	}

	if opts.Become {
		tmp, err := os.CreateTemp("", "mooncake-effect-*")
		if err != nil {
			e.Err = fmt.Errorf("create temp file: %w", err)
			return e
		}
		tmpPath := tmp.Name()
		_ = tmp.Close()
		defer func() { _ = os.Remove(tmpPath) }()
		if err := os.WriteFile(tmpPath, content, 0o600); err != nil {
			e.Err = fmt.Errorf("write temp file: %w", err)
			return e
		}
		cmd := fmt.Sprintf("mv %s %s && chmod %s %s",
			shellQuote(tmpPath), shellQuote(path), formatMode(mode), shellQuote(path))
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else {
		// #nosec G306 — mode is caller-controlled; this is a provisioning tool
		if err := os.WriteFile(path, content, mode); err != nil {
			e.Err = fmt.Errorf("write %s: %w", path, err)
			return e
		}
	}
	e.Performed = true
	return e
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
		if opts.Become {
			if err := p.runSudo("rm -f " + shellQuote(path)); err != nil {
				e.Err = err
				return e
			}
		} else if err := os.Remove(path); err != nil {
			e.Err = fmt.Errorf("remove existing %s: %w", path, err)
			return e
		}
	}

	if opts.Become {
		cmd := fmt.Sprintf("ln -s %s %s", shellQuote(target), shellQuote(path))
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

func (p *defaultPerformer) Hardlink(target, path string, opts actions.PerformerOpts) actions.Effect {
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
		if opts.Become {
			if err := p.runSudo("rm -f " + shellQuote(path)); err != nil {
				e.Err = err
				return e
			}
		} else if err := os.Remove(path); err != nil {
			e.Err = fmt.Errorf("remove existing %s: %w", path, err)
			return e
		}
	}

	if opts.Become {
		cmd := fmt.Sprintf("ln %s %s", shellQuote(target), shellQuote(path))
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

func (p *defaultPerformer) Touch(path string, mode os.FileMode, opts actions.PerformerOpts) actions.Effect {
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

	if opts.Become {
		cmd := fmt.Sprintf("touch %s && chmod %s %s",
			shellQuote(path), formatMode(mode), shellQuote(path))
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

	if opts.Become {
		var cmd string
		switch {
		case info.IsDir() && recursive:
			cmd = "rm -rf " + shellQuote(path)
		case info.IsDir():
			cmd = "rmdir " + shellQuote(path)
		default:
			cmd = "rm -f " + shellQuote(path)
		}
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else {
		var rmErr error
		if recursive {
			rmErr = os.RemoveAll(path)
		} else {
			rmErr = os.Remove(path)
		}
		if rmErr != nil {
			e.Err = fmt.Errorf("remove %s: %w", path, rmErr)
			return e
		}
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

	if opts.Become {
		cmd := fmt.Sprintf("chmod %s %s", formatMode(mode), shellQuote(path))
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else if err := os.Chmod(path, mode); err != nil {
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

	if opts.Become {
		spec := ""
		switch {
		case owner != "" && group != "":
			spec = owner + ":" + group
		case owner != "":
			spec = owner
		case group != "":
			spec = ":" + group
		}
		cmd := fmt.Sprintf("chown %s %s", spec, shellQuote(path))
		if err := p.runSudo(cmd); err != nil {
			e.Err = err
			return e
		}
	} else if err := os.Chown(path, uid, gid); err != nil {
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
func (p *defaultPerformer) runSudo(command string) error {
	if !security.IsBecomeSupported() {
		return errors.New("become not supported on this platform")
	}
	// #nosec G204 — provisioning tool intentionally runs caller-provided commands
	cmd := exec.Command("sudo", "-S", "sh", "-c", command)
	cmd.Stdin = bytes.NewBufferString(p.sudoPass + "\n")
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

// shellQuote is the unexported alias retained for in-package callers.
// New code should prefer ShellQuote at the call site.
func shellQuote(s string) string { return ShellQuote(s) }

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
