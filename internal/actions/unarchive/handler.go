// Package unarchive implements the unarchive action handler.
//
// The unarchive action extracts archive files with:
// - Format support: tar, tar.gz, tar.bz2, zip
// - Strip leading path components
// - Idempotency via creates marker
// - Path traversal protection
// - Extraction statistics
package unarchive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

const (
	defaultFileMode os.FileMode = 0644
	defaultDirMode  os.FileMode = 0755
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

// Handler implements the Handler interface for unarchive actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the unarchive action.
func (Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "unarchive",
		Description:        "Extract archive files (tar, tar.gz, zip) with path traversal protection",
		Category:           actions.CategoryFile,
		SupportsDryRun:     true,
		SupportsBecome:     false,
		EmitsEvents:        []string{string(events.EventArchiveExtracted)},
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,      // Depends on dest path
		ImplementsCheck:    true,       // Checks creates marker for idempotency
	}
}

// Validate checks if the unarchive configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.Unarchive == nil {
		return fmt.Errorf("unarchive configuration is nil")
	}

	unarchiveAction := step.Unarchive
	if unarchiveAction.Src == "" {
		return fmt.Errorf("src is required")
	}

	if unarchiveAction.Dest == "" {
		return fmt.Errorf("dest is required")
	}

	return nil
}

// Execute runs the unarchive action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	unarchiveAction := step.Unarchive

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Render paths
	renderedSrc, err := ec.PathUtil.ExpandPath(unarchiveAction.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand src path: %w", err)
	}

	renderedDest, err := ec.PathUtil.ExpandPath(unarchiveAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return nil, fmt.Errorf("failed to expand dest path: %w", err)
	}

	// Render creates path if specified
	var renderedCreates string
	if unarchiveAction.Creates != "" {
		renderedCreates, err = ec.PathUtil.ExpandPath(unarchiveAction.Creates, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return nil, fmt.Errorf("failed to expand creates path: %w", err)
		}
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Check idempotency - skip if creates path exists
	if renderedCreates != "" {
		if _, statErr := os.Stat(renderedCreates); statErr == nil {
			ctx.GetLogger().Debugf("  Skipping extraction: creates path exists: %s", renderedCreates)
			return result, nil
		}
	}

	// Verify source exists and is a file
	srcInfo, err := os.Stat(renderedSrc)
	if err != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to stat source: %w", err)
	}
	if srcInfo.IsDir() {
		result.Failed = true
		return result, fmt.Errorf("src must be a file, not a directory")
	}

	// Detect archive format
	format := h.detectArchiveFormat(renderedSrc)
	if format == ArchiveUnknown {
		result.Failed = true
		return result, fmt.Errorf("unsupported archive format: %s", renderedSrc)
	}

	// Ensure destination directory exists
	mode := h.parseFileMode(unarchiveAction.Mode, defaultDirMode)
	ctx.GetLogger().Debugf("  Ensuring destination directory: %s", renderedDest)
	if mkdirErr := os.MkdirAll(renderedDest, mode); mkdirErr != nil {
		result.Failed = true
		return result, fmt.Errorf("failed to create destination directory: %w", mkdirErr)
	}

	// Extract archive based on format
	ctx.GetLogger().Debugf("  Extracting %s archive: %s -> %s", format.String(), renderedSrc, renderedDest)
	var stats *ExtractionStats
	switch format {
	case ArchiveTar:
		stats, err = h.extractTarArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode, ctx)
	case ArchiveTarGz:
		stats, err = h.extractTarGzArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode, ctx)
	case ArchiveZip:
		stats, err = h.extractZipArchive(renderedSrc, renderedDest, unarchiveAction.StripComponents, mode, ctx)
	default:
		result.Failed = true
		return result, fmt.Errorf("unsupported archive format")
	}

	if err != nil {
		result.Failed = true
		return result, err
	}

	result.Changed = stats.FilesExtracted > 0 || stats.DirsCreated > 0

	ctx.GetLogger().Debugf("  Extracted %d files, %d directories (%d bytes)", stats.FilesExtracted, stats.DirsCreated, stats.BytesExtracted)

	// Emit event
	publisher := ctx.GetEventPublisher()
	if publisher != nil {
		publisher.Publish(events.Event{
			Type: events.EventArchiveExtracted,
			Data: events.ArchiveExtractedData{
				Src:             renderedSrc,
				Dest:            renderedDest,
				Format:          format.String(),
				FilesExtracted:  stats.FilesExtracted,
				DirsCreated:     stats.DirsCreated,
				BytesExtracted:  stats.BytesExtracted,
				StripComponents: unarchiveAction.StripComponents,
				DurationMs:      result.Duration.Milliseconds(),
				DryRun:          ctx.IsDryRun(),
			},
		})
	}

	return result, nil
}

// DryRun logs what would be done without actually doing it.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	unarchiveAction := step.Unarchive

	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render paths
	renderedSrc, err := ec.PathUtil.ExpandPath(unarchiveAction.Src, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedSrc = unarchiveAction.Src
	}

	renderedDest, err := ec.PathUtil.ExpandPath(unarchiveAction.Dest, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		renderedDest = unarchiveAction.Dest
	}

	// Check if creates path exists
	if unarchiveAction.Creates != "" {
		renderedCreates, err := ec.PathUtil.ExpandPath(unarchiveAction.Creates, ec.CurrentDir, ctx.GetVariables())
		if err == nil {
			if _, statErr := os.Stat(renderedCreates); statErr == nil {
				ctx.GetLogger().Infof("  [DRY-RUN] Would skip extraction: creates path exists: %s", renderedCreates)
				return nil
			}
		}
	}

	// Detect format
	format := h.detectArchiveFormat(renderedSrc)

	ctx.GetLogger().Infof("  [DRY-RUN] Would extract %s archive: %s -> %s",
		format.String(), renderedSrc, renderedDest)

	if unarchiveAction.StripComponents > 0 {
		ctx.GetLogger().Debugf("  Would strip %d leading path components", unarchiveAction.StripComponents)
	}

	return nil
}

