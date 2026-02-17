// Package artifact_validate implements the artifact.validate action handler.
// Validates artifacts against constraints (change budgets) for LLM agent loops.
package artifact_validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// Handler implements the artifact_validate action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "artifact_validate",
		Description:        "Validate artifacts against constraints (change budgets)",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{}, // All platforms
		RequiresSudo:       false,
		ImplementsCheck:    false,
	}
}

// Validate validates the artifact_validate action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.ArtifactValidate == nil {
		return fmt.Errorf("artifact_validate action requires artifact_validate configuration")
	}

	validate := step.ArtifactValidate
	if validate.ArtifactFile == "" {
		return fmt.Errorf("artifact_validate requires artifact_file")
	}

	return nil
}

// Execute executes the artifact_validate action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	validate := step.ArtifactValidate

	// Read artifact metadata
	metadata, err := readArtifactMetadata(validate.ArtifactFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact file: %w", err)
	}

	ec.Logger.Infof("Validating artifact '%s' with %d file changes", metadata.Name, len(metadata.Files))

	// Run validations
	violations := make([]artifacts.ValidationViolation, 0)

	// Validate max files
	if validate.MaxFiles != nil {
		if len(metadata.Files) > *validate.MaxFiles {
			violations = append(violations, artifacts.ValidationViolation{
				Constraint: "max_files",
				Expected:   fmt.Sprintf("<= %d", *validate.MaxFiles),
				Actual:     fmt.Sprintf("%d", len(metadata.Files)),
				Message:    fmt.Sprintf("Too many files changed: %d files (max: %d)", len(metadata.Files), *validate.MaxFiles),
			})
		}
	}

	// Validate max lines changed
	if validate.MaxLinesChanged != nil {
		totalLines := metadata.Summary.TotalLinesChanged
		if totalLines > *validate.MaxLinesChanged {
			violations = append(violations, artifacts.ValidationViolation{
				Constraint: "max_lines_changed",
				Expected:   fmt.Sprintf("<= %d", *validate.MaxLinesChanged),
				Actual:     fmt.Sprintf("%d", totalLines),
				Message:    fmt.Sprintf("Too many lines changed: %d lines (max: %d)", totalLines, *validate.MaxLinesChanged),
			})
		}
	}

	// Validate max file size
	if validate.MaxFileSize != nil {
		for _, file := range metadata.Files {
			if file.SizeAfter > int64(*validate.MaxFileSize) {
				violations = append(violations, artifacts.ValidationViolation{
					Constraint: "max_file_size",
					Expected:   fmt.Sprintf("<= %d bytes", *validate.MaxFileSize),
					Actual:     fmt.Sprintf("%d bytes", file.SizeAfter),
					Message:    fmt.Sprintf("File too large: %s (%d bytes, max: %d)", file.Path, file.SizeAfter, *validate.MaxFileSize),
				})
			}
		}
	}

	// Validate require tests
	if validate.RequireTests {
		hasCodeChanges := false
		hasTestChanges := false

		for _, file := range metadata.Files {
			if file.FileType == "code" && !file.IsTestFile {
				hasCodeChanges = true
			}
			if file.IsTestFile || file.FileType == "test" {
				hasTestChanges = true
			}
		}

		if hasCodeChanges && !hasTestChanges {
			violations = append(violations, artifacts.ValidationViolation{
				Constraint: "require_tests",
				Expected:   "test files must be modified when code files change",
				Actual:     "code changed without test changes",
				Message:    "Code files were modified but no test files were changed",
			})
		}
	}

	// Validate allowed paths
	if len(validate.AllowedPaths) > 0 {
		for _, file := range metadata.Files {
			allowed := false
			for _, pattern := range validate.AllowedPaths {
				if matchGlob(pattern, file.Path) {
					allowed = true
					break
				}
			}
			if !allowed {
				violations = append(violations, artifacts.ValidationViolation{
					Constraint: "allowed_paths",
					Expected:   fmt.Sprintf("matches one of: %v", validate.AllowedPaths),
					Actual:     file.Path,
					Message:    fmt.Sprintf("File not in allowed paths: %s", file.Path),
				})
			}
		}
	}

	// Validate forbidden paths
	if len(validate.ForbiddenPaths) > 0 {
		for _, file := range metadata.Files {
			for _, pattern := range validate.ForbiddenPaths {
				if matchGlob(pattern, file.Path) {
					violations = append(violations, artifacts.ValidationViolation{
						Constraint: "forbidden_paths",
						Expected:   fmt.Sprintf("does not match: %v", validate.ForbiddenPaths),
						Actual:     file.Path,
						Message:    fmt.Sprintf("File in forbidden path: %s", file.Path),
					})
					break
				}
			}
		}
	}

	// Update artifact metadata with validation results
	metadata.Validated = true
	metadata.ValidationPass = len(violations) == 0
	metadata.Violations = violations

	// Write updated metadata back to file
	if err := writeArtifactMetadata(validate.ArtifactFile, metadata); err != nil {
		ec.Logger.Debugf("Failed to update artifact metadata: %v", err)
	}

	// Create result
	result := executor.NewResult()
	result.Changed = false // Validation never changes state

	if len(violations) > 0 {
		// Validation failed
		ec.Logger.Errorf("Artifact validation failed with %d violations:", len(violations))
		for i, v := range violations {
			ec.Logger.Errorf("  %d. %s: %s", i+1, v.Constraint, v.Message)
		}
		return nil, fmt.Errorf("artifact validation failed: %d constraint violations", len(violations))
	}

	// Validation passed
	ec.Logger.Infof("Artifact validation passed: %d files, %d lines changed",
		len(metadata.Files), metadata.Summary.TotalLinesChanged)
	result.Stdout = fmt.Sprintf("Validation passed: %d files checked", len(metadata.Files))

	return result, nil
}

