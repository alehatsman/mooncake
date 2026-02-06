package logger

import (
	"testing"
	"time"
)

// TestNewTUIBuffer_Creation tests buffer creation
func TestNewTUIBuffer_Creation(t *testing.T) {
	buffer := NewTUIBuffer(10)

	if buffer == nil {
		t.Fatal("NewTUIBuffer returned nil")
	}
}

// TestTUIBuffer_AddStep_Basic tests adding steps
func TestTUIBuffer_AddStep_Basic(t *testing.T) {
	buffer := NewTUIBuffer(5)

	buffer.AddStep(StepEntry{
		Name:      "Test Step",
		Status:    StatusSuccess,
		Level:     0,
		Timestamp: time.Now(),
	})

	snapshot := buffer.GetSnapshot()
	if len(snapshot.StepHistory) != 1 {
		t.Errorf("Expected 1 step, got %d", len(snapshot.StepHistory))
	}
}

// TestTUIBuffer_CircularBuffer tests buffer wraparound
func TestTUIBuffer_CircularBuffer(t *testing.T) {
	buffer := NewTUIBuffer(3)

	// Add more than buffer size
	for i := 0; i < 5; i++ {
		buffer.AddStep(StepEntry{
			Name:   "Step",
			Status: StatusSuccess,
		})
	}

	snapshot := buffer.GetSnapshot()
	if len(snapshot.StepHistory) != 3 {
		t.Errorf("Expected 3 steps (circular), got %d", len(snapshot.StepHistory))
	}
}

// TestTUIBuffer_SetCurrentStep_Updates tests current step updates
func TestTUIBuffer_SetCurrentStep_Updates(t *testing.T) {
	buffer := NewTUIBuffer(3)

	buffer.SetCurrentStep("Processing...", ProgressInfo{Current: 5, Total: 10})

	snapshot := buffer.GetSnapshot()
	if snapshot.CurrentStep != "Processing..." {
		t.Errorf("CurrentStep = %s, want 'Processing...'", snapshot.CurrentStep)
	}
	if snapshot.Progress.Current != 5 || snapshot.Progress.Total != 10 {
		t.Errorf("Progress = %v, want {5 10}", snapshot.Progress)
	}
}

// TestTUIBuffer_Completion tests completion stats
func TestTUIBuffer_Completion(t *testing.T) {
	buffer := NewTUIBuffer(2)

	stats := ExecutionStats{
		Duration: 10 * time.Second,
		Executed: 20,
		Skipped:  3,
		Failed:   1,
	}
	buffer.SetCompletion(stats)

	snapshot := buffer.GetSnapshot()
	if snapshot.Completion == nil {
		t.Fatal("Completion should not be nil")
	}
	if snapshot.Completion.Executed != 20 {
		t.Errorf("Executed = %d, want 20", snapshot.Completion.Executed)
	}
}

// TestTUIBuffer_Messages tests message handling
func TestTUIBuffer_Messages(t *testing.T) {
	buffer := NewTUIBuffer(5)

	buffer.AddDebug("Debug 1")
	buffer.AddDebug("Debug 2")
	buffer.AddError("Error 1")

	snapshot := buffer.GetSnapshot()
	if len(snapshot.DebugMessages) != 2 {
		t.Errorf("DebugMessages = %d, want 2", len(snapshot.DebugMessages))
	}
	if len(snapshot.ErrorMessages) != 1 {
		t.Errorf("ErrorMessages = %d, want 1", len(snapshot.ErrorMessages))
	}
}

// TestTUIBuffer_MessageLimit tests max message limit
func TestTUIBuffer_MessageLimit(t *testing.T) {
	buffer := NewTUIBuffer(5)

	// Add more than max
	for i := 0; i < 10; i++ {
		buffer.AddDebug("Debug")
	}

	snapshot := buffer.GetSnapshot()
	// Should only keep last 5 messages
	if len(snapshot.DebugMessages) > 5 {
		t.Errorf("DebugMessages = %d, should be <= 5", len(snapshot.DebugMessages))
	}
}

// TestDetectTerminal_NoError tests terminal detection doesn't panic
func TestDetectTerminal_NoError(t *testing.T) {
	// Just ensure no panic
	_ = DetectTerminal()
}

// TestIsTUISupported_NoError tests TUI support check doesn't panic
func TestIsTUISupported_NoError(t *testing.T) {
	// Just ensure no panic
	_ = IsTUISupported()
}

// TestGetTerminalSize_NoError tests size detection doesn't panic
func TestGetTerminalSize_NoError(t *testing.T) {
	// Just ensure no panic
	_, _ = GetTerminalSize()
}

// TestLoadEmbeddedFrames_NoError tests frame loading doesn't panic
func TestLoadEmbeddedFrames_NoError(t *testing.T) {
	frames, err := LoadEmbeddedFrames()
	if err != nil {
		t.Fatalf("LoadEmbeddedFrames error: %v", err)
	}
	if frames == nil {
		t.Fatal("Expected non-nil frames")
	}
}
