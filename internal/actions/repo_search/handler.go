// Package repo_search implements the repo_search action handler.
//
// The repo_search action searches a codebase for patterns and outputs results in JSON format.
// It supports:
// - Literal and regex-based pattern matching
// - Glob-based file filtering
// - Configurable search paths
// - JSON output for easy parsing by LLM agents
// - Result limiting and directory ignoring
package repo_search

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName = "repo_search"
)

// SearchResult represents a single match in a file.
type SearchResult struct {
	File    string `json:"file"`              // File path relative to search root
	Line    int    `json:"line"`              // Line number (1-based)
	Column  int    `json:"column,omitempty"`  // Column number (1-based, for regex matches)
	Match   string `json:"match"`             // The matched text
	Context string `json:"context,omitempty"` // Full line containing the match
}

// SearchOutput is the JSON structure written to output file.
type SearchOutput struct {
	Pattern     string         `json:"pattern"`               // Search pattern used
	Regex       bool           `json:"regex"`                 // Whether regex mode was used
	Glob        string         `json:"glob,omitempty"`        // Glob pattern used
	Path        string         `json:"path"`                  // Search root path
	TotalFiles  int            `json:"total_files_searched"`  // Number of files searched
	TotalMatches int           `json:"total_matches"`         // Total number of matches found
	Results     []SearchResult `json:"results"`               // Individual match results
	Timestamp   time.Time      `json:"timestamp"`             // When search was performed
}

// Handler implements the Handler interface for repo_search actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the repo_search action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        actionName,
		Description: "Search codebase for patterns and output results in JSON format",
		Category:    actions.CategoryFile,
		SupportsDryRun: true,
		SupportsBecome: false,
		EmitsEvents:        []string{}, // No events emitted (read-only operation)
		Version:            "1.0.0",
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate checks if the repo_search configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.RepoSearch == nil {
		return fmt.Errorf("repo_search configuration is nil")
	}

	rs := step.RepoSearch

	if rs.Pattern == "" {
		hint := actions.GetActionHint(actionName, "pattern")
		return fmt.Errorf("pattern is required%s", hint)
	}

	// Validate regex if regex mode enabled
	if rs.Regex {
		_, err := regexp.Compile(rs.Pattern)
		if err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", rs.Pattern, err)
		}
	}

	// Validate max_results is positive if specified
	if rs.MaxResults != nil && *rs.MaxResults <= 0 {
		return fmt.Errorf("max_results must be positive, got %d", *rs.MaxResults)
	}

	return nil
}

// Execute runs the repo_search action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rs := step.RepoSearch

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false // Search operations never "change" anything

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Determine search root path
	searchPath := rs.Path
	if searchPath == "" {
		searchPath = "."
	}

	// Expand and render path
	renderedPath, err := ec.PathUtil.ExpandPath(searchPath, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path exists
	if _, err := os.Stat(renderedPath); err != nil {
		return result, fmt.Errorf("search path does not exist: %s", renderedPath)
	}

	// Render pattern (support template variables)
	renderedPattern, err := ctx.GetTemplate().Render(rs.Pattern, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to render pattern: %w", err)
	}

	// Perform search
	output, err := h.performSearch(renderedPath, renderedPattern, rs, ctx)
	if err != nil {
		return result, err
	}

	// Write output to file if specified
	if rs.OutputFile != "" {
		outputPath, err := ec.PathUtil.ExpandPath(rs.OutputFile, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("failed to expand output_file path: %w", err)
		}

		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return result, fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Create directory if needed
		if dir := filepath.Dir(outputPath); dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return result, fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
			return result, fmt.Errorf("failed to write output file: %w", err)
		}

		ctx.GetLogger().Infof("  Wrote %d results to %s", output.TotalMatches, outputPath)
	}

	// Set result data
	result.SetData(map[string]interface{}{
		"total_files":   output.TotalFiles,
		"total_matches": output.TotalMatches,
		"results":       output.Results,
	})

	ctx.GetLogger().Infof("  Found %d matches in %d files", output.TotalMatches, output.TotalFiles)

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	rs := step.RepoSearch

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	searchPath := rs.Path
	if searchPath == "" {
		searchPath = "."
	}

	renderedPath, err := ec.PathUtil.ExpandPath(searchPath, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = searchPath
	}

	// Render pattern (best effort)
	renderedPattern, err := ctx.GetTemplate().Render(rs.Pattern, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render pattern: %v", err)
		renderedPattern = rs.Pattern
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would search for pattern '%s' in %s", renderedPattern, renderedPath)

	if rs.Glob != "" {
		ctx.GetLogger().Infof("            Glob filter: %s", rs.Glob)
	}

	if rs.OutputFile != "" {
		outputPath, _ := ec.PathUtil.ExpandPath(rs.OutputFile, ec.CurrentDir, ctx.GetVariables())
		ctx.GetLogger().Infof("            Output file: %s", outputPath)
	}

	if rs.MaxResults != nil {
		ctx.GetLogger().Infof("            Max results: %d", *rs.MaxResults)
	}

	if len(rs.IgnoreDirs) > 0 {
		ctx.GetLogger().Infof("            Ignore dirs: %v", rs.IgnoreDirs)
	}

	return nil
}

