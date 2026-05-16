// Package archive extracts tool downloads (tar.gz / tar / zip) into a
// destination directory, honoring `strip_components` and rejecting
// entries that would escape via path traversal.
//
// Lifted out of internal/actions/tool to keep that package under the
// 1500-LOC handler soft cap (CLAUDE.md §1). The contract is small —
// one exported entry point, Extract — so the seam stays narrow.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// format is the detected kind of a downloaded archive.
type format int

const (
	formatUnknown format = iota
	formatTarGz
	formatTar
	formatZip
)

func detectFormat(path string) format {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return formatTarGz
	case strings.HasSuffix(lower, ".tar"):
		return formatTar
	case strings.HasSuffix(lower, ".zip"):
		return formatZip
	}
	return formatUnknown
}

// IsArchive reports whether path looks like a recognised archive
// (`.tar.gz` / `.tgz` / `.tar` / `.zip`). Callers use this to choose
// between Extract and a bare-binary install path — many GitHub
// releases ship the latter (jq, hadolint, kubectl, gh, …).
func IsArchive(path string) bool {
	return detectFormat(path) != formatUnknown
}

// Extract decompresses srcPath into destDir, honoring stripComponents
// (top-level directories to skip). destDir must already exist. The
// archive format is detected from the srcPath extension.
func Extract(srcPath, destDir string, stripComponents int) error {
	switch detectFormat(srcPath) {
	case formatTarGz:
		return extractTarGz(srcPath, destDir, stripComponents)
	case formatTar:
		return extractTar(srcPath, destDir, stripComponents)
	case formatZip:
		return extractZip(srcPath, destDir, stripComponents)
	}
	return fmt.Errorf("unsupported archive format for %s (supported: .tar.gz, .tgz, .tar, .zip)", srcPath)
}

func extractTarGz(srcPath, destDir string, strip int) error {
	// #nosec G304 -- srcPath is a mooncake-managed temp file
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return extractTarStream(gz, destDir, strip)
}

func extractTar(srcPath, destDir string, strip int) error {
	// #nosec G304 -- srcPath is a mooncake-managed temp file
	f, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()
	return extractTarStream(f, destDir, strip)
}

func extractTarStream(r io.Reader, destDir string, strip int) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("tar read: %w", err)
		}
		rel, ok := stripPath(hdr.Name, strip)
		if !ok {
			continue
		}
		target, err := safeJoin(destDir, rel)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			// #nosec G115 -- tar mode bits fit in os.FileMode for any sane archive
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)&0o777|0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", target, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			// #nosec G115 -- tar mode bits fit in os.FileMode for any sane archive
			mode := os.FileMode(hdr.Mode) & 0o777
			if mode == 0 {
				mode = 0o644
			}
			if err := writeRegular(target, tr, mode); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("mkdir parent %s: %w", target, err)
			}
			_ = os.Remove(target) // best-effort overwrite
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return fmt.Errorf("symlink %s -> %s: %w", target, hdr.Linkname, err)
			}
		default:
			// Skip hardlinks, devices, etc. — uncommon in tool archives.
		}
	}
}

func extractZip(srcPath, destDir string, strip int) error {
	zr, err := zip.OpenReader(srcPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer func() { _ = zr.Close() }()

	for _, zf := range zr.File {
		rel, ok := stripPath(zf.Name, strip)
		if !ok {
			continue
		}
		target, err := safeJoin(destDir, rel)
		if err != nil {
			return err
		}
		if zf.FileInfo().IsDir() {
			if mkErr := os.MkdirAll(target, 0o755); mkErr != nil {
				return fmt.Errorf("mkdir %s: %w", target, mkErr)
			}
			continue
		}
		if mkErr := os.MkdirAll(filepath.Dir(target), 0o755); mkErr != nil {
			return fmt.Errorf("mkdir parent %s: %w", target, mkErr)
		}
		mode := zf.Mode() & 0o777
		if mode == 0 {
			mode = 0o644
		}
		rc, err := zf.Open()
		if err != nil {
			return fmt.Errorf("open zip entry %s: %w", zf.Name, err)
		}
		err = writeRegular(target, rc, mode)
		_ = rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// writeRegular writes r to target with mode, truncating any existing file.
func writeRegular(target string, r io.Reader, mode os.FileMode) error {
	// #nosec G304 -- target is inside the mooncake-managed install dir
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create %s: %w", target, err)
	}
	if _, err := io.Copy(out, r); err != nil {
		_ = out.Close()
		return fmt.Errorf("write %s: %w", target, err)
	}
	return out.Close()
}

// stripPath drops the first `strip` slash-separated components from name.
// Returns ("", false) if the resulting path is empty or escapes the root.
func stripPath(name string, strip int) (string, bool) {
	// Normalize path separators (zip uses forward slashes; tar usually does too).
	clean := filepath.ToSlash(filepath.Clean(name))
	clean = strings.TrimPrefix(clean, "/")
	if clean == "" || clean == "." {
		return "", false
	}
	if strip == 0 {
		return clean, true
	}
	parts := strings.SplitN(clean, "/", strip+1)
	if len(parts) <= strip {
		return "", false
	}
	return parts[strip], true
}

// safeJoin returns destDir/rel, rejecting any rel that would escape
// destDir via .. traversal.
func safeJoin(destDir, rel string) (string, error) {
	abs := filepath.Join(destDir, rel)
	clean := filepath.Clean(abs)
	rootClean := filepath.Clean(destDir) + string(os.PathSeparator)
	if !strings.HasPrefix(clean+string(os.PathSeparator), rootClean) && clean != filepath.Clean(destDir) {
		return "", fmt.Errorf("archive entry escapes destination: %s", rel)
	}
	return clean, nil
}
