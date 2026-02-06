package logger

import (
	"strings"
	"testing"
	"time"
)

func TestNewTUILogger(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	if logger == nil {
		t.Fatal("NewTUILogger() returned nil")
	}

	if logger.buffer == nil {
		t.Error("TUILogger buffer is nil")
	}
	if logger.display == nil {
		t.Error("TUILogger display is nil")
	}
	if logger.animator == nil {
		t.Error("TUILogger animator is nil")
	}
	if logger.done == nil {
		t.Error("TUILogger done channel is nil")
	}

	if logger.logLevel != InfoLevel {
		t.Errorf("logLevel = %d, want %d", logger.logLevel, InfoLevel)
	}
}

func TestTUILogger_SetLogLevel(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	logger.SetLogLevel(DebugLevel)
	if logger.logLevel != DebugLevel {
		t.Errorf("SetLogLevel() failed: logLevel = %d, want %d", logger.logLevel, DebugLevel)
	}

	logger.SetLogLevel(ErrorLevel)
	if logger.logLevel != ErrorLevel {
		t.Errorf("SetLogLevel() failed: logLevel = %d, want %d", logger.logLevel, ErrorLevel)
	}
}

func TestTUILogger_SetLogLevelStr(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	tests := []struct {
		name      string
		levelStr  string
		wantLevel int
		wantErr   bool
	}{
		{"debug", "debug", DebugLevel, false},
		{"info", "info", InfoLevel, false},
		{"error", "error", ErrorLevel, false},
		{"invalid", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := logger.SetLogLevelStr(tt.levelStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetLogLevelStr(%q) error = %v, wantErr %v", tt.levelStr, err, tt.wantErr)
				return
			}

			if !tt.wantErr && logger.logLevel != tt.wantLevel {
				t.Errorf("SetLogLevelStr(%q) level = %d, want %d", tt.levelStr, logger.logLevel, tt.wantLevel)
			}
		})
	}
}

func TestTUILogger_LogStep(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	tests := []struct {
		name   string
		info   StepInfo
		status string
	}{
		{
			name: "running step",
			info: StepInfo{
				Name:       "Install nginx",
				Level:      0,
				GlobalStep: 1,
				Status:     StatusRunning,
			},
			status: StatusRunning,
		},
		{
			name: "success step",
			info: StepInfo{
				Name:       "Configure service",
				Level:      1,
				GlobalStep: 2,
				Status:     StatusSuccess,
			},
			status: StatusSuccess,
		},
		{
			name: "error step",
			info: StepInfo{
				Name:       "Failed step",
				Level:      0,
				GlobalStep: 3,
				Status:     StatusError,
			},
			status: StatusError,
		},
		{
			name: "skipped step",
			info: StepInfo{
				Name:       "Skipped step",
				Level:      0,
				GlobalStep: 4,
				Status:     StatusSkipped,
			},
			status: StatusSkipped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			logger.LogStep(tt.info)

			// Verify step was added to buffer
			snapshot := logger.buffer.GetSnapshot()

			switch tt.status {
			case StatusRunning:
				if snapshot.CurrentStep != tt.info.Name {
					t.Errorf("Running step not set as current: got %q, want %q", snapshot.CurrentStep, tt.info.Name)
				}
			case StatusSuccess, StatusError, StatusSkipped:
				// Should be in history
				found := false
				for _, step := range snapshot.StepHistory {
					if step.Name == tt.info.Name && step.Status == tt.status {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Step %q with status %q not found in history", tt.info.Name, tt.status)
				}
			}
		})
	}
}

func TestTUILogger_LogStep_LogLevel(t *testing.T) {
	logger, err := NewTUILogger(ErrorLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	info := StepInfo{
		Name:       "Test step",
		Level:      0,
		GlobalStep: 1,
		Status:     StatusRunning,
	}

	logger.LogStep(info)

	// At ErrorLevel, step should not be logged
	snapshot := logger.buffer.GetSnapshot()
	if snapshot.CurrentStep != "" {
		t.Error("LogStep() should not log at ErrorLevel")
	}
}

func TestTUILogger_Infof(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	// Infof should be ignored in TUI mode
	logger.Infof("test message")

	// Buffer should not contain the message
	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.DebugMessages) > 0 || len(snapshot.ErrorMessages) > 0 {
		t.Error("Infof() should be ignored in TUI mode")
	}
}

func TestTUILogger_Debugf(t *testing.T) {
	logger, err := NewTUILogger(DebugLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	logger.Debugf("debug message")

	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.DebugMessages) != 1 {
		t.Errorf("Debugf() should add debug message, got %d messages", len(snapshot.DebugMessages))
	}

	if !strings.Contains(snapshot.DebugMessages[0], "debug message") {
		t.Errorf("Debugf() message = %q, want to contain 'debug message'", snapshot.DebugMessages[0])
	}
}

func TestTUILogger_Debugf_LogLevel(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	logger.Debugf("should not appear")

	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.DebugMessages) > 0 {
		t.Error("Debugf() should not log at InfoLevel")
	}
}

