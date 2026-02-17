// Package artifacts provides enhanced metadata structures for artifact generation.
package artifacts

import (
	"path/filepath"
	"strings"
)

// DetailedFileChange represents comprehensive metadata about a file modification.
type DetailedFileChange struct {
	// Basic information
	Path      string `json:"path"`
	Operation string `json:"operation"` // "created", "updated", "deleted", "template"
	StepID    string `json:"step_id,omitempty"`
	StepName  string `json:"step_name,omitempty"`

	// Size information
	SizeBefore int64 `json:"size_before,omitempty"`
	SizeAfter  int64 `json:"size_after,omitempty"`
	SizeDelta  int64 `json:"size_delta,omitempty"` // Positive = grew, negative = shrunk

	// Checksums
	ChecksumBefore string `json:"checksum_before,omitempty"`
	ChecksumAfter  string `json:"checksum_after,omitempty"`

	// Line statistics
	LinesAdded    int `json:"lines_added"`
	LinesRemoved  int `json:"lines_removed"`
	LinesModified int `json:"lines_modified"`
	LinesTotal    int `json:"lines_total"` // Total lines after change

	// Character/token statistics
	CharsAdded   int `json:"chars_added,omitempty"`
	CharsRemoved int `json:"chars_removed,omitempty"`

	// Diff information
	DiffFile    string   `json:"diff_file,omitempty"`    // Relative path to diff file
	HunkCount   int      `json:"hunk_count,omitempty"`   // Number of hunks in diff
	HunkSummary []string `json:"hunk_summary,omitempty"` // Brief description of each hunk

	// Language/file type detection
	Language   string `json:"language,omitempty"`   // Detected language (e.g., "go", "python", "javascript")
	FileType   string `json:"file_type,omitempty"`  // Category (e.g., "code", "test", "config", "docs")
	IsTestFile bool   `json:"is_test_file"`

	// Content (optional, can be large)
	ContentBefore string `json:"content_before,omitempty"`
	ContentAfter  string `json:"content_after,omitempty"`
}

// AggregatedChanges represents high-level statistics across all file changes.
type AggregatedChanges struct {
	// Overall statistics
	TotalFiles        int   `json:"total_files"`
	TotalLinesAdded   int   `json:"total_lines_added"`
	TotalLinesRemoved int   `json:"total_lines_removed"`
	TotalLinesChanged int   `json:"total_lines_changed"` // Sum of added + removed
	TotalCharsAdded   int   `json:"total_chars_added"`
	TotalCharsRemoved int   `json:"total_chars_removed"`
	TotalSizeDelta    int64 `json:"total_size_delta"` // Net size change in bytes

	// File categorization
	FilesCreated int `json:"files_created"`
	FilesUpdated int `json:"files_updated"`
	FilesDeleted int `json:"files_deleted"`

	// File type breakdown
	FilesByLanguage  map[string]int `json:"files_by_language"` // Language -> count
	FilesByType      map[string]int `json:"files_by_type"`     // Type -> count
	TestFilesCount   int            `json:"test_files_count"`
	CodeFilesCount   int            `json:"code_files_count"`
	ConfigFilesCount int            `json:"config_files_count"`

	// Directory-level summaries
	DirectoriesAffected []string       `json:"directories_affected"`
	ChangesByDirectory  map[string]int `json:"changes_by_directory"` // Directory -> file count

	// Top changed files (sorted by lines changed)
	TopChangedFiles []FileChangeSummary `json:"top_changed_files,omitempty"`
}

// FileChangeSummary is a brief summary of a file change for top-N lists.
type FileChangeSummary struct {
	Path         string `json:"path"`
	LinesChanged int    `json:"lines_changed"`
	Operation    string `json:"operation"`
}

