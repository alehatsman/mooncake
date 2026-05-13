// Package lockfile reads and writes mooncake.lock: a YAML record of tools
// installed by the `tool` action, used to enforce checksum reproducibility
// across machines and over time. Lock entries are keyed by
// (backend, name, version, arch); the URL-based backends record the
// resolved URL and SHA256, the mise backend records only the version.
//
// Concurrency: Save() takes an OS-level file lock (flock) on the lockfile
// itself so concurrent `mooncake apply` invocations against the same file
// cannot corrupt it.
package lockfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Filename is the conventional lockfile name. Lives next to the applied
// config (or in the daemon workdir for inline plans — Spec 18).
const Filename = "mooncake.lock"

// Entry records a single (backend, name, version, arch) tool install.
type Entry struct {
	Backend      string `yaml:"backend"`
	Name         string `yaml:"name"`
	Version      string `yaml:"version"`
	ResolvedURL  string `yaml:"resolved_url,omitempty"`   // empty for mise
	SHA256       string `yaml:"sha256,omitempty"`         // empty for mise
	Bin          string `yaml:"bin,omitempty"`            // bin path relative to install dir (URL backends); empty for mise
	LockedAt     string `yaml:"locked_at"`                // RFC3339
	LockedByArch string `yaml:"locked_by_arch,omitempty"` // "linux-amd64"; empty for mise
}

// Lock is the in-memory representation of the lockfile.
type Lock struct {
	mu      sync.Mutex
	Entries []Entry `yaml:"tool"`
}

// Find walks up from startDir looking for an existing mooncake.lock.
// Returns the absolute path of the nearest lockfile, or "" if none is
// found. Stops at the filesystem root. I/O errors during stat are
// ignored (treated as "not found at this level").
//
// Used by read-only callers (`mooncake tool which`, `tool list`,
// `tool env`). Write callers should use their own resolution that
// falls back to startDir + Filename when none exists.
func Find(startDir string) string {
	dir := startDir
	for {
		candidate := filepath.Join(dir, Filename)
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

// Load reads the lockfile at path. A missing file is not an error; it
// returns an empty Lock. Other I/O errors are surfaced.
func Load(path string) (*Lock, error) {
	l := &Lock{}
	// #nosec G304 -- Lockfile path is derived from the applied config dir
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return l, nil
		}
		return nil, fmt.Errorf("read lockfile %s: %w", path, err)
	}
	if len(data) == 0 {
		return l, nil
	}
	if err := yaml.Unmarshal(data, l); err != nil {
		return nil, fmt.Errorf("parse lockfile %s: %w", path, err)
	}
	return l, nil
}

// Lookup returns the entry for the given (backend, name, version, arch)
// tuple, if any. arch may be empty for backends that don't bind by arch
// (e.g. mise).
func (l *Lock) Lookup(backend, name, version, arch string) (Entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.Entries {
		if e.Backend == backend && e.Name == name && e.Version == version && e.LockedByArch == arch {
			return e, true
		}
	}
	return Entry{}, false
}

// LookupByName returns the first entry matching (name, version), regardless
// of backend or arch. Used by `mooncake tool which` and friends. The lockfile
// binds the choice of backend on first install, so by-name lookups are
// unambiguous for the same (name, version).
func (l *Lock) LookupByName(name, version string) (Entry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.Entries {
		if e.Name == name && e.Version == version {
			return e, true
		}
	}
	return Entry{}, false
}

// Set adds or replaces an entry matching (backend, name, version, arch).
func (l *Lock) Set(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for i, existing := range l.Entries {
		if existing.Backend == e.Backend && existing.Name == e.Name && existing.Version == e.Version && existing.LockedByArch == e.LockedByArch {
			l.Entries[i] = e
			return
		}
	}
	l.Entries = append(l.Entries, e)
}

// Save atomically writes the lockfile to path. Takes an OS-level file
// lock on the file during the read-modify-write window so concurrent
// callers serialize.
func (l *Lock) Save(path string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create lockfile parent dir: %w", err)
	}

	// Acquire flock on the lockfile itself. Use the target path; create
	// it if missing so flock has something to grab.
	fl, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lockfile for flock: %w", err)
	}
	defer func() { _ = fl.Close() }()

	if lockErr := lockExclusive(fl); lockErr != nil {
		return fmt.Errorf("flock lockfile: %w", lockErr)
	}
	defer func() { _ = unlock(fl) }()

	data, err := yaml.Marshal(l)
	if err != nil {
		return fmt.Errorf("marshal lockfile: %w", err)
	}

	// Atomic write: temp + rename in the same dir.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mooncake.lock.tmp.*")
	if err != nil {
		return fmt.Errorf("create temp lockfile: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp lockfile: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp lockfile: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename temp lockfile: %w", err)
	}
	return nil
}
