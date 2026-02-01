// Package artifacts handles persistent storage of execution run data.
package artifacts

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/plan"
)

// Config holds configuration for artifact writer.
type Config struct {
	BaseDir           string // Base directory for artifacts (e.g., ".mooncake")
	CaptureStdout     bool   // Whether to capture full stdout
	CaptureStderr     bool   // Whether to capture full stderr
	MaxOutputBytes    int    // Max bytes per step in results.json
	MaxOutputLines    int    // Max lines per step in results.json
	MaxStdoutBytes    int    // Max bytes for stdout.log
	MaxStderrBytes    int    // Max bytes for stderr.log
}

// Writer writes execution artifacts to disk.
type Writer struct {
	config  Config
	runID   string
	runDir  string
	mu      sync.Mutex
	closed  bool

	// File handles
	eventsFile *os.File
	stdoutFile *os.File
	stderrFile *os.File

	// Accumulators
	steps        []StepResult
	changedFiles []FileChange
	runStartTime time.Time
}

// StepResult holds result data for a single step.
type StepResult struct {
	StepID       string                 `json:"step_id"`
	Name         string                 `json:"name"`
	Action       string                 `json:"action"`
	Level        int                    `json:"level"`
	DurationMs   int64                  `json:"duration_ms"`
	Changed      bool                   `json:"changed"`
	Status       string                 `json:"status"` // "success", "failed", "skipped"
	ErrorMessage string                 `json:"error_message,omitempty"`
	OutputLines  int                    `json:"output_lines,omitempty"`
	OutputBytes  int                    `json:"output_bytes,omitempty"`
	Truncated    bool                   `json:"truncated,omitempty"`
	Result       map[string]interface{} `json:"result,omitempty"`
}

// FileChange records a file that was created or modified.
type FileChange struct {
	Path      string `json:"path"`
	Operation string `json:"operation"` // "created", "updated", "template"
	SizeBytes int64  `json:"size_bytes"`
}

// RunSummary contains overall run information.
type RunSummary struct {
	RunID        string    `json:"run_id"`
	RootFile     string    `json:"root_file"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	DurationMs   int64     `json:"duration_ms"`
	TotalSteps   int       `json:"total_steps"`
	SuccessSteps int       `json:"success_steps"`
	FailedSteps  int       `json:"failed_steps"`
	SkippedSteps int       `json:"skipped_steps"`
	ChangedSteps int       `json:"changed_steps"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// NewWriter creates a new artifact writer.
func NewWriter(cfg Config, planData *plan.Plan, systemFacts *facts.Facts) (*Writer, error) {
	// Generate run ID
	runID := generateRunID(planData, systemFacts)

	// Create run directory
	runDir := filepath.Join(cfg.BaseDir, "runs", runID)
	// #nosec G301 -- Artifact directory permissions are intentionally readable
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run directory: %w", err)
	}

	w := &Writer{
		config:       cfg,
		runID:        runID,
		runDir:       runDir,
		steps:        make([]StepResult, 0),
		changedFiles: make([]FileChange, 0),
		runStartTime: time.Now(),
	}

	// Write plan
	if err := w.writePlan(planData); err != nil {
		return nil, fmt.Errorf("failed to write plan: %w", err)
	}

	// Write facts
	if err := w.writeFacts(systemFacts); err != nil {
		return nil, fmt.Errorf("failed to write facts: %w", err)
	}

	// Open events file
	eventsPath := filepath.Join(runDir, "events.jsonl")
	// #nosec G304 -- Artifact file path is intentional functionality
	eventsFile, err := os.Create(eventsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create events file: %w", err)
	}
	w.eventsFile = eventsFile

	// Open stdout/stderr files if capturing
	if cfg.CaptureStdout {
		stdoutPath := filepath.Join(runDir, "stdout.log")
		// #nosec G304 -- Artifact file path is intentional functionality
		stdoutFile, err := os.Create(stdoutPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout file: %w", err)
		}
		w.stdoutFile = stdoutFile
	}

	if cfg.CaptureStderr {
		stderrPath := filepath.Join(runDir, "stderr.log")
		// #nosec G304 -- Artifact file path is intentional functionality
		stderrFile, err := os.Create(stderrPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create stderr file: %w", err)
		}
		w.stderrFile = stderrFile
	}

	return w, nil
}

