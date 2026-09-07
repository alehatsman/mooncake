package modules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// LockFilename is the module lockfile's name.
//
// Deliberately NOT "mooncake.lock": that name is already owned by the `tool`
// action (internal/lockfile) for pinning tool installs. Two unrelated lockfiles
// under one filename would clobber each other on save, silently, in whichever
// order the run happened to write them.
const LockFilename = "mooncake.modules.lock"

// LockVersion is the on-disk schema version. Bumped only for a breaking change
// to the file's shape; a reader that meets a higher version refuses rather than
// guessing.
const LockVersion = 1

// LockEntry pins one module reference to the content hash of the tree that
// reference resolved to.
type LockEntry struct {
	// Ref is "<host>/<owner>/<repo>@<version>" — the cache-dir identity, with
	// any subpath stripped. Several subpath references share one entry.
	Ref string `yaml:"ref"`
	// Hash is the h1: tree hash; see HashDir.
	Hash string `yaml:"hash"`
	// LockedAt is RFC3339, for the human reading a diff. Not verified.
	LockedAt string `yaml:"locked_at,omitempty"`
}

// Lock is the in-memory form of mooncake.modules.lock.
type Lock struct {
	mu      sync.Mutex
	Version int         `yaml:"version"`
	Modules []LockEntry `yaml:"modules"`

	// err, when set, makes every verification fail with it. Set by BrokenLock
	// so a lockfile that exists but cannot be read reaches the operator as a
	// failed run instead of a silently unverified one.
	err error
}

// BrokenLock returns a Lock that fails every verification with err. Used when
// a lockfile is present on disk but unreadable: "present but broken" must not
// collapse into "absent", which would run unverified.
func BrokenLock(err error) *Lock {
	return &Lock{Version: LockVersion, err: err}
}

// LockKey returns the lockfile key for a reference: the cache-dir identity,
// with any subpath dropped. One cached repo, one hash, one entry.
func LockKey(ref Reference) string {
	return fmt.Sprintf("%s/%s/%s@%s", ref.Host, ref.Owner, ref.Repo, ref.Version)
}

// FindLock walks up from startDir looking for a module lockfile. Returns the
// absolute path of the nearest one, or "" if none exists before the filesystem
// root. Stat errors are treated as "not here", same as internal/lockfile.Find.
func FindLock(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, LockFilename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// LoadLock reads the lockfile at path. A missing file is not an error — it
// returns an empty Lock, which verifies nothing. Other I/O and parse errors
// are surfaced: a corrupt lockfile must not degrade into "no verification".
func LoadLock(path string) (*Lock, error) {
	l := &Lock{Version: LockVersion}
	data, err := os.ReadFile(path) // #nosec G304 -- path is derived from the playbook dir
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) == 0 {
		return l, nil
	}
	var onDisk struct {
		Version int         `yaml:"version"`
		Modules []LockEntry `yaml:"modules"`
	}
	if err := yaml.Unmarshal(data, &onDisk); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if onDisk.Version > LockVersion {
		return nil, fmt.Errorf(
			"%s: lock version %d is newer than this mooncake understands (%d); upgrade mooncake",
			path, onDisk.Version, LockVersion)
	}
	l.Version = LockVersion
	l.Modules = onDisk.Modules
	return l, nil
}

// Empty reports whether the lock pins nothing. An empty lock verifies nothing,
// which is how a playbook with no lockfile keeps its pre-lockfile behavior.
func (l *Lock) Empty() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err == nil && len(l.Modules) == 0
}

// LookupLock returns the entry for a ref key, if pinned.
func (l *Lock) LookupLock(key string) (LockEntry, bool) {
	if l == nil {
		return LockEntry{}, false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.Modules {
		if e.Ref == key {
			return e, true
		}
	}
	return LockEntry{}, false
}

// Set adds or replaces the entry for e.Ref.
func (l *Lock) Set(e LockEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, existing := range l.Modules {
		if existing.Ref == e.Ref {
			l.Modules[i] = e
			return
		}
	}
	l.Modules = append(l.Modules, e)
}

// Keep drops every entry whose ref is not in keys. This is what makes `mod
// tidy` a tidy: a module removed from the playbook leaves the lockfile too.
func (l *Lock) Keep(keys map[string]bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	kept := make([]LockEntry, 0, len(l.Modules))
	for _, e := range l.Modules {
		if keys[e.Ref] {
			kept = append(kept, e)
		}
	}
	l.Modules = kept
}

// Refs returns the pinned ref keys, sorted.
func (l *Lock) Refs() []string {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.Modules))
	for _, e := range l.Modules {
		out = append(out, e.Ref)
	}
	sort.Strings(out)
	return out
}

func defaultNowRFC3339() string { return time.Now().UTC().Format(time.RFC3339) }

// NowRFC3339 is the clock used for LockedAt. A var so tests can freeze it.
var NowRFC3339 = defaultNowRFC3339

// SaveLock writes the lock to path: entries sorted by ref so the file is
// byte-deterministic and a diff is reviewable, written to a temp file and
// renamed so a crash mid-write can't leave a half-lockfile behind.
func (l *Lock) SaveLock(path string) error {
	l.mu.Lock()
	entries := make([]LockEntry, len(l.Modules))
	copy(entries, l.Modules)
	l.mu.Unlock()

	sort.Slice(entries, func(i, j int) bool { return entries[i].Ref < entries[j].Ref })

	out, err := yaml.Marshal(struct {
		Version int         `yaml:"version"`
		Modules []LockEntry `yaml:"modules"`
	}{Version: LockVersion, Modules: entries})
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+LockFilename+".tmp.*")
	if err != nil {
		return fmt.Errorf("create temp lockfile: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds

	if _, err := tmp.Write(out); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp lockfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp lockfile: %w", err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp lockfile: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp lockfile into place: %w", err)
	}
	return nil
}

// VerifyDir checks the tree at dir against the pin for key.
//
// Returns nil when the lock is empty (nothing is pinned, so nothing is
// claimed). Otherwise a ref absent from a non-empty lockfile is an error: a
// lockfile that pins some modules but not the one about to run is a lockfile
// that has drifted from the playbook, and silently trusting the unpinned one
// is exactly the hole the lockfile exists to close.
func (l *Lock) VerifyDir(key, dir string) error {
	if l != nil {
		l.mu.Lock()
		lockErr := l.err
		l.mu.Unlock()
		if lockErr != nil {
			return lockErr
		}
	}
	if l.Empty() {
		return nil
	}
	want, ok := l.LookupLock(key)
	if !ok {
		return fmt.Errorf(
			"module %s is not in %s; run `mooncake mod tidy` to pin it",
			key, LockFilename)
	}
	got, err := HashDir(dir)
	if err != nil {
		return err
	}
	if got != want.Hash {
		return fmt.Errorf(
			"module %s failed verification:\n  locked:  %s\n  on disk: %s\n"+
				"The tag moved, the cache was tampered with, or the lockfile is stale.\n"+
				"Re-fetch with `mooncake mod cache clean`, or re-pin with `mooncake mod tidy`.",
			key, want.Hash, got)
	}
	return nil
}
