// Package repo_tree implements the repo_tree action handler.
//
// The repo_tree action generates a JSON representation of a directory structure.
// It supports:
// - Configurable maximum depth
// - Directory exclusion (e.g., .git, node_modules)
// - Optional file inclusion
// - JSON output for easy parsing by LLM agents
package repo_tree

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

const (
	actionName = "repo_tree"
)

// TreeNode represents a file or directory in the tree structure.
type TreeNode struct {
	Name     string      `json:"name"`               // File or directory name
	Type     string      `json:"type"`               // "file" or "directory"
	Path     string      `json:"path"`               // Relative path from root
	Size     int64       `json:"size,omitempty"`     // File size in bytes (files only)
	Children []TreeNode  `json:"children,omitempty"` // Child nodes (directories only)
}

// TreeOutput is the JSON structure written to output file.
type TreeOutput struct {
	RootPath      string    `json:"root_path"`        // Root path that was traversed
	MaxDepth      int       `json:"max_depth"`        // Maximum depth traversed (-1 = unlimited)
	IncludeFiles  bool      `json:"include_files"`    // Whether files were included
	TotalDirs     int       `json:"total_dirs"`       // Total number of directories
	TotalFiles    int       `json:"total_files"`      // Total number of files
	Tree          TreeNode  `json:"tree"`             // Root tree node
	Timestamp     time.Time `json:"timestamp"`        // When tree was generated
}

// Handler implements the Handler interface for repo_tree actions.
type Handler struct{}

// Register this handler on import
func init() {
	actions.Register(&Handler{})
}

// Metadata returns metadata about the repo_tree action.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        actionName,
		Description: "Generate a JSON representation of directory structure",
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

// Validate checks if the repo_tree configuration is valid.
func (h *Handler) Validate(step *config.Step) error {
	if step.RepoTree == nil {
		return fmt.Errorf("repo_tree configuration is nil")
	}

	rt := step.RepoTree

	// Validate max_depth is positive if specified
	if rt.MaxDepth != nil && *rt.MaxDepth <= 0 {
		return fmt.Errorf("max_depth must be positive, got %d", *rt.MaxDepth)
	}

	return nil
}

// Execute runs the repo_tree action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	rt := step.RepoTree

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("context is not an ExecutionContext")
	}

	// Create result
	result := executor.NewResult()
	result.StartTime = time.Now()
	result.Changed = false // Tree generation never "changes" anything

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// Determine root path
	rootPath := rt.Path
	if rootPath == "" {
		rootPath = "."
	}

	// Expand and render path
	renderedPath, err := ec.PathUtil.ExpandPath(rootPath, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		return result, fmt.Errorf("failed to expand path: %w", err)
	}

	// Validate path exists
	if _, statErr := os.Stat(renderedPath); statErr != nil {
		return result, fmt.Errorf("root path does not exist: %s", renderedPath)
	}

	// Determine max depth
	maxDepth := -1 // unlimited
	if rt.MaxDepth != nil {
		maxDepth = *rt.MaxDepth
	}

	// Determine if files should be included (default: true)
	includeFiles := true
	if !rt.IncludeFiles {
		// Only set to false if explicitly set to false
		// This handles the zero value case
		includeFiles = rt.IncludeFiles
	}

	// Build exclude map for faster lookup
	excludeMap := make(map[string]bool)
	for _, dir := range rt.ExcludeDirs {
		excludeMap[dir] = true
	}

	// Generate tree
	output := &TreeOutput{
		RootPath:     renderedPath,
		MaxDepth:     maxDepth,
		IncludeFiles: includeFiles,
		Timestamp:    time.Now(),
	}

	rootNode, err := h.buildTree(renderedPath, "", 0, maxDepth, includeFiles, excludeMap, output, ctx)
	if err != nil {
		return result, fmt.Errorf("failed to build tree: %w", err)
	}

	output.Tree = rootNode

	// Write output to file if specified
	if rt.OutputFile != "" {
		outputPath, err := ec.PathUtil.ExpandPath(rt.OutputFile, ec.CurrentDir, ctx.GetVariables())
		if err != nil {
			return result, fmt.Errorf("failed to expand output_file path: %w", err)
		}

		jsonData, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return result, fmt.Errorf("failed to marshal JSON: %w", err)
		}

		// Create directory if needed
		if dir := filepath.Dir(outputPath); dir != "." {
			if err := os.MkdirAll(dir, 0750); err != nil { // #nosec G301 - output directory permissions
				return result, fmt.Errorf("failed to create output directory: %w", err)
			}
		}

		if err := os.WriteFile(outputPath, jsonData, 0600); err != nil { // #nosec G306 - output file permissions
			return result, fmt.Errorf("failed to write output file: %w", err)
		}

		ctx.GetLogger().Infof("  Wrote tree to %s", outputPath)
	}

	// Set result data
	result.SetData(map[string]interface{}{
		"total_dirs":  output.TotalDirs,
		"total_files": output.TotalFiles,
		"tree":        output.Tree,
	})

	ctx.GetLogger().Infof("  Generated tree: %d directories, %d files", output.TotalDirs, output.TotalFiles)

	return result, nil
}

