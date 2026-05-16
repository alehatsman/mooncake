package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/lockfile"
)

// Outcome is what InstallURL reports back to the action handler.
type Outcome struct {
	Changed     bool
	InstallDir  string
	ResolvedURL string
	Checksum    string // normalized "sha256:..." form
	Reason      string // human-friendly description (logged + plan mode)
}

// InstallURL handles the download → verify → extract flow used by
// URL-based backends (archive-url, github-release). Idempotency: if the
// install dir is non-empty, returns Changed=false without touching the
// network.
//
// lock may be nil. When non-nil, an existing entry's sha256 is treated
// as the source of truth (lockfile checksum enforcement) and a new entry
// is recorded after a successful install. The handler is responsible
// for calling lock.Save afterwards.
func InstallURL(ctx context.Context, spec Spec, plan Plan, facts FactSnapshot, lock *lockfile.Lock) (Outcome, error) {
	installDir, err := InstallDir(spec.Name, spec.Version)
	if err != nil {
		return Outcome{}, err
	}

	// Idempotency: populated install dir means we're done.
	populated, err := installDirIsPopulated(installDir)
	if err != nil {
		return Outcome{}, fmt.Errorf("stat install dir: %w", err)
	}
	if populated {
		return Outcome{
			Changed:    false,
			InstallDir: installDir,
			Reason:     fmt.Sprintf("already installed at %s", installDir),
		}, nil
	}

	// Decide expected checksum source: lock entry > inline > TOFU.
	archKey := archKey(facts)
	expected := ""
	if lock != nil {
		if e, ok := lock.Lookup(spec.Backend, spec.Name, spec.Version, archKey); ok {
			expected = e.SHA256
			// Cross-backend safety: a lock entry under a different backend
			// for the same (name, version) is a hard error; the handler
			// checks for that before calling us.
		}
	}
	if expected == "" {
		expected = plan.Checksum
	}

	// Download.
	root, err := StoreRoot()
	if err != nil {
		return Outcome{}, err
	}
	if mkErr := os.MkdirAll(root, 0o755); mkErr != nil {
		return Outcome{}, fmt.Errorf("create store root: %w", mkErr)
	}
	tmpFile, err := fetchToTempFile(ctx, plan.URL, root)
	if err != nil {
		return Outcome{}, err
	}
	defer func() { _ = os.Remove(tmpFile) }()

	// Verify or compute checksum.
	got, err := hashFileSHA256(tmpFile)
	if err != nil {
		return Outcome{}, err
	}
	if expected != "" && !checksumsEqual(expected, got) {
		return Outcome{}, fmt.Errorf("checksum mismatch for %s: expected %s, got %s", plan.URL, expected, got)
	}
	finalChecksum := got // always record the computed one (canonical form)

	// Install into <installDir>.tmp then rename atomically. Detect whether
	// the downloaded artifact is an archive (.tar.gz/.tgz/.tar/.zip) or a
	// bare binary (no recognized extension) — many GitHub releases ship
	// the latter (jq, hadolint, kind, kubectl, k9s, gh, mc, etc.) and the
	// archive-only path used to error out with "unsupported archive
	// format" before the binary could be installed at all.
	tmpDir := installDir + ".tmp"
	_ = os.RemoveAll(tmpDir) // clean any prior crashed extract
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return Outcome{}, fmt.Errorf("create tmp install dir: %w", err)
	}
	if detectFormat(tmpFile) == formatUnknown {
		if err := installBareBinary(tmpFile, tmpDir, spec, plan); err != nil {
			_ = os.RemoveAll(tmpDir)
			return Outcome{}, fmt.Errorf("install bare binary: %w", err)
		}
	} else {
		if err := extractArchive(tmpFile, tmpDir, plan.StripComponents); err != nil {
			_ = os.RemoveAll(tmpDir)
			return Outcome{}, fmt.Errorf("extract: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(installDir), 0o755); err != nil {
		_ = os.RemoveAll(tmpDir)
		return Outcome{}, fmt.Errorf("create install parent dir: %w", err)
	}
	if err := os.Rename(tmpDir, installDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		return Outcome{}, fmt.Errorf("rename install dir: %w", err)
	}

	// Record lock entry (caller persists).
	if lock != nil {
		lock.Set(lockfile.Entry{
			Backend:      spec.Backend,
			Name:         spec.Name,
			Version:      spec.Version,
			ResolvedURL:  plan.URL,
			SHA256:       finalChecksum,
			Bin:          spec.Bin,
			LockedAt:     time.Now().UTC().Format(time.RFC3339),
			LockedByArch: archKey,
		})
	}

	return Outcome{
		Changed:     true,
		InstallDir:  installDir,
		ResolvedURL: plan.URL,
		Checksum:    finalChecksum,
		Reason:      fmt.Sprintf("installed %s %s from %s", spec.Name, spec.Version, plan.URL),
	}, nil
}

// archKey is the lockfile's locked_by_arch column. Empty for backends
// that don't bind by arch (mise); "os-arch" for URL-based backends.
func archKey(facts FactSnapshot) string {
	if facts.OS == "" && facts.Arch == "" {
		return ""
	}
	return facts.OS + "-" + facts.Arch
}