func TestTUILogger_Errorf(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	// Set a running step first
	logger.LogStep(StepInfo{
		Name:       "Test step",
		Status:     StatusRunning,
		Level:      0,
		GlobalStep: 1,
	})

	logger.Errorf("error message")

	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.ErrorMessages) != 1 {
		t.Errorf("Errorf() should add error message, got %d messages", len(snapshot.ErrorMessages))
	}

	if !strings.Contains(snapshot.ErrorMessages[0], "error message") {
		t.Errorf("Errorf() message = %q, want to contain 'error message'", snapshot.ErrorMessages[0])
	}

	// Should mark last step as error
	found := false
	for _, step := range snapshot.StepHistory {
		if step.Name == "Test step" && step.Status == StatusError {
			found = true
			break
		}
	}
	if !found {
		t.Error("Errorf() should mark last step as error")
	}
}

func TestTUILogger_Codef(t *testing.T) {
	logger, err := NewTUILogger(DebugLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	logger.Codef("code line 1\ncode line 2")

	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.DebugMessages) < 2 {
		t.Errorf("Codef() should add multiple debug messages, got %d", len(snapshot.DebugMessages))
	}
}

func TestTUILogger_Textf(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	logger.Textf("text message")

	snapshot := logger.buffer.GetSnapshot()
	if len(snapshot.DebugMessages) != 1 {
		t.Errorf("Textf() should add debug message, got %d messages", len(snapshot.DebugMessages))
	}
}

func TestTUILogger_Mooncake(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	// Should not panic
	logger.Mooncake()
}

func TestTUILogger_WithPadLevel(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	paddedLogger := logger.WithPadLevel(2)

	if paddedLogger == nil {
		t.Fatal("WithPadLevel() returned nil")
	}

	tuiLogger, ok := paddedLogger.(*TUILogger)
	if !ok {
		t.Fatalf("WithPadLevel() returned wrong type: %T", paddedLogger)
	}

	if tuiLogger.padLevel != 2 {
		t.Errorf("WithPadLevel(2) padLevel = %d, want 2", tuiLogger.padLevel)
	}

	// Should share same buffer
	if tuiLogger.buffer != logger.buffer {
		t.Error("WithPadLevel() should share the same buffer")
	}
}

func TestTUILogger_Complete(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	stats := ExecutionStats{
		Duration: 1 * time.Second,
		Executed: 10,
		Skipped:  2,
		Failed:   0,
	}

	logger.Complete(stats)

	snapshot := logger.buffer.GetSnapshot()
	if snapshot.Completion == nil {
		t.Fatal("Complete() should set completion stats")
	}

	if snapshot.Completion.Executed != 10 {
		t.Errorf("Completion.Executed = %d, want 10", snapshot.Completion.Executed)
	}
	if snapshot.Completion.Skipped != 2 {
		t.Errorf("Completion.Skipped = %d, want 2", snapshot.Completion.Skipped)
	}
}

func TestTUILogger_SetRedactor(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	redactor := &testRedactor{
		redactFunc: func(text string) string {
			return "REDACTED"
		},
	}

	logger.SetRedactor(redactor)

	// Verify redactor is set
	result := logger.redact("secret")
	if result != "REDACTED" {
		t.Errorf("redact() = %q, want 'REDACTED'", result)
	}
}

func TestTUILogger_Redact(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	tests := []struct {
		name     string
		redactor Redactor
		input    string
		want     string
	}{
		{
			name:     "without redactor",
			redactor: nil,
			input:    "sensitive data",
			want:     "sensitive data",
		},
		{
			name: "with redactor",
			redactor: &testRedactor{
				redactFunc: func(text string) string {
					return strings.ReplaceAll(text, "password", "***")
				},
			},
			input: "password123",
			want:  "***123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger.SetRedactor(tt.redactor)
			got := logger.redact(tt.input)
			if got != tt.want {
				t.Errorf("redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTUILogger_StartStop(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	// Start animation
	logger.Start()

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop animation
	logger.Stop()

	// Should not panic
}

func TestTUILogger_ConcurrentAccess(t *testing.T) {
	logger, err := NewTUILogger(InfoLevel)
	if err != nil {
		t.Fatalf("NewTUILogger() error = %v", err)
	}

	done := make(chan bool)

	// Goroutine 1: Log steps
	go func() {
		for i := 0; i < 50; i++ {
			logger.LogStep(StepInfo{
				Name:       "Step",
				Status:     StatusRunning,
				Level:      0,
				GlobalStep: i,
			})
		}
		done <- true
	}()

	// Goroutine 2: Log errors
	go func() {
		for i := 0; i < 50; i++ {
			logger.Errorf("error %d", i)
		}
		done <- true
	}()

	// Goroutine 3: Log debug
	go func() {
		for i := 0; i < 50; i++ {
			logger.Debugf("debug %d", i)
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 3; i++ {
		<-done
	}

	// Should not panic
}
