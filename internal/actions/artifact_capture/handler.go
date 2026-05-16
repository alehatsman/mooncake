// Package artifact_capture implements the artifact.capture action handler.
// Captures file changes with enhanced metadata for LLM agent loops.
package artifact_capture

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/plan"
)

const (
	defaultOutputDir    = "./artifacts"
	defaultFormat       = "both"
	defaultMaxDiffSize  = 1 * 1024 * 1024 // 1MB
	defaultMaxPlanSteps = 20              // Don't embed plans with more than 20 steps
)

// Handler implements the artifact_capture action handler.
type Handler struct{}

func init() {
	actions.Register(&Handler{})
}

// Metadata returns the action metadata.
func (h *Handler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:               "artifact.capture",
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

	ec.Svc.Logger.Infof("Capturing artifacts to: %s", artifactDir)

	// Create file change tracker and subscribe to events
	tracker := newFileChangeTracker()
	trackerID := ec.Svc.EventPublisher.Subscribe(tracker)
	defer ec.Svc.EventPublisher.Unsubscribe(trackerID)

	// Use planner to expand includes, loops, and other plan-time directives
	planner, err := plan.NewPlanner()
	if err != nil {
		return nil, fmt.Errorf("failed to create planner: %w", err)
	}
	expandedSteps, err := planner.ExpandStepsWithContext(capture.Steps, ec.GetVariables(), ec.CurrentDir)
	if err != nil {
		return nil, fmt.Errorf("failed to expand artifact_capture steps: %w", err)
	}

	ec.Svc.Logger.Infof("Executing %d steps within artifact capture", len(expandedSteps))

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
		ec.Svc.Logger.Debugf("Executing artifact capture step %d/%d: %s", i+1, len(expandedSteps), expandedStep.Name)

		if err := executor.ExecuteStep(expandedStep, ec); err != nil {
			return nil, fmt.Errorf("artifact_capture step %d failed: %w", i+1, err)
		}

		// Track if any step changed
		if ec.CurrentResult != nil && ec.CurrentResult.Changed {
			anyChanged = true
		}
	}
	duration := time.Since(startTime)

	// Drain in-flight events before reading the tracker — the channel
	// publisher dispatches OnEvent on a per-subscriber goroutine, so any
	// EventFileCreated/EventFileUpdated emitted by an inner file.write
	// or text.* step may still be queued when ExecuteStep returns. Without
	// this Flush, GetFileChanges() saw an empty slice and the artifact
	// reported "0 file changes" despite the writes succeeding.
	ec.Svc.EventPublisher.Flush()

	// Collect file changes from tracker
	fileChanges := tracker.GetFileChanges()

	ec.Svc.Logger.Infof("Captured %d file changes", len(fileChanges))

	// Enhance file changes with detailed metadata
	detailedChanges := make([]artifacts.DetailedFileChange, 0, len(fileChanges))
	for _, fc := range fileChanges {
		var beforeContent, afterContent string

		if capture.CaptureContent {
			// Issue #27: prefer the in-event before/after bytes when the
			// upstream handler plumbed them through — that gives us a true
			// pre-write snapshot rather than a post-write disk re-read.
			// Falls back to reading from disk for handlers that don't carry
			// content through events yet (other file actions are follow-ups).
			if fc.ContentBefore != nil {
				if len(fc.ContentBefore) > maxDiffSize {
					beforeContent = string(fc.ContentBefore[:maxDiffSize])
				} else {
					beforeContent = string(fc.ContentBefore)
				}
			}
			if fc.ContentAfter != nil {
				if len(fc.ContentAfter) > maxDiffSize {
					afterContent = string(fc.ContentAfter[:maxDiffSize])
				} else {
					afterContent = string(fc.ContentAfter)
				}
			} else {
				afterContent, _ = readFileContent(fc.Path, maxDiffSize)
			}
		}

		detailed := artifacts.EnhanceFileChange(&fc, beforeContent, afterContent)
		if capture.CaptureContent {
			detailed.ContentBefore = beforeContent
			detailed.ContentAfter = afterContent
		}
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

	var shouldEmbedPlan bool
	if capture.EmbedPlan != nil {
		shouldEmbedPlan = *capture.EmbedPlan
	} else {
		// Auto-decide based on plan size
		shouldEmbedPlan = len(capture.Steps) <= maxPlanSteps
	}

	if shouldEmbedPlan && len(capture.Steps) <= maxPlanSteps {
		// Capture only user-set variables, not facts/metrics — agents want
		// to see what the playbook configured, not the 100+ entries of
		// system inventory (cpu_flags, etc.) that bloated changes.json.
		// The planner pre-merges facts into Scope.User for template
		// availability, so subtract the facts keyset to leave only the
		// playbook-supplied vars. Facts can be re-derived on replay; user
		// vars are what the run actually parameterized on.
		metadata.Plan = &artifacts.EmbeddedPlan{
			StepCount:   len(capture.Steps),
			Steps:       capture.Steps,
			InitialVars: userVarsOnly(ec.Scope.User),
		}
		ec.Svc.Logger.Debugf("Embedded plan with %d steps in artifact", len(capture.Steps))
	} else {
		metadata.PlanSummary = fmt.Sprintf("%d steps executed", len(capture.Steps))
		ec.Svc.Logger.Debugf("Plan summary: %d steps (not embedded, exceeds max %d)", len(capture.Steps), maxPlanSteps)
	}

	// Write output files based on format
	if format == "json" || format == "both" {
		if err := writeJSONArtifact(artifactDir, metadata); err != nil {
			return nil, fmt.Errorf("failed to write JSON artifact: %w", err)
		}
		ec.Svc.Logger.Infof("Written JSON artifact: %s/changes.json", artifactDir)
	}

	if format == "markdown" || format == "both" {
		if err := writeMarkdownSummary(artifactDir, metadata); err != nil {
			return nil, fmt.Errorf("failed to write markdown summary: %w", err)
		}
		ec.Svc.Logger.Infof("Written markdown summary: %s/SUMMARY.md", artifactDir)
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

	ec.Svc.Logger.Infof("Artifact capture complete: %d files changed", len(fileChanges))

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
		ec.Svc.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (%d steps, planner creation failed)",
			artifactDir, len(capture.Steps))
		return nil
	}
	expandedSteps, err := planner.ExpandStepsWithContext(capture.Steps, ec.GetVariables(), ec.CurrentDir)
	if err != nil {
		ec.Svc.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (%d steps, expansion failed)",
			artifactDir, len(capture.Steps))
		return nil
	}

	format := capture.Format
	if format == "" {
		format = defaultFormat
	}

	ec.Svc.Logger.Infof("  [DRY-RUN] Would capture artifacts to '%s' (steps: %d, format: %s)",
		artifactDir, len(expandedSteps), format)

	return nil
}

