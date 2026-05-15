package agentd

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// File-sync endpoint. The controller PUTs each file of its plan tree under
// `<state_dir>/synced/<scope>/<relative-path>`; agentd accepts the body
// after validating that the resolved path stays inside the synced root and
// that no traversal or symlink trickery is attempted.
//
// Wire shape:
//
//	PUT  /v1/files?scope=<s>&path=<p>   X-Sha256 optional
//	HEAD /v1/files?scope=<s>&path=<p>&sha256=<hex>
//
// See spec-43 for the full contract.

// Limits and validation rules.
const (
	maxScopeBytes    = 128
	maxRelPathBytes  = 1024
	maxScopeSegBytes = 64
)

// validScopeChar reports whether c is allowed in a scope segment.
// Segments are joined by '/' to produce the scope; each segment matches
// `[A-Za-z0-9_-]+`. Two segments at most (e.g. "<controller_id>/<dir_hash>").
func validScopeChar(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z':
		return true
	case c >= 'A' && c <= 'Z':
		return true
	case c >= '0' && c <= '9':
		return true
	case c == '_' || c == '-':
		return true
	}
	return false
}

func validateScope(scope string) error {
	if scope == "" {
		return errors.New("scope is empty")
	}
	if len(scope) > maxScopeBytes {
		return fmt.Errorf("scope exceeds %d bytes", maxScopeBytes)
	}
	segs := strings.Split(scope, "/")
	if len(segs) > 2 {
		return errors.New("scope has more than one '/' separator")
	}
	for _, seg := range segs {
		if seg == "" {
			return errors.New("scope segment is empty")
		}
		if len(seg) > maxScopeSegBytes {
			return fmt.Errorf("scope segment exceeds %d bytes", maxScopeSegBytes)
		}
		for i := 0; i < len(seg); i++ {
			if !validScopeChar(seg[i]) {
				return fmt.Errorf("scope contains invalid char %q", seg[i])
			}
		}
	}
	return nil
}

// resolveSyncPath validates rel against the synced root for the given scope
// and returns the absolute on-disk path. It rejects path traversal, absolute
// paths, overlong paths, and paths whose existing prefix passes through a
// symlink.
//
// The returned path is safe to write to (or read for HEAD) without further
// validation. Callers still need to MkdirAll the parent directory.
func resolveSyncPath(syncedRoot, scope, rel string) (string, error) {
	if err := validateScope(scope); err != nil {
		return "", err
	}
	if rel == "" {
		return "", errors.New("path is empty")
	}
	if len(rel) > maxRelPathBytes {
		return "", fmt.Errorf("path exceeds %d bytes", maxRelPathBytes)
	}
	if filepath.IsAbs(rel) {
		return "", errors.New("path must be relative")
	}
	// Reject null bytes pre-Clean — Clean preserves them and the OS would
	// truncate at the null, defeating our prefix check.
	if strings.ContainsRune(rel, 0) {
		return "", errors.New("path contains null byte")
	}

	cleaned := filepath.Clean(rel)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", errors.New("path escapes synced root")
	}

	scopeRoot := filepath.Join(syncedRoot, scope)
	full := filepath.Join(scopeRoot, cleaned)

	// Belt-and-braces: even after Clean we verify the result sits under
	// scopeRoot. Use string prefix with separator suffix to avoid the
	// `/syncedfoo` vs `/synced` confusion.
	rootWithSep := scopeRoot + string(filepath.Separator)
	if full != scopeRoot && !strings.HasPrefix(full, rootWithSep) {
		return "", errors.New("path escapes synced root after clean")
	}

	// Walk every existing path component from syncedRoot to `full` and
	// reject if any is a symlink. Stops at the first non-existent component
	// — deeper checks are pointless and will be created cleanly.
	if err := ensureNoSymlinkComponents(syncedRoot, full); err != nil {
		return "", err
	}

	return full, nil
}

func ensureNoSymlinkComponents(syncedRoot, full string) error {
	rel, err := filepath.Rel(syncedRoot, full)
	if err != nil {
		return fmt.Errorf("compute rel: %w", err)
	}
	cur := syncedRoot
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == "" || seg == "." {
			continue
		}
		cur = filepath.Join(cur, seg)
		info, err := os.Lstat(cur)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			return fmt.Errorf("lstat %s: %w", cur, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in path: %s", cur)
		}
	}
	return nil
}