// DryRun logs what the artifact validate would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	validate := step.ArtifactValidate

	constraints := make([]string, 0)
	if validate.MaxFiles != nil {
		constraints = append(constraints, fmt.Sprintf("max_files=%d", *validate.MaxFiles))
	}
	if validate.MaxLinesChanged != nil {
		constraints = append(constraints, fmt.Sprintf("max_lines=%d", *validate.MaxLinesChanged))
	}
	if validate.MaxFileSize != nil {
		constraints = append(constraints, fmt.Sprintf("max_size=%d", *validate.MaxFileSize))
	}
	if validate.RequireTests {
		constraints = append(constraints, "require_tests=true")
	}
	if len(validate.AllowedPaths) > 0 {
		constraints = append(constraints, fmt.Sprintf("allowed_paths=%d", len(validate.AllowedPaths)))
	}
	if len(validate.ForbiddenPaths) > 0 {
		constraints = append(constraints, fmt.Sprintf("forbidden_paths=%d", len(validate.ForbiddenPaths)))
	}

	ec.Logger.Infof("  [DRY-RUN] Would validate artifact '%s' (constraints: %s)",
		validate.ArtifactFile, strings.Join(constraints, ", "))

	return nil
}

// readArtifactMetadata reads artifact metadata from JSON file.
func readArtifactMetadata(path string) (*artifacts.ArtifactMetadata, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path from config
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var metadata artifacts.ArtifactMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &metadata, nil
}

// writeArtifactMetadata writes artifact metadata to JSON file.
func writeArtifactMetadata(path string, metadata *artifacts.ArtifactMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil { // #nosec G306 -- standard file permissions
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// matchGlob checks if a path matches a glob pattern.
// Supports * and ** wildcards.
func matchGlob(pattern, path string) bool {
	// Handle ** (match any number of path components)
	if strings.Contains(pattern, "**") {
		// Split pattern by **
		parts := strings.Split(pattern, "**")

		// For patterns like "src/**/*.go"
		if len(parts) == 2 {
			prefix := strings.TrimSuffix(parts[0], "/")
			suffix := strings.TrimPrefix(parts[1], "/")

			// Check prefix
			if prefix != "" {
				if !strings.HasPrefix(path, prefix+"/") && path != prefix {
					return false
				}
				// Remove prefix from path for suffix matching
				if strings.HasPrefix(path, prefix+"/") {
					path = path[len(prefix)+1:]
				}
			}

			// Check suffix
			if suffix != "" {
				// For suffix patterns like "*.go", use filepath.Match
				if strings.Contains(suffix, "/") {
					// Multi-part suffix like "pkg/*.go"
					return strings.HasSuffix(path, "/"+suffix) ||
						matchSuffixPattern(path, suffix)
				}
				// Simple suffix like "*.go"
				matched, _ := filepath.Match(suffix, filepath.Base(path))
				return matched
			}

			return true
		}
	}

	// Use filepath.Match for simple patterns
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}

// matchSuffixPattern checks if path ends with a pattern.
func matchSuffixPattern(path, suffix string) bool {
	parts := strings.Split(path, "/")
	suffixParts := strings.Split(suffix, "/")

	if len(parts) < len(suffixParts) {
		return false
	}

	// Check each suffix part
	for i := 0; i < len(suffixParts); i++ {
		pathPart := parts[len(parts)-len(suffixParts)+i]
		suffixPart := suffixParts[i]

		matched, _ := filepath.Match(suffixPart, pathPart)
		if !matched {
			return false
		}
	}

	return true
}
