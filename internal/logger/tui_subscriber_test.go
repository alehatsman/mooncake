package logger

import (
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

func TestNewTUISubscriber(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	if sub == nil {
		t.Fatal("NewTUISubscriber() returned nil")
	}

	if sub.buffer == nil {
		t.Error("TUISubscriber buffer is nil")
	}
	if sub.display == nil {
		t.Error("TUISubscriber display is nil")
	}
	if sub.animator == nil {
		t.Error("TUISubscriber animator is nil")
	}
	if sub.done == nil {
		t.Error("TUISubscriber done channel is nil")
	}

	if sub.logLevel != InfoLevel {
		t.Errorf("logLevel = %d, want %d", sub.logLevel, InfoLevel)
	}

	if sub.isRunning {
		t.Error("isRunning should be false initially")
	}
}

func TestTUISubscriber_Start(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	sub.Start()

	if !sub.isRunning {
		t.Error("Start() should set isRunning to true")
	}

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	sub.Stop()
}

func TestTUISubscriber_Start_AlreadyRunning(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	sub.Start()
	if !sub.isRunning {
		t.Error("Start() should set isRunning to true")
	}

	// Start again (should be no-op)
	sub.Start()

	sub.Stop()
}

func TestTUISubscriber_Stop(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	sub.Start()
	time.Sleep(50 * time.Millisecond)

	sub.Stop()

	if sub.isRunning {
		t.Error("Stop() should set isRunning to false")
	}
}

func TestTUISubscriber_Stop_NotRunning(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	// Stop without starting (should be no-op)
	sub.Stop()

	if sub.isRunning {
		t.Error("isRunning should remain false")
	}
}

func TestTUISubscriber_SetRedactor(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	redactor := &testRedactor{
		redactFunc: func(text string) string {
			return "REDACTED"
		},
	}

	sub.SetRedactor(redactor)

	if sub.redactor == nil {
		t.Error("SetRedactor() should set redactor")
	}
}

func TestTUISubscriber_Close(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	sub.Start()
	time.Sleep(50 * time.Millisecond)

	// Close should stop the animation
	sub.Close()

	if sub.isRunning {
		t.Error("Close() should stop animation")
	}
}

func TestTUISubscriber_OnEvent_StepStarted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	event := events.Event{
		Type:      events.EventStepStarted,
		Timestamp: time.Now(),
		Data: events.StepStartedData{
			StepID:     "step-1",
			Name:       "Install nginx",
			Level:      0,
			GlobalStep: 1,
			Action:     "shell",
		},
	}

	sub.OnEvent(event)

	// Verify current step is set
	snapshot := sub.buffer.GetSnapshot()
	if snapshot.CurrentStep != "Install nginx" {
		t.Errorf("CurrentStep = %q, want 'Install nginx'", snapshot.CurrentStep)
	}

	if sub.currentStep == nil {
		t.Error("currentStep should be set")
	}
	if sub.currentStep.Name != "Install nginx" {
		t.Errorf("currentStep.Name = %q, want 'Install nginx'", sub.currentStep.Name)
	}
}

func TestTUISubscriber_OnEvent_StepStarted_LogLevel(t *testing.T) {
	sub, err := NewTUISubscriber(ErrorLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	event := events.Event{
		Type:      events.EventStepStarted,
		Timestamp: time.Now(),
		Data: events.StepStartedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			GlobalStep: 1,
			Action:     "shell",
		},
	}

	sub.OnEvent(event)

	// At ErrorLevel, should not process
	snapshot := sub.buffer.GetSnapshot()
	if snapshot.CurrentStep != "" {
		t.Error("CurrentStep should not be set at ErrorLevel")
	}
}

func TestTUISubscriber_OnEvent_StepCompleted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	// Set a current step first
	sub.currentStep = &StepInfo{
		Name:   "Test step",
		Status: StatusRunning,
	}

	event := events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			DurationMs: 100,
			Changed:    true,
		},
	}

	sub.OnEvent(event)

	// Verify step is in history
	snapshot := sub.buffer.GetSnapshot()
	found := false
	for _, step := range snapshot.StepHistory {
		if step.Name == "Test step" && step.Status == StatusSuccess {
			found = true
			break
		}
	}
	if !found {
		t.Error("Completed step should be in history with success status")
	}

	// Current step should be cleared
	if sub.currentStep != nil {
		t.Error("currentStep should be cleared after completion")
	}
}

func TestTUISubscriber_OnEvent_StepFailed(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	event := events.Event{
		Type:      events.EventStepFailed,
		Timestamp: time.Now(),
		Data: events.StepFailedData{
			StepID:       "step-1",
			Name:         "Failed step",
			Level:        0,
			ErrorMessage: "command failed",
			DurationMs:   50,
		},
	}

	sub.OnEvent(event)

	// Verify step is in history with error status
	snapshot := sub.buffer.GetSnapshot()
	found := false
	for _, step := range snapshot.StepHistory {
		if step.Name == "Failed step" && step.Status == StatusError {
			found = true
			break
		}
	}
	if !found {
		t.Error("Failed step should be in history with error status")
	}

	// Current step should be cleared
	if sub.currentStep != nil {
		t.Error("currentStep should be cleared after failure")
	}
}