// putFileHandler handles PUT /v1/files. The body is streamed to a temp file
// while sha256 is computed in parallel; on success the temp is fsynced and
// atomically renamed into place. The full request body is bounded by
// cfg.MaxSyncBytes via http.MaxBytesReader.
func (s *Server) putFileHandler(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	rel := r.URL.Query().Get("path")
	if scope == "" || rel == "" {
		writeError(w, http.StatusBadRequest, "missing_params", "scope and path are required")
		return
	}

	full, err := resolveSyncPath(s.cfg.SyncedRoot(), scope, rel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_path", err.Error())
		return
	}

	expectedSha := strings.TrimSpace(r.Header.Get("X-Sha256"))
	if expectedSha != "" {
		// Lowercase the user-supplied hex for byte-comparison consistency
		// with hex.EncodeToString below.
		expectedSha = strings.ToLower(expectedSha)
		if !isHex(expectedSha) || len(expectedSha) != sha256.Size*2 {
			writeError(w, http.StatusBadRequest, "invalid_sha256", "X-Sha256 must be 64 hex chars")
			return
		}
	}

	// Ensure the parent directory exists. MkdirAll is a no-op if already
	// present. Mode 0700 — sync content is per-user state.
	parent := filepath.Dir(full)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		s.log.Error("files: mkdir parent", "path", parent, "err", err)
		writeError(w, http.StatusInternalServerError, "mkdir_failed", err.Error())
		return
	}

	tmp, err := os.CreateTemp(parent, filepath.Base(full)+".tmp.*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tempfile_failed", err.Error())
		return
	}
	// Guarantee cleanup of the temp on any non-rename exit.
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		writeError(w, http.StatusInternalServerError, "chmod_failed", err.Error())
		return
	}

	body := http.MaxBytesReader(w, r.Body, s.cfg.MaxSyncBytes)
	defer func() { _ = body.Close() }()

	var hasher hash.Hash
	var writer io.Writer = tmp
	if expectedSha != "" {
		hasher = sha256.New()
		writer = io.MultiWriter(tmp, hasher)
	}

	if _, err := io.Copy(writer, body); err != nil {
		var mbErr *http.MaxBytesError
		if errors.As(err, &mbErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				fmt.Sprintf("body exceeds %d bytes", s.cfg.MaxSyncBytes))
			return
		}
		s.log.Error("files: copy body", "path", full, "err", err)
		writeError(w, http.StatusInternalServerError, "write_failed", err.Error())
		return
	}

	if hasher != nil {
		gotSha := hex.EncodeToString(hasher.Sum(nil))
		if gotSha != expectedSha {
			writeError(w, http.StatusUnprocessableEntity, "sha256_mismatch",
				fmt.Sprintf("expected %s, got %s", expectedSha, gotSha))
			return
		}
	}

	if err := tmp.Sync(); err != nil {
		s.log.Warn("files: fsync", "path", tmpPath, "err", err)
		// Continue anyway — fsync failure is logged but not fatal for the
		// sync use case (the controller can re-PUT if data is lost).
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "close_failed", err.Error())
		return
	}
	if err := os.Rename(tmpPath, full); err != nil {
		writeError(w, http.StatusInternalServerError, "rename_failed", err.Error())
		return
	}
	committed = true
	w.WriteHeader(http.StatusNoContent)
}

// headFileHandler handles HEAD /v1/files. Returns 200 only when the file
// exists at the resolved path AND its sha256 matches the query parameter.
// Otherwise 404. Body is always empty.
func (s *Server) headFileHandler(w http.ResponseWriter, r *http.Request) {
	scope := r.URL.Query().Get("scope")
	rel := r.URL.Query().Get("path")
	sha := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("sha256")))
	if scope == "" || rel == "" || sha == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !isHex(sha) || len(sha) != sha256.Size*2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	full, err := resolveSyncPath(s.cfg.SyncedRoot(), scope, rel)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	f, err := os.Open(full)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if hex.EncodeToString(h.Sum(nil)) != sha {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func isHex(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}
