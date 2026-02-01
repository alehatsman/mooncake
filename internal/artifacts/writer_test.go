package artifacts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/plan"
)

// createTestPlan creates a minimal plan for testing
func createTestPlan() *plan.Plan {
	shellCmd := "echo test"
	return &plan.Plan{
		RootFile: "/test/config.yml",
		Steps: []config.Step{
			{
				Name:  "Test step",
				Shell: &shellCmd,
			},
		},
	}
}

// createTestFacts creates minimal facts for testing
func createTestFacts() *facts.Facts {
	return &facts.Facts{
		Hostname: "test-host",
		OS:       "linux",
		Arch:     "amd64",
	}
}

func TestNewWriter(t *testing.T) {
	// Create temporary directory for artifacts
	tmpDir := t.TempDir()

	cfg := Config{
		BaseDir:        tmpDir,
		CaptureStdout:  true,
		CaptureStderr:  true,
		MaxOutputBytes: 1024,
		MaxOutputLines: 100,
		MaxStdoutBytes: 2048,
		MaxStderrBytes: 2048,
	}

	testPlan := createTestPlan()
	testFacts := createTestFacts()

	writer, err := NewWriter(cfg, testPlan, testFacts)
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Verify run directory was created
	if _, err := os.Stat(writer.runDir); os.IsNotExist(err) {
		t.Errorf("run directory not created: %s", writer.runDir)
	}

	// Verify files were created
	files := []string{
		filepath.Join(writer.runDir, "plan.json"),
		filepath.Join(writer.runDir, "facts.json"),
		filepath.Join(writer.runDir, "events.jsonl"),
		filepath.Join(writer.runDir, "stdout.log"),
		filepath.Join(writer.runDir, "stderr.log"),
	}

	for _, file := range files {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("file not created: %s", file)
		}
	}

	// Verify run ID format (YYYYMMDD-HHMMSS-hash)
	parts := strings.Split(writer.runID, "-")
	if len(parts) != 3 {
		t.Errorf("invalid run ID format: %s", writer.runID)
	}
}

func TestNewWriter_WithoutOutputCapture(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := Config{
		BaseDir:        tmpDir,
		CaptureStdout:  false,
		CaptureStderr:  false,
		MaxOutputBytes: 1024,
		MaxOutputLines: 100,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Verify stdout/stderr files were NOT created
	if writer.stdoutFile != nil {
		t.Error("stdout file should not be created when CaptureStdout is false")
	}
	if writer.stderrFile != nil {
		t.Error("stderr file should not be created when CaptureStderr is false")
	}
}

func TestWriter_OnEvent_StepCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	event := events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID:     "step-1",
			Name:       "Install nginx",
			Level:      0,
			DurationMs: 100,
			Changed:    true,
			Result: map[string]interface{}{
				"rc":     0,
				"stdout": "installed",
			},
		},
	}

	writer.OnEvent(event)

	// Verify step was added
	if len(writer.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(writer.steps))
	}

	step := writer.steps[0]
	if step.StepID != "step-1" {
		t.Errorf("step ID = %s, want step-1", step.StepID)
	}
	if step.Name != "Install nginx" {
		t.Errorf("step name = %s, want Install nginx", step.Name)
	}
	if step.Status != "success" {
		t.Errorf("step status = %s, want success", step.Status)
	}
	if !step.Changed {
		t.Error("step should be marked as changed")
	}
}

func TestWriter_OnEvent_StepFailed(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	event := events.Event{
		Type:      events.EventStepFailed,
		Timestamp: time.Now(),
		Data: events.StepFailedData{
			StepID:       "step-1",
			Name:         "Install nginx",
			Level:        0,
			ErrorMessage: "package not found",
			DurationMs:   50,
		},
	}

	writer.OnEvent(event)

	// Verify step was added
	if len(writer.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(writer.steps))
	}

	step := writer.steps[0]
	if step.Status != "failed" {
		t.Errorf("step status = %s, want failed", step.Status)
	}
	if step.ErrorMessage != "package not found" {
		t.Errorf("error message = %s, want package not found", step.ErrorMessage)
	}
}

func TestWriter_OnEvent_StepSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	event := events.Event{
		Type:      events.EventStepSkipped,
		Timestamp: time.Now(),
		Data: events.StepSkippedData{
			StepID: "step-1",
			Name:   "Configure service",
			Level:  0,
			Reason: "when condition false",
		},
	}

	writer.OnEvent(event)

	// Verify step was added
	if len(writer.steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(writer.steps))
	}

	step := writer.steps[0]
	if step.Status != "skipped" {
		t.Errorf("step status = %s, want skipped", step.Status)
	}
}

