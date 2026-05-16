package agentd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// sanityCheckBinaryTimeout bounds the `<staged> --version` probe.
// mooncake --version exits in tens of milliseconds on every supported
// platform; 5 s is generous and lets the upgrade handler return
// binary_unhealthy promptly when the staged binary deadlocks (F027).
const sanityCheckBinaryTimeout = 5 * time.Second

// Self-upgrade endpoints. The controller pushes a new agentd binary that
// matches the daemon's GOOS/GOARCH via PUT, then asks for a replace via
// POST. Linux-only in v1 (see selfReplace_linux.go for the actual swap
// + re-exec; Windows lands as a follow-up because it needs the
// MoveFile + scheduled-task dance).
//
// Wire shape:
//
//   PUT  /v1/self/binary       body=<bytes>
//                              X-Mooncake-Binary-SHA256: <hex>
//                              X-Mooncake-Binary-OS:     linux|windows|darwin
//                              X-Mooncake-Binary-Arch:   amd64|arm64
//        → 202 {"staged_path": "...", "sha256": "..."}
//
//   POST /v1/self/replace      body={"staged_path","sha256","force":bool}
//        → 202 {"old_pid":N,"old_version":"...","new_version":"..."}
//        (response sent BEFORE the actual exec; client must poll
//         /v1/version for the PID-changed signal)
//
// See docs/working/design/fleet-upgrade.md for the full design.

// maxBinaryBytes caps PUT body size. Mooncake static-linked binaries are
// ~25 MiB today; 64 MiB leaves headroom for debug/symbol-rich builds.
const maxBinaryBytes int64 = 64 << 20

// upgradeMu serialises self-upgrade activity. One stage-or-replace in
// flight per daemon — concurrent uploads would race on the staged file
// path and on the binary swap.
var upgradeMu sync.Mutex

type stageResponse struct {
	StagedPath string `json:"staged_path"`
	SHA256     string `json:"sha256"`
}

type replaceRequest struct {
	StagedPath string `json:"staged_path"`
	SHA256     string `json:"sha256"`
	Force      bool   `json:"force,omitempty"`
}

type replaceResponse struct {
	OldPID     int    `json:"old_pid"`
	OldVersion string `json:"old_version"`
	NewVersion string `json:"new_version"`
}

func (s *Server) selfBinaryHandler(w http.ResponseWriter, r *http.Request) {
	if !upgradeMu.TryLock() {
		writeError(w, http.StatusConflict, "upgrade_in_progress",
			"another upgrade is already staging or replacing on this daemon")
		return
	}
	defer upgradeMu.Unlock()

	wantSHA := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Mooncake-Binary-SHA256")))
	wantOS := strings.TrimSpace(r.Header.Get("X-Mooncake-Binary-OS"))
	wantArch := strings.TrimSpace(r.Header.Get("X-Mooncake-Binary-Arch"))

	if wantSHA == "" || len(wantSHA) != 64 {
		writeError(w, http.StatusBadRequest, "missing_sha",
			"X-Mooncake-Binary-SHA256 header is required (hex-encoded sha256, 64 chars)")
		return
	}
	if wantOS != runtime.GOOS {
		writeError(w, http.StatusBadRequest, "os_mismatch",
			fmt.Sprintf("binary OS %q does not match daemon OS %q", wantOS, runtime.GOOS))
		return
	}
	if wantArch != runtime.GOARCH {
		writeError(w, http.StatusBadRequest, "arch_mismatch",
			fmt.Sprintf("binary Arch %q does not match daemon Arch %q", wantArch, runtime.GOARCH))
		return
	}

	upgradeDir := filepath.Join(s.cfg.StateDir, "upgrade")
	if err := os.MkdirAll(upgradeDir, 0o700); err != nil {
		writeError(w, http.StatusInternalServerError, "create_upgrade_dir", err.Error())
		return
	}

	// Stream to a temp file under upgrade/, hashing as we go. Rename to the
	// final name only after the SHA matches the header — keeps a partial /
	// tampered upload from ever leaving a half-staged binary behind.
	tmpPath := filepath.Join(upgradeDir, fmt.Sprintf("staging-%d.tmp", os.Getpid()))
	tmp, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open_tmp", err.Error())
		return
	}
	hasher := sha256.New()
	limited := http.MaxBytesReader(w, r.Body, maxBinaryBytes)
	if _, err := io.Copy(io.MultiWriter(tmp, hasher), limited); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "close_tmp", err.Error())
		return
	}

	gotSHA := hex.EncodeToString(hasher.Sum(nil))
	if gotSHA != wantSHA {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusBadRequest, "sha_mismatch",
			fmt.Sprintf("body sha256 %s does not match header %s", gotSHA, wantSHA))
		return
	}

	stagedPath := filepath.Join(upgradeDir, "staged-"+gotSHA[:8]+stagedSuffix())
	// #nosec G302 -- staged binary must be executable (0755)
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "chmod_tmp", err.Error())
		return
	}
	if err := os.Rename(tmpPath, stagedPath); err != nil {
		_ = os.Remove(tmpPath)
		writeError(w, http.StatusInternalServerError, "rename_staged", err.Error())
		return
	}

	// Sanity check: the staged binary must successfully report its version.
	// Catches obviously-broken artefacts (corrupt download, wrong arch that
	// somehow slipped past header check) before they get swapped in.
	if err := sanityCheckBinary(stagedPath); err != nil {
		_ = os.Remove(stagedPath)
		writeError(w, http.StatusBadRequest, "binary_unhealthy",
			fmt.Sprintf("staged binary failed --version: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusAccepted, stageResponse{
		StagedPath: stagedPath,
		SHA256:     gotSHA,
	})
}