// OnEvent processes events and writes to artifacts.
func (w *Writer) OnEvent(event events.Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}

	// Write event to events.jsonl
	if w.eventsFile != nil {
		if err := json.NewEncoder(w.eventsFile).Encode(event); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing event: %v\n", err)
		}
	}

	// Process event type
	switch event.Type {
	case events.EventStepCompleted:
		if data, ok := event.Data.(events.StepCompletedData); ok {
			w.steps = append(w.steps, StepResult{
				StepID:     data.StepID,
				Name:       data.Name,
				Action:     "", // Not available in event
				Level:      data.Level,
				DurationMs: data.DurationMs,
				Changed:    data.Changed,
				Status:     "success",
				Result:     data.Result,
			})
		}

	case events.EventStepFailed:
		if data, ok := event.Data.(events.StepFailedData); ok {
			w.steps = append(w.steps, StepResult{
				StepID:       data.StepID,
				Name:         data.Name,
				Level:        data.Level,
				DurationMs:   data.DurationMs,
				Status:       "failed",
				ErrorMessage: data.ErrorMessage,
			})
		}

	case events.EventStepSkipped:
		if data, ok := event.Data.(events.StepSkippedData); ok {
			w.steps = append(w.steps, StepResult{
				StepID: data.StepID,
				Name:   data.Name,
				Level:  data.Level,
				Status: "skipped",
			})
		}

	case events.EventStepStdout:
		if data, ok := event.Data.(events.StepOutputData); ok {
			if w.stdoutFile != nil {
				if _, err := fmt.Fprintf(w.stdoutFile, "[%s] %s\n", data.StepID, data.Line); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing stdout: %v\n", err)
				}
			}
		}

	case events.EventStepStderr:
		if data, ok := event.Data.(events.StepOutputData); ok {
			if w.stderrFile != nil {
				if _, err := fmt.Fprintf(w.stderrFile, "[%s] %s\n", data.StepID, data.Line); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing stderr: %v\n", err)
				}
			}
		}

	case events.EventFileCreated:
		if data, ok := event.Data.(events.FileOperationData); ok {
			w.changedFiles = append(w.changedFiles, FileChange{
				Path:      data.Path,
				Operation: "created",
				SizeBytes: data.SizeBytes,
			})
		}

	case events.EventFileUpdated:
		if data, ok := event.Data.(events.FileOperationData); ok {
			w.changedFiles = append(w.changedFiles, FileChange{
				Path:      data.Path,
				Operation: "updated",
				SizeBytes: data.SizeBytes,
			})
		}

	case events.EventTemplateRender:
		if data, ok := event.Data.(events.TemplateRenderData); ok {
			w.changedFiles = append(w.changedFiles, FileChange{
				Path:      data.DestPath,
				Operation: "template",
				SizeBytes: data.SizeBytes,
			})
		}

	case events.EventRunCompleted:
		if data, ok := event.Data.(events.RunCompletedData); ok {
			if err := w.writeResults(data); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing results: %v\n", err)
			}
			if err := w.writeChangedFiles(); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing changed files: %v\n", err)
			}
			if err := w.writeSummary(data); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing summary: %v\n", err)
			}
		}
	}
}

// Close closes all open files.
func (w *Writer) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return
	}

	if w.eventsFile != nil {
		if err := w.eventsFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing events file: %v\n", err)
		}
	}
	if w.stdoutFile != nil {
		if err := w.stdoutFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing stdout file: %v\n", err)
		}
	}
	if w.stderrFile != nil {
		if err := w.stderrFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing stderr file: %v\n", err)
		}
	}

	w.closed = true
}

// writePlan writes the plan to plan.json.
func (w *Writer) writePlan(planData *plan.Plan) error {
	planPath := filepath.Join(w.runDir, "plan.json")
	return plan.SavePlanToFile(planData, planPath)
}

// writeFacts writes system facts to facts.json.
func (w *Writer) writeFacts(systemFacts *facts.Facts) error {
	factsPath := filepath.Join(w.runDir, "facts.json")
	// #nosec G304 -- Artifact file path is intentional functionality
	factsFile, err := os.Create(factsPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := factsFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing facts file: %v\n", err)
		}
	}()

	encoder := json.NewEncoder(factsFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(systemFacts)
}

// writeResults writes step results to results.json.
func (w *Writer) writeResults(runData events.RunCompletedData) error {
	resultsPath := filepath.Join(w.runDir, "results.json")
	// #nosec G304 -- Artifact file path is intentional functionality
	resultsFile, err := os.Create(resultsPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := resultsFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing results file: %v\n", err)
		}
	}()

	results := map[string]interface{}{
		"run_id":        w.runID,
		"total_steps":   runData.TotalSteps,
		"success_steps": runData.SuccessSteps,
		"failed_steps":  runData.FailedSteps,
		"skipped_steps": runData.SkippedSteps,
		"steps":         w.steps,
	}

	encoder := json.NewEncoder(resultsFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

// writeChangedFiles writes changed files to diff.json.
func (w *Writer) writeChangedFiles() error {
	diffPath := filepath.Join(w.runDir, "diff.json")
	// #nosec G304 -- Artifact file path is intentional functionality
	diffFile, err := os.Create(diffPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := diffFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing diff file: %v\n", err)
		}
	}()

	diff := map[string]interface{}{
		"changed_files": w.changedFiles,
		"total":         len(w.changedFiles),
	}

	encoder := json.NewEncoder(diffFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(diff)
}

// writeSummary writes run summary to summary.json.
func (w *Writer) writeSummary(runData events.RunCompletedData) error {
	summaryPath := filepath.Join(w.runDir, "summary.json")
	// #nosec G304 -- Artifact file path is intentional functionality
	summaryFile, err := os.Create(summaryPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := summaryFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing summary file: %v\n", err)
		}
	}()

	summary := RunSummary{
		RunID:        w.runID,
		StartTime:    w.runStartTime,
		EndTime:      time.Now(),
		DurationMs:   runData.DurationMs,
		TotalSteps:   runData.TotalSteps,
		SuccessSteps: runData.SuccessSteps,
		FailedSteps:  runData.FailedSteps,
		SkippedSteps: runData.SkippedSteps,
		ChangedSteps: runData.ChangedSteps,
		Success:      runData.Success,
		ErrorMessage: runData.ErrorMessage,
	}

	encoder := json.NewEncoder(summaryFile)
	encoder.SetIndent("", "  ")
	return encoder.Encode(summary)
}

// generateRunID creates a unique run ID.
func generateRunID(planData *plan.Plan, systemFacts *facts.Facts) string {
	timestamp := time.Now().Format("20060102-150405")
	hash := sha256.New()
	hash.Write([]byte(planData.RootFile))
	hash.Write([]byte(systemFacts.Hostname))
	shortHash := fmt.Sprintf("%x", hash.Sum(nil))[:6]
	return fmt.Sprintf("%s-%s", timestamp, shortHash)
}