// readFileContent reads up to maxSize bytes from path without loading the
// full file into memory first. Peak allocation is bounded at maxSize+1
// regardless of file size (fixes F041).
func readFileContent(path string, maxSize int) (string, error) {
	// #nosec G304 -- path comes from tracked file changes
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck // read-only file; close error is not actionable
	// Read one byte past the cap so truncation is exact rather than guessed.
	data, err := io.ReadAll(io.LimitReader(f, int64(maxSize)+1))
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

// userVarsOnly returns a copy of scopeUser with any key that comes from
// the system facts table removed. The planner pre-merges facts into
// Scope.User so templates can read them as plain `{{ foo }}` lookups; for
// the artifact metadata we want only the playbook-supplied parameters.
//
// The execute-time scope's typed Facts pointer is nil (it's populated by
// the CLI's step / facts paths, not by ExecutePlan), so we snapshot the
// fact keyset directly from facts.Collect() to know what to filter.
func userVarsOnly(scopeUser map[string]interface{}) map[string]interface{} {
	if len(scopeUser) == 0 {
		return nil
	}
	factKeys := map[string]struct{}{}
	for k := range facts.Collect().ToMap() {
		factKeys[k] = struct{}{}
	}
	out := make(map[string]interface{}, len(scopeUser))
	for k, v := range scopeUser {
		if _, isFact := factKeys[k]; isFact {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
				SizeBefore:     data.SizeBefore,
				ChecksumBefore: data.ChecksumBefore,
				ChecksumAfter:  data.ChecksumAfter,
				ContentBefore:  data.ContentBefore,
				ContentAfter:   data.ContentAfter,
			})
		}

	case events.EventFileUpdated:
		if data, ok := event.Data.(events.FileOperationData); ok {
			t.changes = append(t.changes, artifacts.FileChange{
				Path:           data.Path,
				Operation:      "updated",
				SizeBytes:      data.SizeBytes,
				SizeBefore:     data.SizeBefore,
				ChecksumBefore: data.ChecksumBefore,
				ChecksumAfter:  data.ChecksumAfter,
				ContentBefore:  data.ContentBefore,
				ContentAfter:   data.ContentAfter,
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

// Run is the Spec 16 unified entry point. Artifact capture re-runs its
// inner steps every execute (it's not idempotent — capturing is the
// point), so plan mode always reports would-change with the inner-step
// count for context.
//
// Deep inspection (running each inner step's plan-mode prediction
// recursively) is a follow-up; today the plan output for an
// artifact_capture step shows the step's name and inner count, while
// the inner steps themselves are not inspected.
func (h *Handler) Run(ctx actions.Context, step *config.Step) (actions.Result, error) {
	if ctx.Mode() != actions.ModePlan {
		return h.Execute(ctx, step)
	}

	c := step.ArtifactCapture
	result := executor.NewResult()
	result.Checkable = true
	result.WouldChange = true
	result.Reason = fmt.Sprintf("would capture artifact %q (%d inner step(s))", c.Name, len(c.Steps))
	return result, nil
}