// DryRun logs what would happen without making changes.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	rt := step.RepoTree

	// We need ExecutionContext for PathUtil
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("context is not an ExecutionContext")
	}

	// Render path
	rootPath := rt.Path
	if rootPath == "" {
		rootPath = "."
	}

	renderedPath, err := ec.PathUtil.ExpandPath(rootPath, ec.CurrentDir, ctx.GetVariables())
	if err != nil {
		ctx.GetLogger().Infof("  [DRY-RUN] Warning: Failed to render path: %v", err)
		renderedPath = rootPath
	}

	ctx.GetLogger().Infof("  [DRY-RUN] Would generate tree for %s", renderedPath)

	if rt.MaxDepth != nil {
		ctx.GetLogger().Infof("            Max depth: %d", *rt.MaxDepth)
	} else {
		ctx.GetLogger().Infof("            Max depth: unlimited")
	}

	if rt.OutputFile != "" {
		outputPath, _ := ec.PathUtil.ExpandPath(rt.OutputFile, ec.CurrentDir, ctx.GetVariables())
		ctx.GetLogger().Infof("            Output file: %s", outputPath)
	}

	if len(rt.ExcludeDirs) > 0 {
		ctx.GetLogger().Infof("            Exclude dirs: %v", rt.ExcludeDirs)
	}

	if !rt.IncludeFiles {
		ctx.GetLogger().Infof("            Include files: false (directories only)")
	}

	return nil
}

// buildTree recursively builds the tree structure
func (h *Handler) buildTree(
	absPath, relPath string,
	currentDepth, maxDepth int,
	includeFiles bool,
	excludeMap map[string]bool,
	output *TreeOutput,
	ctx actions.Context,
) (TreeNode, error) {
	info, err := os.Stat(absPath)
	if err != nil {
		return TreeNode{}, err
	}

	name := filepath.Base(absPath)
	if relPath == "" {
		// Root node uses full path as name
		name = filepath.Base(absPath)
		if name == "." {
			name = absPath
		}
	}

	node := TreeNode{
		Name: name,
		Path: relPath,
	}

	if !info.IsDir() {
		// File node
		output.TotalFiles++
		node.Type = "file"
		node.Size = info.Size()
		return node, nil
	}

	// Directory node
	output.TotalDirs++
	node.Type = "directory"

	// Check if we've reached max depth
	if maxDepth >= 0 && currentDepth >= maxDepth {
		return node, nil
	}

	// Read directory contents
	entries, err := os.ReadDir(absPath)
	if err != nil {
		// Log but continue on permission errors
		ctx.GetLogger().Debugf("  Warning: Failed to read directory %s: %v", absPath, err)
		return node, nil
	}

	// Process children
	node.Children = make([]TreeNode, 0)
	for _, entry := range entries {
		entryName := entry.Name()

		// Skip excluded directories
		if excludeMap[entryName] {
			continue
		}

		// Skip files if not including them
		if !entry.IsDir() && !includeFiles {
			continue
		}

		childAbsPath := filepath.Join(absPath, entryName)
		childRelPath := entryName
		if relPath != "" {
			childRelPath = filepath.Join(relPath, entryName)
		}

		childNode, err := h.buildTree(
			childAbsPath,
			childRelPath,
			currentDepth+1,
			maxDepth,
			includeFiles,
			excludeMap,
			output,
			ctx,
		)
		if err != nil {
			ctx.GetLogger().Debugf("  Warning: Failed to process %s: %v", childAbsPath, err)
			continue
		}

		node.Children = append(node.Children, childNode)
	}

	return node, nil
}
