package unarchive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/alehatsman/mooncake/internal/pathutil"
)

// archiveMatchesDest reports whether every entry in srcPath already
// exists at destDir with matching shape. Used by Execute as a pre-
// extract idempotency gate (MT-46).
//
// Match semantics per entry:
//   - directory: a directory exists at the target path (mode ignored).
//   - regular file: a file exists with matching size in bytes.
//   - symlink: a symlink exists with matching target.
//
// Returns false on the first mismatch — does not enumerate every
// divergence. A read error or missing source surfaces as an error so
// the caller can fall through to the extract path (the extractor's
// error reporting is richer).
func (h *Handler) archiveMatchesDest(format ArchiveFormat, srcPath, destDir string, stripComponents int) (bool, error) {
	switch format {
	case ArchiveTar:
		f, err := os.Open(srcPath) // #nosec G304 -- caller-validated path
		if err != nil {
			return false, fmt.Errorf("open tar: %w", err)
		}
		defer f.Close() //nolint:errcheck
		return h.tarMatchesDest(f, destDir, stripComponents)
	case ArchiveTarGz:
		f, err := os.Open(srcPath) // #nosec G304 -- caller-validated path
		if err != nil {
			return false, fmt.Errorf("open tar.gz: %w", err)
		}
		defer f.Close() //nolint:errcheck
		gz, err := gzip.NewReader(f)
		if err != nil {
			return false, fmt.Errorf("gunzip: %w", err)
		}
		defer gz.Close() //nolint:errcheck
		return h.tarMatchesDest(gz, destDir, stripComponents)
	case ArchiveZip:
		zr, err := zip.OpenReader(srcPath)
		if err != nil {
			return false, fmt.Errorf("open zip: %w", err)
		}
		defer zr.Close() //nolint:errcheck
		return h.zipMatchesDest(zr, destDir, stripComponents)
	}
	return false, fmt.Errorf("unsupported archive format")
}

func (h *Handler) tarMatchesDest(reader io.Reader, destDir string, stripComponents int) (bool, error) {
	tr := tar.NewReader(reader)
	entries := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			// Empty archive (no in-scope entries) is not idempotent-eligible:
			// preserve the existing contract that extraction always emits an
			// event, even if it copies zero bytes.
			return entries > 0, nil
		}
		if err != nil {
			return false, fmt.Errorf("tar read: %w", err)
		}

		extractPath, shouldExtract := h.stripPathComponents(header.Name, stripComponents)
		if !shouldExtract {
			continue
		}
		entries++
		if err := pathutil.ValidateNoPathTraversal(extractPath); err != nil {
			return false, err
		}
		targetPath, err := pathutil.SafeJoin(destDir, extractPath)
		if err != nil {
			return false, err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			info, statErr := os.Stat(targetPath)
			if statErr != nil || !info.IsDir() {
				return false, nil
			}
		case tar.TypeReg:
			info, statErr := os.Lstat(targetPath)
			if statErr != nil {
				return false, nil
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return false, nil
			}
			if info.Size() != header.Size {
				return false, nil
			}
		case tar.TypeSymlink:
			info, statErr := os.Lstat(targetPath)
			if statErr != nil {
				return false, nil
			}
			if info.Mode()&os.ModeSymlink == 0 {
				return false, nil
			}
			existing, readErr := os.Readlink(targetPath)
			if readErr != nil || existing != header.Linkname {
				return false, nil
			}
		default:
			// Unknown entry types are conservative: force re-extract.
			return false, nil
		}
	}
}

func (h *Handler) zipMatchesDest(zr *zip.ReadCloser, destDir string, stripComponents int) (bool, error) {
	entries := 0
	for _, file := range zr.File {
		extractPath, shouldExtract := h.stripPathComponents(file.Name, stripComponents)
		if !shouldExtract {
			continue
		}
		entries++
		if err := pathutil.ValidateNoPathTraversal(extractPath); err != nil {
			return false, err
		}
		targetPath, err := pathutil.SafeJoin(destDir, extractPath)
		if err != nil {
			return false, err
		}

		if file.FileInfo().IsDir() {
			info, statErr := os.Stat(targetPath)
			if statErr != nil || !info.IsDir() {
				return false, nil
			}
			continue
		}

		info, statErr := os.Lstat(targetPath)
		if statErr != nil {
			return false, nil
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
		// #nosec G115 -- zip uncompressed size fits in int64 for our scale
		if info.Size() != int64(file.UncompressedSize64) {
			return false, nil
		}
	}
	return entries > 0, nil
}
