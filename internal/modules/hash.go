package modules

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// HashPrefix labels the hash algorithm. Same shape and same algorithm as Go's
// dirhash "h1:" so the format is familiar rather than novel: the summary is one
// "<sha256-hex>  <relpath>\n" line per regular file, sorted by path, and the
// hash is base64(sha256(summary)).
const HashPrefix = "h1:"

// gitDir is excluded from the hash. A shallow clone's .git contains pack files
// and refs that differ between clones of the same tag, so including it would
// make the hash unstable for no gain — the working tree IS the module.
const gitDir = ".git"

// hashCache memoizes HashDir by directory. A module cache dir is immutable for
// the life of a run (Fetcher treats a populated dir as a hit and never rewrites
// it), so a run that resolves the same module from twenty `use:` steps hashes
// it once. Deliberately never invalidated: a process that outlives a cache
// mutation is `mooncake mod tidy` writing what it just fetched, which reads the
// hash after the fetch, not before.
var (
	hashCacheMu sync.Mutex
	hashCache   = map[string]string{}
)

// HashDir returns the h1: content hash of the module tree rooted at dir.
//
// Covered: every regular file, by slash-separated path relative to dir.
// Excluded: the .git directory, and every non-regular entry (symlink, device,
// socket, fifo) — see the spec's Non-goals. File modes are not covered.
func HashDir(dir string) (string, error) {
	hashCacheMu.Lock()
	if h, ok := hashCache[dir]; ok {
		hashCacheMu.Unlock()
		return h, nil
	}
	hashCacheMu.Unlock()

	h, err := hashDirUncached(dir)
	if err != nil {
		return "", err
	}

	hashCacheMu.Lock()
	hashCache[dir] = h
	hashCacheMu.Unlock()
	return h, nil
}

func hashDirUncached(dir string) (string, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("hash module dir %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("hash module dir %s: not a directory", dir)
	}

	type entry struct{ name, sum string }
	var entries []entry

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == gitDir {
				return fs.SkipDir
			}
			return nil
		}
		// Regular files only. A symlink's Type() is ModeSymlink, not zero.
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		entries = append(entries, entry{name: filepath.ToSlash(rel), sum: sum})
		return nil
	})
	if walkErr != nil {
		return "", fmt.Errorf("hash module dir %s: %w", dir, walkErr)
	}

	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	var summary strings.Builder
	for _, e := range entries {
		// Two spaces between sum and name, matching sha256sum / Go dirhash.
		fmt.Fprintf(&summary, "%s  %s\n", e.sum, e.name)
	}
	digest := sha256.Sum256([]byte(summary.String()))
	return HashPrefix + base64.StdEncoding.EncodeToString(digest[:]), nil
}

// hashFile returns the lowercase hex sha256 of one file's contents, streamed so
// a large asset in a module doesn't have to fit in memory.
func hashFile(path string) (string, error) {
	f, err := os.Open(path) // #nosec G304 -- path comes from a WalkDir of the module cache
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