func TestTUISubscriber_OnEvent_StepSkipped(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	event := events.Event{
		Type:      events.EventStepSkipped,
		Timestamp: time.Now(),
		Data: events.StepSkippedData{
			StepID: "step-1",
			Name:   "Skipped step",
			Level:  0,
			Reason: "condition not met",
		},
	}

	sub.OnEvent(event)

	// Verify step is in history with skipped status
	snapshot := sub.buffer.GetSnapshot()
	found := false
	for _, step := range snapshot.StepHistory {
		if step.Name == "Skipped step" && step.Status == StatusSkipped {
			found = true
			break
		}
	}
	if !found {
		t.Error("Skipped step should be in history with skipped status")
	}
}

func TestTUISubscriber_OnEvent_RunCompleted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	// Set a current step
	sub.currentStep = &StepInfo{
		Name:   "Current step",
		Status: StatusRunning,
	}

	event := events.Event{
		Type:      events.EventRunCompleted,
		Timestamp: time.Now(),
		Data: events.RunCompletedData{
			TotalSteps:   10,
			SuccessSteps: 8,
			FailedSteps:  0,
			SkippedSteps: 2,
			ChangedSteps: 5,
			DurationMs:   1000,
			Success:      true,
		},
	}

	sub.OnEvent(event)

	// Current step should be cleared
	if sub.currentStep != nil {
		t.Error("currentStep should be cleared on run completion")
	}

	// Completion stats should be set
	snapshot := sub.buffer.GetSnapshot()
	if snapshot.Completion == nil {
		t.Fatal("Completion stats should be set")
	}

	if snapshot.Completion.Executed != 8 {
		t.Errorf("Completion.Executed = %d, want 8", snapshot.Completion.Executed)
	}
	if snapshot.Completion.Skipped != 2 {
		t.Errorf("Completion.Skipped = %d, want 2", snapshot.Completion.Skipped)
	}
	if snapshot.Completion.Failed != 0 {
		t.Errorf("Completion.Failed = %d, want 0", snapshot.Completion.Failed)
	}
}

func TestTUISubscriber_OnEvent_InvalidData(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	tests := []struct {
		name      string
		eventType events.EventType
		data      interface{}
	}{
		{
			name:      "step started with wrong data",
			eventType: events.EventStepStarted,
			data:      "invalid",
		},
		{
			name:      "step completed with wrong data",
			eventType: events.EventStepCompleted,
			data:      123,
		},
		{
			name:      "step failed with wrong data",
			eventType: events.EventStepFailed,
			data:      true,
		},
		{
			name:      "step skipped with wrong data",
			eventType: events.EventStepSkipped,
			data:      nil,
		},
		{
			name:      "run completed with wrong data",
			eventType: events.EventRunCompleted,
			data:      []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			// Should not panic with invalid data
			sub.OnEvent(event)
		})
	}
}

func TestTUISubscriber_OnEvent_UnknownEventType(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	// Test with various unknown event types
	unknownEvents := []events.EventType{
		events.EventFileCreated,
		events.EventFileUpdated,
		events.EventDirCreated,
		events.EventTemplateRender,
		events.EventVarsSet,
		events.EventRunStarted,
	}

	for _, eventType := range unknownEvents {
		event := events.Event{
			Type:      eventType,
			Timestamp: time.Now(),
			Data:      nil,
		}

		// Should not panic
		sub.OnEvent(event)
	}
}