// ArtifactMetadata represents complete metadata for an artifact capture.
type ArtifactMetadata struct {
	// Basic info
	Name        string `json:"name"`
	CaptureTime string `json:"capture_time"` // ISO8601 timestamp
	RunID       string `json:"run_id,omitempty"`

	// Aggregated statistics
	Summary AggregatedChanges `json:"summary"`

	// Detailed per-file changes
	Files []DetailedFileChange `json:"files"`

	// Validation results (if artifact.validate was run)
	Validated      bool                  `json:"validated,omitempty"`
	ValidationPass bool                  `json:"validation_pass,omitempty"`
	Violations     []ValidationViolation `json:"violations,omitempty"`

	// Additional metadata
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ValidationViolation represents a validation constraint that was violated.
type ValidationViolation struct {
	Constraint string `json:"constraint"` // e.g., "max_files", "max_lines_changed"
	Expected   string `json:"expected"`   // Expected value/constraint
	Actual     string `json:"actual"`     // Actual value
	Message    string `json:"message"`    // Human-readable message
}

// DetectLanguage detects programming language from file path/extension.
func DetectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))

	languageMap := map[string]string{
		// Programming languages
		".go":    "go",
		".py":    "python",
		".js":    "javascript",
		".ts":    "typescript",
		".jsx":   "javascript",
		".tsx":   "typescript",
		".java":  "java",
		".c":     "c",
		".cpp":   "cpp",
		".cc":    "cpp",
		".cxx":   "cpp",
		".h":     "c",
		".hpp":   "cpp",
		".cs":    "csharp",
		".rb":    "ruby",
		".php":   "php",
		".rs":    "rust",
		".swift": "swift",
		".kt":    "kotlin",
		".scala": "scala",
		".sh":    "shell",
		".bash":  "shell",
		".zsh":   "shell",
		".fish":  "shell",

		// Markup/Data
		".html": "html",
		".htm":  "html",
		".xml":  "xml",
		".json": "json",
		".yaml": "yaml",
		".yml":  "yaml",
		".toml": "toml",
		".md":   "markdown",
		".rst":  "restructuredtext",

		// Config
		".conf":       "config",
		".cfg":        "config",
		".ini":        "config",
		".properties": "config",

		// Styles
		".css":  "css",
		".scss": "scss",
		".sass": "sass",
		".less": "less",

		// SQL
		".sql": "sql",

		// Other
		".r":   "r",
		".R":   "r",
		".m":   "matlab",
		".lua": "lua",
		".vim": "vim",
	}

	if lang, ok := languageMap[ext]; ok {
		return lang
	}

	// Check for special filenames
	base := filepath.Base(path)
	if strings.HasPrefix(base, "Dockerfile") {
		return "dockerfile"
	}
	if base == "Makefile" || base == "makefile" {
		return "makefile"
	}

	return "unknown"
}

// DetectFileType categorizes file into broad type.
func DetectFileType(path string) string {
	base := strings.ToLower(filepath.Base(path))

	// Test files
	if strings.Contains(base, "_test.") || strings.Contains(base, ".test.") ||
		strings.HasSuffix(base, "_spec.") || strings.HasPrefix(base, "test_") ||
		strings.Contains(path, "/tests/") || strings.Contains(path, "/test/") {
		return "test"
	}

	// Documentation
	if strings.HasSuffix(base, ".md") || strings.HasSuffix(base, ".rst") ||
		strings.HasSuffix(base, ".txt") || strings.Contains(path, "/docs/") ||
		strings.Contains(path, "/doc/") {
		return "docs"
	}

	// Configuration
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" || ext == ".yaml" || ext == ".yml" || ext == ".toml" ||
		ext == ".ini" || ext == ".conf" || ext == ".cfg" || ext == ".properties" ||
		base == "dockerfile" || base == "makefile" {
		return "config"
	}

	// Code (default for programming languages)
	lang := DetectLanguage(path)
	if lang != "unknown" && lang != "markdown" && lang != "config" {
		return "code"
	}

	return "other"
}

// IsTestFile determines if a file is a test file.
func IsTestFile(path string) bool {
	return DetectFileType(path) == "test"
}

// CalculateLineStats calculates line statistics from before/after content.
func CalculateLineStats(beforeContent, afterContent string) (added, removed, modified int) {
	beforeLines := strings.Split(beforeContent, "\n")
	afterLines := strings.Split(afterContent, "\n")

	// Simple line-by-line comparison
	// This is a basic implementation - could be enhanced with proper diff algorithm
	beforeIdx, afterIdx := 0, 0

	for beforeIdx < len(beforeLines) || afterIdx < len(afterLines) {
		if beforeIdx < len(beforeLines) && afterIdx < len(afterLines) {
			if beforeLines[beforeIdx] == afterLines[afterIdx] {
				// Unchanged line
				beforeIdx++
				afterIdx++
			} else {
				// Modified line
				modified++
				beforeIdx++
				afterIdx++
			}
		} else if beforeIdx < len(beforeLines) {
			// Deleted line
			removed++
			beforeIdx++
		} else {
			// Added line
			added++
			afterIdx++
		}
	}

	// Adjust: if we counted modifications, those are actually add+remove
	if modified > 0 {
		// Simple heuristic: count modifications as both add and remove
		added += modified
		removed += modified
		modified = 0
	}

	return added, removed, modified
}

