package fleet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// FileEntry is one file inside a plan-dir. The walker fills AbsPath, RelPath,
// and Size; Sha256 is computed lazily by ComputeSha256 (or by SyncTo when it
// needs to issue a HEAD).
type FileEntry struct {
	AbsPath string // absolute path on the controller
	RelPath string // path relative to plan-dir, forward-slash separated
	Size    int64
	Sha256  string // lowercase hex; empty until ComputeSha256 runs
}

// SyncStats summarizes one SyncTo call.
type SyncStats struct {
	Total      int   // total files considered
	Put        int   // files actually uploaded
	Skipped    int   // files skipped via HEAD-hit
	BytesTotal int64 // sum of all file sizes
	BytesPut   int64 // sum of sizes of files actually uploaded
}

// Walk enumerates files under planDir, returning entries plus the
// cumulative size. Refuses to descend into pathological trees:
//
//   - any symlink anywhere in the tree → error (v1 limitation)
//   - cumulative size > maxBytes → error
//
// Top-level `.git` and `.DS_Store` are skipped. Everything else — including
// dotfiles, nested `.git` dirs (don't put one in your plan tree), and
// `presets/`-style subdirectories — is included verbatim.
func Walk(planDir string, maxBytes int64) ([]FileEntry, int64, error) {
	planDir = filepath.Clean(planDir)
	info, err := os.Lstat(planDir)
	if err != nil {
		return nil, 0, fmt.Errorf("stat plan-dir: %w", err)
	}
	if !info.IsDir() {
		return nil, 0, fmt.Errorf("plan-dir %s is not a directory", planDir)
	}

	var entries []FileEntry
	var total int64
	skipTopLevel := map[string]bool{".git": true, ".DS_Store": true}

	err = filepath.WalkDir(planDir, func(p string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if p == planDir {
			return nil
		}
		// Top-level skips.
		if filepath.Dir(p) == planDir && skipTopLevel[d.Name()] {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Lstat to detect symlinks before they get followed.
		li, err := os.Lstat(p)
		if err != nil {
			return fmt.Errorf("lstat %s: %w", p, err)
		}
		if li.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not supported in plan-dir (v1): %s", p)
		}
		if d.IsDir() {
			return nil
		}
		if !li.Mode().IsRegular() {
			// Device files, sockets, FIFOs are never plan content. The
			// common hit is `fleet apply /tmp/x.yml` choking on
			// /tmp/.X11-unix/X0 (a socket). Skip silently so the
			// canonical "throw-away file in /tmp" flow works. Issue #15.
			return nil
		}

		rel, err := filepath.Rel(planDir, p)
		if err != nil {
			return fmt.Errorf("compute rel: %w", err)
		}
		total += li.Size()
		if maxBytes > 0 && total > maxBytes {
			return fmt.Errorf("plan-dir exceeds --max-sync-size (%d bytes); abort", maxBytes)
		}
		entries = append(entries, FileEntry{
			AbsPath: p,
			RelPath: filepath.ToSlash(rel),
			Size:    li.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if len(entries) == 0 {
		return nil, 0, fmt.Errorf("plan-dir %s contains no files", planDir)
	}
	return entries, total, nil
}

// ComputeSha256 fills e.Sha256 with the lowercase hex of the file's SHA-256.
// No-op if already populated.
func (e *FileEntry) ComputeSha256() error {
	if e.Sha256 != "" {
		return nil
	}
	f, err := os.Open(e.AbsPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", e.AbsPath, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash %s: %w", e.AbsPath, err)
	}
	e.Sha256 = hex.EncodeToString(h.Sum(nil))
	return nil
}

// SyncTo uploads files to peer under the given scope, HEAD-skipping anything
// the daemon already has byte-identically. Entries are mutated in place to
// populate Sha256.
func SyncTo(ctx context.Context, peer *transport.Client, entries []FileEntry, scope string) (SyncStats, error) {
	stats := SyncStats{Total: len(entries)}
	for i := range entries {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		e := &entries[i]
		stats.BytesTotal += e.Size

		if err := e.ComputeSha256(); err != nil {
			return stats, err
		}

		hit, err := peer.Head(ctx, scope, e.RelPath, e.Sha256)
		if err != nil {
			return stats, fmt.Errorf("HEAD %s: %w", e.RelPath, err)
		}
		if hit {
			stats.Skipped++
			continue
		}
		if err := peer.Put(ctx, scope, e.RelPath, e.AbsPath, e.Sha256); err != nil {
			return stats, fmt.Errorf("PUT %s: %w", e.RelPath, err)
		}
		stats.Put++
		stats.BytesPut += e.Size
	}
	return stats, nil
}

// ScopeFor builds a deterministic sync scope key for (controller, plan-dir).
// Same controller + same plan-dir-abspath → same scope on every peer, so
// reruns reuse the same on-disk tree and HEAD-skip works.
//
// Shape: `<controller_id>/<sha256(planDir)[:16]>`. Total length: 36 + 1 +
// 16 = 53 chars, well under the daemon's 128-byte scope cap.
func ScopeFor(controllerID, planDir string) (string, error) {
	if controllerID == "" {
		return "", errors.New("ScopeFor: controllerID is empty")
	}
	if planDir == "" {
		return "", errors.New("ScopeFor: planDir is empty")
	}
	abs, err := filepath.Abs(planDir)
	if err != nil {
		return "", fmt.Errorf("abs plan-dir: %w", err)
	}
	abs = filepath.Clean(abs)
	sum := sha256.Sum256([]byte(abs))
	return controllerID + "/" + hex.EncodeToString(sum[:])[:16], nil
}

// PeerPath builds the absolute on-disk path of a synced file as seen by the
// daemon. Uses forward-slash join (`path.Join`, not `filepath.Join`) because
// the wire shape is always POSIX — the daemon's SyncedRoot is always
// forward-slash-separated regardless of where the controller runs.
func PeerPath(syncedRoot, scope, rel string) string {
	rel = strings.TrimPrefix(rel, "/")
	return path.Join(syncedRoot, scope, rel)
}