func (s *Server) selfReplaceHandler(w http.ResponseWriter, r *http.Request) {
	if !upgradeMu.TryLock() {
		writeError(w, http.StatusConflict, "upgrade_in_progress",
			"another upgrade is already staging or replacing on this daemon")
		return
	}
	defer upgradeMu.Unlock()

	var req replaceRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<14))
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	upgradeDir := filepath.Join(s.cfg.StateDir, "upgrade")
	// Force staged_path to live under upgradeDir — refuses any traversal
	// or attempt to swap in an unrelated file.
	cleanStaged := filepath.Clean(req.StagedPath)
	relStaged, err := filepath.Rel(upgradeDir, cleanStaged)
	if err != nil || strings.HasPrefix(relStaged, "..") || strings.ContainsRune(relStaged, os.PathSeparator) {
		writeError(w, http.StatusBadRequest, "bad_staged_path",
			fmt.Sprintf("staged_path must be directly under %s", upgradeDir))
		return
	}

	wantSHA := strings.ToLower(strings.TrimSpace(req.SHA256))
	if len(wantSHA) != 64 {
		writeError(w, http.StatusBadRequest, "missing_sha",
			"sha256 (hex, 64 chars) is required")
		return
	}
	gotSHA, err := fileSHA256(cleanStaged)
	if err != nil {
		writeError(w, http.StatusNotFound, "staged_not_found", err.Error())
		return
	}
	if gotSHA != wantSHA {
		writeError(w, http.StatusBadRequest, "sha_mismatch",
			fmt.Sprintf("staged sha256 %s does not match request %s", gotSHA, wantSHA))
		return
	}

	_, running := s.worker.Stats()
	if running > 0 && !req.Force {
		writeError(w, http.StatusConflict, "runs_in_flight",
			fmt.Sprintf("%d run(s) in flight; pass force=true to upgrade anyway", running))
		return
	}

	currentPath, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "locate_self", err.Error())
		return
	}

	// Snapshot the running binary so a human can restore it manually if
	// the new one fails to come back up. Tagged with a timestamp so
	// repeated upgrades don't clobber each other.
	previousPath := filepath.Join(upgradeDir, fmt.Sprintf("previous-%d%s", time.Now().Unix(), stagedSuffix()))
	if err := copyFile(currentPath, previousPath); err != nil {
		writeError(w, http.StatusInternalServerError, "snapshot_current", err.Error())
		return
	}

	// Swap is OS-specific. Linux: rename + later syscall.Exec; Windows
	// (when implemented): MoveFile-the-running-exe trick + scheduled
	// task restart.
	if err := swapBinary(cleanStaged, currentPath); err != nil {
		// Try best-effort to restore the snapshot so we don't leave the
		// daemon's binary in a partially-replaced state.
		_ = os.Remove(currentPath)
		_ = os.Rename(previousPath, currentPath)
		writeError(w, http.StatusInternalServerError, "swap_binary", err.Error())
		return
	}

	// Reply BEFORE the restart so the controller observes a clean 202.
	// The actual handoff runs in a goroutine that gives the response a
	// second to flush over the wire.
	writeJSON(w, http.StatusAccepted, replaceResponse{
		OldPID:     os.Getpid(),
		OldVersion: s.version,
		// NewVersion is what the new binary will report; we can't know
		// without exec'ing it, so the controller learns this by polling
		// /v1/version after restart.
		NewVersion: "",
	})

	go func() {
		time.Sleep(1 * time.Second)
		if err := reExec(currentPath); err != nil {
			s.log.Error("self-upgrade re-exec failed", "err", err, "binary", currentPath)
		}
	}()
}

// sanityCheckBinary runs `<staged> --version` under
// sanityCheckBinaryTimeout. Returns nil iff the process exits 0 and the
// output mentions "mooncake". The output is otherwise discarded — we
// only care that the binary can start; matching the expected version
// is the controller's job (via post-restart probe). The timeout is
// load-bearing: without it a staged binary that deadlocks during init
// would hang the upgrade handler indefinitely and block every
// subsequent /v1/self/* request via upgradeMu (F027).
func sanityCheckBinary(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), sanityCheckBinaryTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	cmd.Stdin = nil
	out, err := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("staged binary timed out on --version after %s", sanityCheckBinaryTimeout)
	}
	if err != nil {
		return fmt.Errorf("%w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(strings.ToLower(string(out)), "mooncake") {
		return fmt.Errorf("output %q does not look like mooncake --version", strings.TrimSpace(string(out)))
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()                                                        //nolint:errcheck
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755) // #nosec G302 -- binary must be executable
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// stagedSuffix returns the executable suffix for the host OS so paths
// inside the upgrade dir are correct on Windows. Linux/macOS get an
// empty string.
func stagedSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