// AggregateChanges creates aggregated statistics from detailed file changes.
func AggregateChanges(files []DetailedFileChange) AggregatedChanges {
	agg := AggregatedChanges{
		FilesByLanguage:    make(map[string]int),
		FilesByType:        make(map[string]int),
		ChangesByDirectory: make(map[string]int),
		DirectoriesAffected: []string{},
	}

	dirSet := make(map[string]bool)

	for _, file := range files {
		agg.TotalFiles++
		agg.TotalLinesAdded += file.LinesAdded
		agg.TotalLinesRemoved += file.LinesRemoved
		agg.TotalCharsAdded += file.CharsAdded
		agg.TotalCharsRemoved += file.CharsRemoved
		agg.TotalSizeDelta += file.SizeDelta

		// Categorize by operation
		switch file.Operation {
		case "created":
			agg.FilesCreated++
		case "updated":
			agg.FilesUpdated++
		case "deleted":
			agg.FilesDeleted++
		}

		// Categorize by language
		if file.Language != "" && file.Language != "unknown" {
			agg.FilesByLanguage[file.Language]++
		}

		// Categorize by type
		if file.FileType != "" {
			agg.FilesByType[file.FileType]++

			switch file.FileType {
			case "test":
				agg.TestFilesCount++
			case "code":
				agg.CodeFilesCount++
			case "config":
				agg.ConfigFilesCount++
			}
		}

		// Track directories
		dir := filepath.Dir(file.Path)
		if !dirSet[dir] {
			dirSet[dir] = true
			agg.DirectoriesAffected = append(agg.DirectoriesAffected, dir)
		}
		agg.ChangesByDirectory[dir]++
	}

	agg.TotalLinesChanged = agg.TotalLinesAdded + agg.TotalLinesRemoved

	// Get top changed files (up to 10)
	topFiles := make([]DetailedFileChange, len(files))
	copy(topFiles, files)

	// Simple sort by lines changed (bubble sort for small N)
	for i := 0; i < len(topFiles)-1; i++ {
		for j := 0; j < len(topFiles)-i-1; j++ {
			if (topFiles[j].LinesAdded + topFiles[j].LinesRemoved) <
				(topFiles[j+1].LinesAdded + topFiles[j+1].LinesRemoved) {
				topFiles[j], topFiles[j+1] = topFiles[j+1], topFiles[j]
			}
		}
	}

	// Take top 10
	limit := 10
	if len(topFiles) < limit {
		limit = len(topFiles)
	}

	for i := 0; i < limit; i++ {
		agg.TopChangedFiles = append(agg.TopChangedFiles, FileChangeSummary{
			Path:         topFiles[i].Path,
			LinesChanged: topFiles[i].LinesAdded + topFiles[i].LinesRemoved,
			Operation:    topFiles[i].Operation,
		})
	}

	return agg
}

// EnhanceFileChange enriches a basic FileChange with detailed metadata.
func EnhanceFileChange(fc *FileChange, beforeContent, afterContent string) *DetailedFileChange {
	detailed := &DetailedFileChange{
		Path:           fc.Path,
		Operation:      fc.Operation,
		StepID:         fc.StepID,
		SizeAfter:      fc.SizeBytes,
		ChecksumBefore: fc.ChecksumBefore,
		ChecksumAfter:  fc.ChecksumAfter,
		DiffFile:       fc.DiffFile,
		Language:       DetectLanguage(fc.Path),
		FileType:       DetectFileType(fc.Path),
		IsTestFile:     IsTestFile(fc.Path),
	}

	// Calculate line statistics
	if beforeContent != "" || afterContent != "" {
		added, removed, modified := CalculateLineStats(beforeContent, afterContent)
		detailed.LinesAdded = added
		detailed.LinesRemoved = removed
		detailed.LinesModified = modified
		detailed.LinesTotal = len(strings.Split(afterContent, "\n"))

		// Character statistics
		detailed.CharsAdded = len(afterContent) - len(beforeContent)
		if detailed.CharsAdded < 0 {
			detailed.CharsRemoved = -detailed.CharsAdded
			detailed.CharsAdded = 0
		}
	}

	return detailed
}
