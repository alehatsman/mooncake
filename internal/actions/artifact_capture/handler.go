// Package artifact_capture implements the artifact.capture action handler.
// Captures file changes with enhanced metadata for LLM agent loops.
package artifact_capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
)

const (
	defaultOutputDir    = "./artifacts"
	defaultFormat       = "both"
	defaultMaxDiffSize  = 1 * 1024 * 1024 // 1MB
	defaultMaxPlanSteps = 20               // Don't embed plans with more than 20 steps
)

// Handler implements the artifact_capture action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "artifact_capture",
		Description:        "Capture file changes with enhanced metadata for LLM agents",
		Category:           actions.CategorySystem,
		SupportsDryRun:     true,
		SupportedPlatforms: []string{}, // All platforms (meta-action)
		RequiresSudo:       false,      // Depends on constituent steps
		ImplementsCheck:    false,      // Meta-action, delegates to steps
	}
}

// Validate validates the artifact_capture action configuration.
func (h *Handler) Validate(step *config.Step) error {
	if step.ArtifactCapture == nil {
		return fmt.Errorf("artifact_capture action requires artifact_capture configuration")
	}

	capture := step.ArtifactCapture
	if capture.Name == "" {
		return fmt.Errorf("artifact_capture name is required")
	}
	if len(capture.Steps) == 0 {
		return fmt.Errorf("artifact_capture requires at least one step")
	}

	// Validate format if specified
	if capture.Format != "" {
		switch capture.Format {
		case "json", "markdown", "both":
			// Valid
		default:
			return fmt.Errorf("artifact_capture format must be one of: json, markdown, both")
		}
	}

	return nil
}