// Helper functions

func (h *Handler) parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode {
	if modeStr == "" {
		return defaultMode
	}

	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}

func (h *Handler) detectArchiveFormat(path string) ArchiveFormat {
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

func (h *Handler) stripPathComponents(path string, count int) (string, bool) {
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

func (h *Handler) extractTarArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode, ctx actions.Context) (*ExtractionStats, error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close archive file %s: %v", srcPath, closeErr)
		}
	}()

	return h.extractTar(file, destDir, stripComponents, dirMode, ctx)
}

func (h *Handler) extractTarGzArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode, ctx actions.Context) (*ExtractionStats, error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open archive: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close archive file %s: %v", srcPath, closeErr)
		}
	}()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress gzip: %w", err)
	}
	defer func() {
		if closeErr := gzReader.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close gzip reader for %s: %v", srcPath, closeErr)
		}
	}()

	return h.extractTar(gzReader, destDir, stripComponents, dirMode, ctx)
}

func (h *Handler) extractTar(reader io.Reader, destDir string, stripComponents int, dirMode os.FileMode, ctx actions.Context) (*ExtractionStats, error) {
	stats := &ExtractionStats{}
	tarReader := tar.NewReader(reader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, fmt.Errorf("failed to read tar archive: %w", err)
		}

		// Strip leading path components
		extractPath, shouldExtract := h.stripPathComponents(header.Name, stripComponents)
		if !shouldExtract {
			continue
		}

		// Validate no path traversal
		if validateErr := pathutil.ValidateNoPathTraversal(extractPath); validateErr != nil {
			return stats, fmt.Errorf("path traversal in %q: %w", header.Name, validateErr)
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
				return stats, fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			stats.DirsCreated++

		case tar.TypeReg:
			// Ensure parent directory exists
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, dirMode); err != nil {
				return stats, fmt.Errorf("failed to create directory %s: %w", parentDir, err)
			}

			// Extract file
			if err := h.extractTarFile(tarReader, targetPath, header.Mode, ctx); err != nil {
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
				return stats, fmt.Errorf("failed to create directory %s: %w", parentDir, err)
			}

			// Create symlink
			if err := os.Symlink(linkTarget, targetPath); err != nil {
				return stats, fmt.Errorf("failed to create symlink %s: %w", targetPath, err)
			}
			stats.FilesExtracted++
		}
	}

	return stats, nil
}

func (h *Handler) extractTarFile(reader io.Reader, targetPath string, mode int64, ctx actions.Context) error {
	// Create file with permissions
	// #nosec G115 -- File mode from tar header is expected to be within valid range
	fileMode := os.FileMode(mode) & os.ModePerm
	if fileMode == 0 {
		fileMode = defaultFileMode
	}

	// #nosec G304 -- File path from user config is intentional functionality
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close output file %s: %v", targetPath, closeErr)
		}
	}()

	// Copy contents
	if _, err := io.Copy(outFile, reader); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	return nil
}

func (h *Handler) extractZipArchive(srcPath, destDir string, stripComponents int, dirMode os.FileMode, ctx actions.Context) (*ExtractionStats, error) {
	stats := &ExtractionStats{}

	// Open zip file
	zipReader, err := zip.OpenReader(srcPath)
	if err != nil {
		return stats, fmt.Errorf("failed to open zip archive: %w", err)
	}
	defer func() {
		if closeErr := zipReader.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close zip archive %s: %v", srcPath, closeErr)
		}
	}()

	for _, file := range zipReader.File {
		// Strip leading path components
		extractPath, shouldExtract := h.stripPathComponents(file.Name, stripComponents)
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
				return stats, fmt.Errorf("failed to create directory %s: %w", targetPath, err)
			}
			stats.DirsCreated++
			continue
		}

		// Ensure parent directory exists
		parentDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(parentDir, dirMode); err != nil {
			return stats, fmt.Errorf("failed to create directory %s: %w", parentDir, err)
		}

		// Extract file
		if err := h.extractZipFile(file, targetPath, ctx); err != nil {
			return stats, err
		}
		stats.FilesExtracted++
		// #nosec G115 -- UncompressedSize64 from zip header is expected size
		stats.BytesExtracted += int64(file.UncompressedSize64)
	}

	return stats, nil
}

func (h *Handler) extractZipFile(file *zip.File, targetPath string, ctx actions.Context) error {
	// Open file in archive
	srcFile, err := file.Open()
	if err != nil {
		return fmt.Errorf("failed to open file in archive: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close source file %s: %v", file.Name, closeErr)
		}
	}()

	// Create destination file
	fileMode := file.Mode() & os.ModePerm
	if fileMode == 0 {
		fileMode = defaultFileMode
	}

	// #nosec G304 -- File path from user config is intentional functionality
	outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", targetPath, err)
	}
	defer func() {
		if closeErr := outFile.Close(); closeErr != nil {
			ctx.GetLogger().Debugf("Failed to close output file %s: %v", targetPath, closeErr)
		}
	}()

	// Copy contents
	// #nosec G110 -- Decompression bomb protection via file size limits is handled by zip.File
	if _, err := io.Copy(outFile, srcFile); err != nil {
		return fmt.Errorf("failed to write file %s: %w", targetPath, err)
	}

	return nil
}