func TestTUISubscriber_ConcurrentAccess(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	done := make(chan bool)

	// Goroutine 1: Send events
	go func() {
		for i := 0; i < 50; i++ {
			event := events.Event{
				Type:      events.EventStepStarted,
				Timestamp: time.Now(),
				Data: events.StepStartedData{
					StepID:     "step",
					Name:       "Step",
					Level:      0,
					GlobalStep: i,
					Action:     "shell",
				},
			}
			sub.OnEvent(event)
		}
		done <- true
	}()

	// Goroutine 2: Set redactor
	go func() {
		for i := 0; i < 50; i++ {
			sub.SetRedactor(&testRedactor{})
		}
		done <- true
	}()

	// Goroutine 3: Start/Stop
	go func() {
		for i := 0; i < 5; i++ {
			sub.Start()
			time.Sleep(10 * time.Millisecond)
			sub.Stop()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not panic
}

func TestTUISubscriber_HandleStepStarted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	data := events.StepStartedData{
		StepID:     "step-1",
		Name:       "Test step",
		Level:      1,
		GlobalStep: 5,
		Action:     "shell",
	}

	sub.handleStepStarted(data)

	// Verify buffer state
	snapshot := sub.buffer.GetSnapshot()
	if snapshot.CurrentStep != "Test step" {
		t.Errorf("CurrentStep = %q, want 'Test step'", snapshot.CurrentStep)
	}

	if snapshot.Progress.Current != 5 {
		t.Errorf("Progress.Current = %d, want 5", snapshot.Progress.Current)
	}

	// Verify current step tracking
	if sub.currentStep == nil {
		t.Fatal("currentStep should be set")
	}
	if sub.currentStep.Name != "Test step" {
		t.Errorf("currentStep.Name = %q, want 'Test step'", sub.currentStep.Name)
	}
	if sub.currentStep.Level != 1 {
		t.Errorf("currentStep.Level = %d, want 1", sub.currentStep.Level)
	}
	if sub.currentStep.Status != StatusRunning {
		t.Errorf("currentStep.Status = %q, want %q", sub.currentStep.Status, StatusRunning)
	}
}

func TestTUISubscriber_HandleStepCompleted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	data := events.StepCompletedData{
		StepID:     "step-1",
		Name:       "Completed step",
		Level:      0,
		DurationMs: 150,
		Changed:    true,
	}

	sub.handleStepCompleted(data)

	// Verify step in history
	snapshot := sub.buffer.GetSnapshot()
	if len(snapshot.StepHistory) != 1 {
		t.Fatalf("StepHistory length = %d, want 1", len(snapshot.StepHistory))
	}

	step := snapshot.StepHistory[0]
	if step.Name != "Completed step" {
		t.Errorf("Step name = %q, want 'Completed step'", step.Name)
	}
	if step.Status != StatusSuccess {
		t.Errorf("Step status = %q, want %q", step.Status, StatusSuccess)
	}
}

func TestTUISubscriber_HandleStepFailed(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	data := events.StepFailedData{
		StepID:       "step-1",
		Name:         "Failed step",
		Level:        1,
		ErrorMessage: "error occurred",
		DurationMs:   75,
	}

	sub.handleStepFailed(data)

	// Verify step in history
	snapshot := sub.buffer.GetSnapshot()
	if len(snapshot.StepHistory) != 1 {
		t.Fatalf("StepHistory length = %d, want 1", len(snapshot.StepHistory))
	}

	step := snapshot.StepHistory[0]
	if step.Name != "Failed step" {
		t.Errorf("Step name = %q, want 'Failed step'", step.Name)
	}
	if step.Status != StatusError {
		t.Errorf("Step status = %q, want %q", step.Status, StatusError)
	}
	if step.Level != 1 {
		t.Errorf("Step level = %d, want 1", step.Level)
	}
}

func TestTUISubscriber_HandleStepSkipped(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	data := events.StepSkippedData{
		StepID: "step-1",
		Name:   "Skipped step",
		Level:  2,
		Reason: "when: false",
	}

	sub.handleStepSkipped(data)

	// Verify step in history
	snapshot := sub.buffer.GetSnapshot()
	if len(snapshot.StepHistory) != 1 {
		t.Fatalf("StepHistory length = %d, want 1", len(snapshot.StepHistory))
	}

	step := snapshot.StepHistory[0]
	if step.Name != "Skipped step" {
		t.Errorf("Step name = %q, want 'Skipped step'", step.Name)
	}
	if step.Status != StatusSkipped {
		t.Errorf("Step status = %q, want %q", step.Status, StatusSkipped)
	}
	if step.Level != 2 {
		t.Errorf("Step level = %d, want 2", step.Level)
	}
}

func TestTUISubscriber_HandleRunCompleted(t *testing.T) {
	sub, err := NewTUISubscriber(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUISubscriber() error = %v", err)
	}

	// Set current step to verify it gets cleared
	sub.currentStep = &StepInfo{Name: "Test"}

	data := events.RunCompletedData{
		TotalSteps:   15,
		SuccessSteps: 12,
		FailedSteps:  1,
		SkippedSteps: 2,
		ChangedSteps: 8,
		DurationMs:   2500,
		Success:      false,
	}

	sub.handleRunCompleted(data)

	// Verify current step cleared
	if sub.currentStep != nil {
		t.Error("currentStep should be cleared")
	}

	// Verify completion stats
	snapshot := sub.buffer.GetSnapshot()
	if snapshot.Completion == nil {
		t.Fatal("Completion should be set")
	}

	if snapshot.Completion.Executed != 12 {
		t.Errorf("Executed = %d, want 12", snapshot.Completion.Executed)
	}
	if snapshot.Completion.Skipped != 2 {
		t.Errorf("Skipped = %d, want 2", snapshot.Completion.Skipped)
	}
	if snapshot.Completion.Failed != 1 {
		t.Errorf("Failed = %d, want 1", snapshot.Completion.Failed)
	}

	expectedDuration := 2500 * time.Millisecond
	if snapshot.Completion.Duration != expectedDuration {
		t.Errorf("Duration = %v, want %v", snapshot.Completion.Duration, expectedDuration)
	}
}