// performSearch executes the actual search operation
func (h *Handler) performSearch(rootPath, pattern string, rs *config.RepoSearch, ctx actions.Context) (*SearchOutput, error) {
	output := &SearchOutput{
		Pattern:   pattern,
		Regex:     rs.Regex,
		Glob:      rs.Glob,
		Path:      rootPath,
		Results:   make([]SearchResult, 0),
		Timestamp: time.Now(),
	}

	// Compile regex if needed
	var re *regexp.Regexp
	var err error
	if rs.Regex {
		re, err = regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to compile regex: %w", err)
		}
	}

	// Build ignore map for faster lookup
	ignoreMap := make(map[string]bool)
	for _, dir := range rs.IgnoreDirs {
		ignoreMap[dir] = true
	}

	// Walk the directory tree
	filesSearched := 0
	matchCount := 0
	maxResults := -1
	if rs.MaxResults != nil {
		maxResults = *rs.MaxResults
	}

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Log but continue on permission errors
			ctx.GetLogger().Debugf("  Warning: %v", err)
			return nil
		}

		// Skip directories in ignore list
		if info.IsDir() {
			if ignoreMap[info.Name()] || ignoreMap[filepath.Base(path)] {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip non-regular files
		if !info.Mode().IsRegular() {
			return nil
		}

		// Apply glob filter if specified
		if rs.Glob != "" {
			matched, err := filepath.Match(rs.Glob, filepath.Base(path))
			if err != nil {
				return fmt.Errorf("invalid glob pattern: %w", err)
			}
			if !matched {
				// Also try matching against relative path for patterns like "**/*.ts"
				relPath, _ := filepath.Rel(rootPath, path)
				matched, _ = filepath.Match(rs.Glob, relPath)
				if !matched {
					// Simple contains check for patterns like "*.{ts,js}"
					if !h.matchesGlobPattern(filepath.Base(path), rs.Glob) {
						return nil
					}
				}
			}
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			ctx.GetLogger().Debugf("  Warning: Failed to read %s: %v", path, err)
			return nil
		}

		// Search in file
		relPath, _ := filepath.Rel(rootPath, path)
		if relPath == "" {
			relPath = path
		}

		fileMatches := h.searchInFile(relPath, string(content), pattern, re, rs.Regex)
		if len(fileMatches) > 0 {
			filesSearched++
			for _, match := range fileMatches {
				if maxResults > 0 && matchCount >= maxResults {
					return filepath.SkipAll // Stop searching
				}
				output.Results = append(output.Results, match)
				matchCount++
			}
		}

		return nil
	})

	if err != nil && err != filepath.SkipAll {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	output.TotalFiles = filesSearched
	output.TotalMatches = matchCount

	return output, nil
}

// searchInFile searches for pattern in file content and returns matches
func (h *Handler) searchInFile(filePath, content, pattern string, re *regexp.Regexp, useRegex bool) []SearchResult {
	results := make([]SearchResult, 0)
	lines := strings.Split(content, "\n")

	for lineNum, line := range lines {
		if useRegex && re != nil {
			// Regex search
			matches := re.FindAllStringIndex(line, -1)
			for _, match := range matches {
				results = append(results, SearchResult{
					File:    filePath,
					Line:    lineNum + 1,
					Column:  match[0] + 1,
					Match:   line[match[0]:match[1]],
					Context: line,
				})
			}
		} else {
			// Literal search
			if strings.Contains(line, pattern) {
				col := strings.Index(line, pattern)
				results = append(results, SearchResult{
					File:    filePath,
					Line:    lineNum + 1,
					Column:  col + 1,
					Match:   pattern,
					Context: line,
				})
			}
		}
	}

	return results
}

// matchesGlobPattern checks if filename matches a glob pattern (simple implementation)
func (h *Handler) matchesGlobPattern(filename, globPattern string) bool {
	// Handle patterns like "*.{ts,js}" by extracting extensions
	if strings.Contains(globPattern, "{") && strings.Contains(globPattern, "}") {
		start := strings.Index(globPattern, "{")
		end := strings.Index(globPattern, "}")
		if start < end {
			prefix := globPattern[:start]
			suffix := globPattern[end+1:]
			extensions := strings.Split(globPattern[start+1:end], ",")

			for _, ext := range extensions {
				pattern := prefix + strings.TrimSpace(ext) + suffix
				if matched, _ := filepath.Match(pattern, filename); matched {
					return true
				}
			}
		}
	}

	// Fallback to simple match
	matched, _ := filepath.Match(globPattern, filename)
	return matched
}
