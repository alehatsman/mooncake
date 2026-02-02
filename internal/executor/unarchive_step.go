package executor

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

// ArchiveFormat represents the type of archive being extracted.
type ArchiveFormat int

const (
	ArchiveUnknown ArchiveFormat = iota
	ArchiveTar
	ArchiveTarGz
	ArchiveZip
)

// String returns the string representation of the archive format.
func (f ArchiveFormat) String() string {
	switch f {
	case ArchiveTar:
		return "tar"
	case ArchiveTarGz:
		return "tar.gz"
	case ArchiveZip:
		return "zip"
	default:
		return "unknown"
	}
}

// ExtractionStats tracks statistics from archive extraction.
type ExtractionStats struct {
	FilesExtracted int
	DirsCreated    int
	BytesExtracted int64
}

// HandleUnarchive extracts archive files with path traversal protection.
func HandleUnarchive(step config.Step, ec *ExecutionContext) error {
	unarchiveAction := step.Unarchive

	// Validate required fields
	if unarchiveAction.Src == "" {
		return &StepValidationError{Field: "src", Message: "required for unarchive"}
	}
	if unarchiveAction.Dest == "" {
		return &StepValidationError{Field: "dest", Message: "required for unarchive"}
	}

	// Render paths with template variables
	renderedSrc, err := ec.PathUtil.ExpandPath(unarchiveAction.Src, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "src path", Cause: err}
	}

	renderedDest, err := ec.PathUtil.ExpandPath(unarchiveAction.Dest, ec.CurrentDir, ec.Variables)
	if err != nil {
		return &RenderError{Field: "dest path", Cause: err}
	}

	// Render creates path if specified
	var renderedCreates string
	if unarchiveAction.Creates != "" {
		renderedCreates, err = ec.PathUtil.ExpandPath(unarchiveAction.Creates, ec.CurrentDir, ec.Variables)
		if err != nil {
			return &RenderError{Field: "creates path", Cause: err}
		}
	}

	// Create result object
	result := NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Check idempotency - skip if creates path exists
	if renderedCreates != "" {
		if _, err := os.Stat(renderedCreates); err == nil {
			ec.Logger.Debugf("  Skipping extraction: creates path exists: %s", renderedCreates)
			return nil
		}
	}

	// Verify source exists and is a file
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		return &FileOperationError{Operation: "read", Path: renderedSrc, Cause: err}
	}
	if srcInfo.IsDir() {
		return &StepValidationError{Field: "src", Message: "must be a file, not a directory"}
	}

	// Detect archive format
	format := detectArchiveFormat(renderedSrc)
	if format == ArchiveUnknown {
		return &StepValidationError{Field: "src", Message: fmt.Sprintf("unsupported archive format: %s", renderedSrc)}
	}

	// Handle dry-run mode
	if ec.HandleDryRun(func(dryRun *dryRunLogger) {
		dryRun.LogArchiveExtraction(renderedSrc, renderedDest, format.String(), unarchiveAction.StripComponents)
		dryRun.LogRegister(step)
	}) {
		// Emit dry-run event
		ec.EmitEvent(events.EventArchiveExtracted, events.ArchiveExtractedData{
			Src:             renderedSrc,
			Dest:            renderedDest,
			Format:          format.String(),
			StripComponents: unarchiveAction.StripComponents,
			DryRun:          true,
		})
		return nil
	}

	// Ensure destination directory exists
	mode := parseFileMode(unarchiveAction.Mode, defaultDirMode)
	ec.Logger.Debugf("  Ensuring destination directory: %s", renderedDest)
	if err := os.MkdirAll(renderedDest, mode); err != nil {
		markStepFailed(result, step, ec)
		return &FileOperationError{Operation: "create", Path: renderedDest, Cause: err}
	}

	// Extract archive based on format
	ec.Logger.Debugf("  Extracting %s archive: %s -> %s", format.String(), renderedSrc, renderedDest)
	var stats *ExtractionStats
	switch format {
	case ArchiveTar:
		stats, err = extractTarArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode)
	case ArchiveTarGz:
		stats, err = extractTarGzArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode)
	case ArchiveZip:
		stats, err = extractZipArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode)
	default:
		return &StepValidationError{Field: "src", Message: "unsupported archive format"}
	}

	if err != nil {
		markStepFailed(result, step, ec)
		return err
	}

	result.Changed = stats.FilesExtracted > 0 || stats.DirsCreated > 0

	// Emit archive.extracted event
	durationMs := result.Duration.Milliseconds()
	ec.EmitEvent(events.EventArchiveExtracted, events.ArchiveExtractedData{
		Src:             renderedSrc,
		Dest:            renderedDest,
		Format:          format.String(),
		FilesExtracted:  stats.FilesExtracted,
		DirsCreated:     stats.DirsCreated,
		BytesExtracted:  stats.BytesExtracted,
		StripComponents: unarchiveAction.StripComponents,
		DurationMs:      durationMs,
		DryRun:          ec.DryRun,
	})

	// Register the result if register is specified
	if step.Register != "" {
		result.RegisterTo(ec.Variables, step.Register)
		ec.Logger.Debugf("  Registered result as: %s (changed=%v)", step.Register, result.Changed)
	}

	// Set result in context for event emission
	ec.CurrentResult = result

	ec.Logger.Debugf("  Extracted %d files, %d directories (%d bytes)", stats.FilesExtracted, stats.DirsCreated, stats.BytesExtracted)

	return nil
}