// Execute executes the artifact_capture action.
func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return nil, fmt.Errorf("invalid context type")
	}

	capture := step.ArtifactCapture

	// Set defaults
	outputDir := capture.OutputDir
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	format := capture.Format
	if format == "" {
		format = defaultFormat
	}
	maxDiffSize := capture.MaxDiffSize
	if maxDiffSize == 0 {
		maxDiffSize = defaultMaxDiffSize
	}

	// Create artifact directory
	artifactDir := filepath.Join(outputDir, capture.Name)
	if err := os.MkdirAll(artifactDir, 0755); err != nil { // #nosec G301 -- standard directory permissions
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	ec.Logger.Infof("Capturing artifacts to: %s", artifactDir)

	// Create file change tracker and subscribe to events
	tracker := newFileChangeTracker()
	trackerID := ec.EventPublisher.Subscribe(tracker)
	defer ec.EventPublisher.Unsubscribe(trackerID)

	// Use planner to expand includes, loops, and other plan-time directives
	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}
	expandedSteps, err := planner.ExpandStepsWithContext(capture.Steps, ec.Variables, ec.CurrentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand artifact_capture steps: %w", err)
	}

	ec.Logger.Infof("Executing %d steps within artifact capture", len(expandedSteps))

	// Emit artifact capture start event
	ec.EmitEvent(events.EventArtifactCaptureStart, events.ArtifactCaptureData{
		Name:       capture.Name,
		OutputDir:  artifactDir,
		StepsCount: len(expandedSteps),
	})

	// Execute steps
	anyChanged := false
	startTime := time.Now()
	for i, expandedStep := range expandedSteps {
		ec.Logger.Debugf("Executing artifact capture step %d/%d: %s", i+1, len(expandedSteps), expandedStep.Name)

		if err := executor.ExecuteStep(expandedStep, ec); err != nil {
			return nil, fmt.Errorf("artifact_capture step %d failed: %w", i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}
	duration := time.Since(startTime)

	// Collect file changes from tracker
	fileChanges := tracker.GetFileChanges()

	ec.Logger.Infof("Captured %d file changes", len(fileChanges))

	// Enhance file changes with detailed metadata
	detailedChanges := make([]artifacts.DetailedFileChange, 0, len(fileChanges))
	for _, fc := range fileChanges {
		var beforeContent, afterContent string

		if capture.CaptureContent {
			// Read content if requested
			// Note: before content not available in current implementation
			// Could be enhanced in future with shadow filesystem
			afterContent, _ = readFileContent(fc.Path, maxDiffSize)
		}

		detailed := artifacts.EnhanceFileChange(&fc, beforeContent, afterContent)
		detailedChanges = append(detailedChanges, *detailed)
	}

	// Aggregate statistics
	aggregated := artifacts.AggregateChanges(detailedChanges)

	// Create artifact metadata
	metadata := artifacts.ArtifactMetadata{
		Name:        capture.Name,
		CaptureTime: time.Now().Format(time.RFC3339),
		Summary:     aggregated,
		Files:       detailedChanges,
	}

	// Embed plan if requested (for LLM agent context)
	maxPlanSteps := capture.MaxPlanSteps
	if maxPlanSteps == 0 {
		maxPlanSteps = defaultMaxPlanSteps
	}

	shouldEmbedPlan := true
	if capture.EmbedPlan != nil {
		shouldEmbedPlan = *capture.EmbedPlan
	} else {
		// Auto-decide based on plan size
		shouldEmbedPlan = len(capture.Steps) <= maxPlanSteps
	}

	if shouldEmbedPlan && len(capture.Steps) <= maxPlanSteps {
		metadata.Plan = &artifacts.EmbeddedPlan{
			StepCount:   len(capture.Steps),
			Steps:       capture.Steps,
			InitialVars: ec.Variables, // Capture initial variables for reproducibility
		}
		ec.Logger.Debugf("Embedded plan with %d steps in artifact", len(capture.Steps))
	} else {
		metadata.PlanSummary = fmt.Sprintf("%d steps executed", len(capture.Steps))
		ec.Logger.Debugf("Plan summary: %d steps (not embedded, exceeds max %d)", len(capture.Steps), maxPlanSteps)
	}

	// Write output files based on format
	if format == "json" || format == "both" {
		if err := writeJSONArtifact(artifactDir, metadata); err != nil {
			return nil, fmt.Errorf("failed to write JSON artifact: %w", err)
		}
		ec.Logger.Infof("Written JSON artifact: %s/changes.json", artifactDir)
	}

	if format == "markdown" || format == "both" {
		if err := writeMarkdownSummary(artifactDir, metadata); err != nil {
			return nil, fmt.Errorf("failed to write markdown summary: %w", err)
		}
		ec.Logger.Infof("Written markdown summary: %s/SUMMARY.md", artifactDir)
	}

	// Create result
	result := executor.NewResult()
	result.Changed = anyChanged
	result.Stdout = fmt.Sprintf("Captured %d file changes to %s", len(fileChanges), artifactDir)

	// Emit artifact capture complete event
	ec.EmitEvent(events.EventArtifactCaptureComplete, events.ArtifactCaptureData{
		Name:         capture.Name,
		OutputDir:    artifactDir,
		StepsCount:   len(expandedSteps),
		FilesChanged: len(fileChanges),
		DurationMs:   duration.Milliseconds(),
	})

	ec.Logger.Infof("Artifact capture complete: %d files changed", len(fileChanges))

	return result, nil
}

// DryRun logs what the artifact capture would do.
func (h *Handler) DryRun(ctx actions.Context, step *config.Step) error {
	ec, ok := ctx.(*executor.ExecutionContext)
	if !ok {
		return fmt.Errorf("invalid context type")
	}

	capture := step.ArtifactCapture

	outputDir := capture.OutputDir
	if outputDir == "" {
		outputDir = defaultOutputDir
	}
	artifactDir := filepath.Join(outputDir, capture.Name)

	// Expand steps to get count
	planner, err := plan.NewPlanner()
	if err != nil {
		ec.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (%d steps, planner creation failed)",
			artifactDir, len(capture.Steps))
		return nil
	}
	expandedSteps, err := planner.ExpandStepsWithContext(capture.Steps, ec.Variables, ec.CurrentDir)
	if err != nil {
		ec.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (%d steps, expansion failed)",
			artifactDir, len(capture.Steps))
		return nil
	}

	format := capture.Format
	if format == "" {
		format = defaultFormat
	}

	ec.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (steps: %d, format: %s)",
		artifactDir, len(expandedSteps), format)

	return nil
}

// readFileContent reads file content up to maxSize bytes.
func readFileContent(path string, maxSize int) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from tracked file changes
	if err != nil {
		return "", err
	}

	if len(data) > maxSize {
		data = data[:maxSize]
	}

	return string(data), nil
}