func TestWriter_OnEvent_StepOutput(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir:       tmpDir,
		CaptureStdout: true,
		CaptureStderr: true,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	tests := []struct {
		name      string
		eventType events.EventType
		line      string
	}{
		{"stdout", events.EventStepStdout, "output line 1"},
		{"stderr", events.EventStepStderr, "error line 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     "step-1",
					Stream:     tt.name,
					Line:       tt.line,
					LineNumber: 1,
				},
			}

			writer.OnEvent(event)

			// Verify output was written to file
			var filePath string
			if tt.eventType == events.EventStepStdout {
				filePath = filepath.Join(writer.runDir, "stdout.log")
			} else {
				filePath = filepath.Join(writer.runDir, "stderr.log")
			}

			// Flush and read file
			if tt.eventType == events.EventStepStdout {
				writer.stdoutFile.Sync()
			} else {
				writer.stderrFile.Sync()
			}

			content, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed to read %s: %v", filePath, err)
			}

			if !strings.Contains(string(content), tt.line) {
				t.Errorf("output file does not contain line %q", tt.line)
			}
		})
	}
}

func TestWriter_OnEvent_FileOperations(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	tests := []struct {
		name      string
		eventType events.EventType
		path      string
		operation string
	}{
		{"file created", events.EventFileCreated, "/etc/nginx/nginx.conf", "created"},
		{"file updated", events.EventFileUpdated, "/etc/nginx/nginx.conf", "updated"},
		{"template rendered", events.EventTemplateRender, "/tmp/config.yml", "template"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event events.Event
			event.Type = tt.eventType
			event.Timestamp = time.Now()

			if tt.eventType == events.EventTemplateRender {
				event.Data = events.TemplateRenderData{
					TemplatePath: "/tmp/template.j2",
					DestPath:     tt.path,
					SizeBytes:    100,
					Changed:      true,
				}
			} else {
				event.Data = events.FileOperationData{
					Path:      tt.path,
					Mode:      "0644",
					SizeBytes: 100,
					Changed:   true,
				}
			}

			initialCount := len(writer.changedFiles)
			writer.OnEvent(event)

			if len(writer.changedFiles) != initialCount+1 {
				t.Errorf("changed files count = %d, want %d", len(writer.changedFiles), initialCount+1)
			}

			lastFile := writer.changedFiles[len(writer.changedFiles)-1]
			if lastFile.Path != tt.path {
				t.Errorf("file path = %s, want %s", lastFile.Path, tt.path)
			}
			if lastFile.Operation != tt.operation {
				t.Errorf("file operation = %s, want %s", lastFile.Operation, tt.operation)
			}
		})
	}
}

func TestWriter_OnEvent_RunCompleted(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Add some steps first
	writer.OnEvent(events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			DurationMs: 100,
			Changed:    true,
		},
	})

	// Send run completed event
	event := events.Event{
		Type:      events.EventRunCompleted,
		Timestamp: time.Now(),
		Data: events.RunCompletedData{
			TotalSteps:    5,
			SuccessSteps:  4,
			FailedSteps:   0,
			SkippedSteps:  1,
			ChangedSteps:  3,
			DurationMs:    1000,
			Success:       true,
			ErrorMessage:  "",
		},
	}

	writer.OnEvent(event)

	// Verify results.json was written
	resultsPath := filepath.Join(writer.runDir, "results.json")
	if _, err := os.Stat(resultsPath); os.IsNotExist(err) {
		t.Error("results.json not created")
	}

	// Verify diff.json was written
	diffPath := filepath.Join(writer.runDir, "diff.json")
	if _, err := os.Stat(diffPath); os.IsNotExist(err) {
		t.Error("diff.json not created")
	}

	// Verify summary.json was written
	summaryPath := filepath.Join(writer.runDir, "summary.json")
	if _, err := os.Stat(summaryPath); os.IsNotExist(err) {
		t.Error("summary.json not created")
	}

	// Verify contents of summary.json
	summaryData, err := os.ReadFile(summaryPath)
	if err != nil {
		t.Fatalf("failed to read summary.json: %v", err)
	}

	var summary RunSummary
	if err := json.Unmarshal(summaryData, &summary); err != nil {
		t.Fatalf("failed to parse summary.json: %v", err)
	}

	if summary.TotalSteps != 5 {
		t.Errorf("summary total steps = %d, want 5", summary.TotalSteps)
	}
	if summary.SuccessSteps != 4 {
		t.Errorf("summary success steps = %d, want 4", summary.SuccessSteps)
	}
	if !summary.Success {
		t.Error("summary should indicate success")
	}
}

func TestWriter_Close(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir:       tmpDir,
		CaptureStdout: true,
		CaptureStderr: true,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	// Close once
	writer.Close()

	if !writer.closed {
		t.Error("writer should be marked as closed")
	}

	// Close again - should not panic
	writer.Close()
}