// detectArchiveFormat detects the archive format from file extension.
func detectArchiveFormat(path string) ArchiveFormat {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		return ArchiveTarGz
	}
	if strings.HasSuffix(lower, ".tar") {
		return ArchiveTar
	}
	if strings.HasSuffix(lower, ".zip") {
		return ArchiveZip
	}
	return ArchiveUnknown
}

// stripPathComponents strips N leading path components from a path.
// Returns the stripped path and whether it should be extracted.
// If all components are stripped, returns "", false.
func stripPathComponents(path string, count int) (string, bool) {
	if count <= 0 {
		return path, true
	}

	// Normalize to forward slashes
	normalized := filepath.ToSlash(path)
	parts := strings.Split(normalized, "/")

	// If we're stripping more components than exist, skip this entry
	if count >= len(parts) {
		return "", false
	}

	// Strip the leading components
	stripped := strings.Join(parts[count:], "/")
	if stripped == "" {
		return "", false
	}

	return stripped, true
}

// extractTarArchive extracts a tar archive to the destination directory.
func extractTarArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode) (*ExtractionStats, error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(srcPath)
	if err != nil {
		return nil, &FileOperationError{Operation: "read", Path: srcPath, Cause: err}
	}
	defer file.Close()

	return extractTar(file, destDir, stripComponents, dirMode)
}

// extractTarGzArchive extracts a gzipped tar archive to the destination directory.
func extractTarGzArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode) (*ExtractionStats, error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(srcPath)
	if err != nil {
		return nil, &FileOperationError{Operation: "read", Path: srcPath, Cause: err}
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, &FileOperationError{Operation: "decompress", Path: srcPath, Cause: err}
	}
	defer gzReader.Close()

	return extractTar(gzReader, destDir, stripComponents, dirMode)
}