// writeJSONArtifact writes the artifact metadata as JSON.
func writeJSONArtifact(dir string, metadata artifacts.ArtifactMetadata) error {
	path := filepath.Join(dir, "changes.json")

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil { // #nosec G306 -- standard file permissions
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// writeMarkdownSummary writes a human-readable markdown summary.
func writeMarkdownSummary(dir string, metadata artifacts.ArtifactMetadata) error {
	path := filepath.Join(dir, "SUMMARY.md")

	var buf []byte
	buf = append(buf, []byte(fmt.Sprintf("# Artifact Capture: %s\n\n", metadata.Name))...)
	buf = append(buf, []byte(fmt.Sprintf("**Captured:** %s\n\n", metadata.CaptureTime))...)

	// Summary statistics
	buf = append(buf, []byte("## Summary\n\n")...)
	buf = append(buf, []byte(fmt.Sprintf("- **Total Files:** %d\n", metadata.Summary.TotalFiles))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Lines Added:** %d\n", metadata.Summary.TotalLinesAdded))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Lines Removed:** %d\n", metadata.Summary.TotalLinesRemoved))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Total Lines Changed:** %d\n", metadata.Summary.TotalLinesChanged))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Files Created:** %d\n", metadata.Summary.FilesCreated))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Files Updated:** %d\n", metadata.Summary.FilesUpdated))...)
	buf = append(buf, []byte(fmt.Sprintf("- **Files Deleted:** %d\n\n", metadata.Summary.FilesDeleted))...)

	// File type breakdown
	if len(metadata.Summary.FilesByLanguage) > 0 {
		buf = append(buf, []byte("## Files by Language\n\n")...)
		for lang, count := range metadata.Summary.FilesByLanguage {
			buf = append(buf, []byte(fmt.Sprintf("- **%s:** %d\n", lang, count))...)
		}
		buf = append(buf, []byte("\n")...)
	}

	// File type breakdown
	if len(metadata.Summary.FilesByType) > 0 {
		buf = append(buf, []byte("## Files by Type\n\n")...)
		for fileType, count := range metadata.Summary.FilesByType {
			buf = append(buf, []byte(fmt.Sprintf("- **%s:** %d\n", fileType, count))...)
		}
		buf = append(buf, []byte("\n")...)
	}

	// Top changed files
	if len(metadata.Summary.TopChangedFiles) > 0 {
		buf = append(buf, []byte("## Top Changed Files\n\n")...)
		for i, file := range metadata.Summary.TopChangedFiles {
			buf = append(buf, []byte(fmt.Sprintf("%d. **%s** (%d lines, %s)\n",
				i+1, file.Path, file.LinesChanged, file.Operation))...)
		}
		buf = append(buf, []byte("\n")...)
	}

	// Detailed file changes
	buf = append(buf, []byte("## File Changes\n\n")...)
	for _, file := range metadata.Files {
		buf = append(buf, []byte(fmt.Sprintf("### %s\n\n", file.Path))...)
		buf = append(buf, []byte(fmt.Sprintf("- **Operation:** %s\n", file.Operation))...)
		if file.Language != "" && file.Language != "unknown" {
			buf = append(buf, []byte(fmt.Sprintf("- **Language:** %s\n", file.Language))...)
		}
		if file.FileType != "" {
			buf = append(buf, []byte(fmt.Sprintf("- **Type:** %s\n", file.FileType))...)
		}
		buf = append(buf, []byte(fmt.Sprintf("- **Lines Added:** %d\n", file.LinesAdded))...)
		buf = append(buf, []byte(fmt.Sprintf("- **Lines Removed:** %d\n", file.LinesRemoved))...)
		buf = append(buf, []byte(fmt.Sprintf("- **Size After:** %d bytes\n\n", file.SizeAfter))...)
	}

	if err := os.WriteFile(path, buf, 0644); err != nil { // #nosec G306 -- standard file permissions
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}

// fileChangeTracker tracks file changes via events.
type fileChangeTracker struct {
	changes []artifacts.FileChange
}

// newFileChangeTracker creates a new file change tracker.
func newFileChangeTracker() *fileChangeTracker {
	return &fileChangeTracker{
		changes: make([]artifacts.FileChange, 0),
	}
}

// OnEvent handles events and tracks file changes.
func (t *fileChangeTracker) OnEvent(event events.Event) {
	switch event.Type {
	case events.EventFileCreated:
		if data, ok := event.Data.(events.FileOperationData); ok {
			t.changes = append(t.changes, artifacts.FileChange{
				Path:           data.Path,
				Operation:      "created",
				SizeBytes:      data.SizeBytes,
				ChecksumBefore: data.ChecksumBefore,
				ChecksumAfter:  data.ChecksumAfter,
			})
		}

	case events.EventFileUpdated:
		if data, ok := event.Data.(events.FileOperationData); ok {
			t.changes = append(t.changes, artifacts.FileChange{
				Path:           data.Path,
				Operation:      "updated",
				SizeBytes:      data.SizeBytes,
				ChecksumBefore: data.ChecksumBefore,
				ChecksumAfter:  data.ChecksumAfter,
			})
		}

	case events.EventTemplateRender:
		if data, ok := event.Data.(events.TemplateRenderData); ok {
			t.changes = append(t.changes, artifacts.FileChange{
				Path:      data.DestPath,
				Operation: "template",
				SizeBytes: data.SizeBytes,
			})
		}
	}
}

// GetFileChanges returns all tracked file changes.
func (t *fileChangeTracker) GetFileChanges() []artifacts.FileChange {
	return t.changes
}

// Close implements the Subscriber interface (no-op for this tracker).
func (t *fileChangeTracker) Close() {
	// No resources to clean up
}