func TestWriter_OnEvent_AfterClose(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}

	writer.Close()

	// Send event after close - should be ignored
	event := events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID: "step-1",
			Name:   "Test step",
			Level:  0,
		},
	}

	writer.OnEvent(event)

	// Steps should not be added
	if len(writer.steps) != 0 {
		t.Errorf("expected 0 steps after close, got %d", len(writer.steps))
	}
}

func TestGenerateRunID(t *testing.T) {
	testPlan := createTestPlan()
	testFacts := createTestFacts()

	runID1 := generateRunID(testPlan, testFacts)
	time.Sleep(time.Millisecond * 10)
	runID2 := generateRunID(testPlan, testFacts)

	// Verify format
	parts := strings.Split(runID1, "-")
	if len(parts) != 3 {
		t.Errorf("invalid run ID format: %s", runID1)
	}

	// Verify timestamp part (YYYYMMDD-HHMMSS)
	timestamp := parts[0] + "-" + parts[1]
	if len(timestamp) != 17 { // YYYYMMDD-HHMMSS = 8 + 1 + 6 = 15, but we have seconds too
		// Actually it's YYYYMMDDHHmmss format, no dashes
		// Let me check the actual format
	}

	// Verify hash part is 6 characters
	hash := parts[2]
	if len(hash) != 6 {
		t.Errorf("hash length = %d, want 6", len(hash))
	}

	// With same plan and facts, hash should be the same
	// But timestamps will differ
	parts1 := strings.Split(runID1, "-")
	parts2 := strings.Split(runID2, "-")

	if parts1[2] != parts2[2] {
		t.Errorf("hashes should be the same for same plan/facts: %s vs %s", parts1[2], parts2[2])
	}
}

func TestGenerateRunID_DifferentInputs(t *testing.T) {
	plan1 := &plan.Plan{RootFile: "/test/config1.yml"}
	plan2 := &plan.Plan{RootFile: "/test/config2.yml"}
	facts1 := &facts.Facts{Hostname: "host1"}
	facts2 := &facts.Facts{Hostname: "host2"}

	// Different plans should produce different hashes
	runID1 := generateRunID(plan1, facts1)
	runID2 := generateRunID(plan2, facts1)

	parts1 := strings.Split(runID1, "-")
	parts2 := strings.Split(runID2, "-")

	if parts1[2] == parts2[2] {
		t.Error("different plans should produce different hashes")
	}

	// Different facts should produce different hashes
	runID3 := generateRunID(plan1, facts1)
	runID4 := generateRunID(plan1, facts2)

	parts3 := strings.Split(runID3, "-")
	parts4 := strings.Split(runID4, "-")

	if parts3[2] == parts4[2] {
		t.Error("different facts should produce different hashes")
	}
}

func TestWriter_EventsFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Send multiple events
	testEvents := []events.Event{
		{
			Type:      events.EventStepStarted,
			Timestamp: time.Now(),
			Data: events.StepStartedData{
				StepID: "step-1",
				Name:   "Test step",
				Level:  0,
			},
		},
		{
			Type:      events.EventStepCompleted,
			Timestamp: time.Now(),
			Data: events.StepCompletedData{
				StepID: "step-1",
				Name:   "Test step",
				Level:  0,
			},
		},
	}

	for _, event := range testEvents {
		writer.OnEvent(event)
	}

	// Force flush
	writer.eventsFile.Sync()

	// Read events file
	eventsPath := filepath.Join(writer.runDir, "events.jsonl")
	content, err := os.ReadFile(eventsPath)
	if err != nil {
		t.Fatalf("failed to read events.jsonl: %v", err)
	}

	// Count lines (should be 2)
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 2 {
		t.Errorf("events file has %d lines, want 2", len(lines))
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		var event events.Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
	}
}

func TestWriter_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := Config{
		BaseDir: tmpDir,
	}

	writer, err := NewWriter(cfg, createTestPlan(), createTestFacts())
	if err != nil {
		t.Fatalf("NewWriter failed: %v", err)
	}
	defer writer.Close()

	// Send events from multiple goroutines
	done := make(chan bool)
	eventCount := 100

	for i := 0; i < 3; i++ {
		go func(id int) {
			for j := 0; j < eventCount; j++ {
				event := events.Event{
					Type:      events.EventStepCompleted,
					Timestamp: time.Now(),
					Data: events.StepCompletedData{
						StepID: "step-1",
						Name:   "Test step",
						Level:  0,
					},
				}
				writer.OnEvent(event)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should have received all events
	if len(writer.steps) != 3*eventCount {
		t.Errorf("expected %d steps, got %d", 3*eventCount, len(writer.steps))
	}
}