// extractTar extracts a tar archive from a reader to the destination directory.
func extractTar(reader io.Reader, destDir string, stripComponents int, dirMode os.FileMode) (*ExtractionStats, error) {
	stats := &ExtractionStats{}
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, &FileOperationError{Operation: "read", Path: "tar archive", Cause: err}
		}

		// Strip leading path components
		extractPath, shouldExtract := stripPathComponents(header.Name, stripComponents)
		if !shouldExtract {
			continue
		}

		// Validate no path traversal
		if err := pathutil.ValidateNoPathTraversal(extractPath); err != nil {
			return stats, fmt.Errorf("path traversal in %q: %w", header.Name, err)
		}

		// Use SafeJoin for final path
		targetPath, err := pathutil.SafeJoin(destDir, extractPath)
		if err != nil {
			return stats, fmt.Errorf("invalid path %q: %w", header.Name, err)
		}

		// Handle different file types
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, dirMode); err != nil {
				return stats, &FileOperationError{Operation: "create", Path: targetPath, Cause: err}
			}
			stats.DirsCreated++

		case tar.TypeReg:
			// Ensure parent directory exists
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, dirMode); err != nil {
				return stats, &FileOperationError{Operation: "create", Path: parentDir, Cause: err}
			}

			// Extract file
			if err := extractTarFile(tarReader, targetPath, header.Mode); err != nil {
				return stats, err
			}
			stats.FilesExtracted++
			stats.BytesExtracted += header.Size

		case tar.TypeSymlink:
			// Validate symlink target doesn't escape destination
			linkTarget := header.Linkname
			if err := pathutil.ValidateNoPathTraversal(linkTarget); err != nil {
				return stats, fmt.Errorf("symlink target traversal in %q -> %q: %w", header.Name, linkTarget, err)
			}

			// Ensure parent directory exists
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, dirMode); err != nil {
				return stats, &FileOperationError{Operation: "create", Path: parentDir, Cause: err}
			}

			// Create symlink
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return stats, &FileOperationError{Operation: "symlink", Path: targetPath, Cause: err}
			}
			stats.FilesExtracted++
		}
	}

	return stats, nil
}

// extractTarFile extracts a single file from a tar reader to the target path.
func extractTarFile(reader io.Reader, targetPath string, mode int64) error {
	// Create file with permissions
	fileMode := os.FileMode(mode) & os.ModePerm
	if fileMode == 0 {
		fileMode = defaultFileMode
	}

	// #nosec G304 -- File path from user config is intentional functionality
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return &FileOperationError{Operation: "create", Path: targetPath, Cause: err}
	}
	defer outFile.Close()

	// Copy contents
	if _, err := io.Copy(outFile, reader); err != nil {
		return &FileOperationError{Operation: "write", Path: targetPath, Cause: err}
	}

	return nil
}

// extractZipArchive extracts a zip archive to the destination directory.
func extractZipArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode) (*ExtractionStats, error) {
	stats := &ExtractionStats{}

	// Open zip file
	zipReader, err := zip.OpenReader(srcPath)
	if err != nil {
		return stats, &FileOperationError{Operation: "read", Path: srcPath, Cause: err}
	}
	defer zipReader.Close()

	for _, file := range zipReader.File {
		// Strip leading path components
		extractPath, shouldExtract := stripPathComponents(file.Name, stripComponents)
		if !shouldExtract {
			continue
		}

		// Validate no path traversal
		if err := pathutil.ValidateNoPathTraversal(extractPath); err != nil {
			return stats, fmt.Errorf("path traversal in %q: %w", file.Name, err)
		}

		// Use SafeJoin for final path
		targetPath, err := pathutil.SafeJoin(destDir, extractPath)
		if err != nil {
			return stats, fmt.Errorf("invalid path %q: %w", file.Name, err)
		}

		// Check if it's a directory
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, dirMode); err != nil {
				return stats, &FileOperationError{Operation: "create", Path: targetPath, Cause: err}
			}
			stats.DirsCreated++
			continue
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, dirMode); err != nil {
			return stats, &FileOperationError{Operation: "create", Path: parentDir, Cause: err}
		}

		// Extract file
		if err := extractZipFile(file, targetPath); err != nil {
			return stats, err
		}
		stats.FilesExtracted++
		stats.BytesExtracted += int64(file.UncompressedSize64)
	}

	return stats, nil
}

// extractZipFile extracts a single file from a zip archive to the target path.
func extractZipFile(file *zip.File, targetPath string) error {
	// Open file in archive
	srcFile, err := file.Open()
	if err != nil {
		return &FileOperationError{Operation: "read", Path: file.Name, Cause: err}
	}
	defer srcFile.Close()

	// Create destination file
	fileMode := file.Mode() & os.ModePerm
	if fileMode == 0 {
		fileMode = defaultFileMode
	}

	// #nosec G304 -- File path from user config is intentional functionality
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return &FileOperationError{Operation: "create", Path: targetPath, Cause: err}
	}
	defer outFile.Close()

	// Copy contents
	if _, err := io.Copy(outFile, srcFile); err != nil {
		return &FileOperationError{Operation: "write", Path: targetPath, Cause: err}
	}

	return nil
}
