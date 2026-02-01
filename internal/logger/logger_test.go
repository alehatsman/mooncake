package logger

import (
	"testing"
)

func TestConsoleLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  int
		logFunc   func(Logger, string)
		shouldLog bool
	}{
		{
			name:      "debug level logs debug",
			logLevel:  DebugLevel,
			logFunc:   func(l Logger, msg string) { l.Debugf(msg) },
			shouldLog: true,
		},
		{
			name:      "info level skips debug",
			logLevel:  InfoLevel,
			logFunc:   func(l Logger, msg string) { l.Debugf(msg) },
			shouldLog: false,
		},
		{
			name:      "info level logs info",
			logLevel:  InfoLevel,
			logFunc:   func(l Logger, msg string) { l.Infof(msg) },
			shouldLog: true,
		},
		{
			name:      "error level skips info",
			logLevel:  ErrorLevel,
			logFunc:   func(l Logger, msg string) { l.Infof(msg) },
			shouldLog: false,
		},
		{
			name:      "error level logs error",
			logLevel:  ErrorLevel,
			logFunc:   func(l Logger, msg string) { l.Errorf(msg) },
			shouldLog: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testLog := NewTestLogger()
			testLog.SetLogLevel(tt.logLevel)

			tt.logFunc(testLog, "test message")

			logged := testLog.Count() > 0
			if logged != tt.shouldLog {
				t.Errorf("logLevel %d: expected logged=%v, got logged=%v", tt.logLevel, tt.shouldLog, logged)
			}
		})
	}
}

func TestConsoleLogger_SetLogLevelStr(t *testing.T) {
	tests := []struct {
		name      string
		levelStr  string
		wantLevel int
		wantErr   bool
	}{
		{
			name:      "debug string",
			levelStr:  "debug",
			wantLevel: DebugLevel,
			wantErr:   false,
		},
		{
			name:      "info string",
			levelStr:  "info",
			wantLevel: InfoLevel,
			wantErr:   false,
		},
		{
			name:      "error string",
			levelStr:  "error",
			wantLevel: ErrorLevel,
			wantErr:   false,
		},
		{
			name:      "invalid string",
			levelStr:  "invalid",
			wantLevel: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewConsoleLogger(InfoLevel)
			err := logger.SetLogLevelStr(tt.levelStr)

			if (err != nil) != tt.wantErr {
				t.Errorf("SetLogLevelStr() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && logger.logLevel != tt.wantLevel {
				t.Errorf("SetLogLevelStr() level = %v, want %v", logger.logLevel, tt.wantLevel)
			}
		})
	}
}

func TestConsoleLogger_WithPadLevel(t *testing.T) {
	logger := NewLogger(InfoLevel)

	paddedLogger := logger.WithPadLevel(2)

	// Verify it returns a ConsoleLogger with correct pad level
	if consoleLogger, ok := paddedLogger.(*ConsoleLogger); ok {
		if consoleLogger.padLevel != 2 {
			t.Errorf("WithPadLevel(2) padLevel = %v, want 2", consoleLogger.padLevel)
		}
		expectedPad := "    " // 2 levels * 2 spaces each
		if consoleLogger.pad != expectedPad {
			t.Errorf("WithPadLevel(2) pad = %q, want %q", consoleLogger.pad, expectedPad)
		}
	} else {
		t.Errorf("WithPadLevel() returned wrong type: %T", paddedLogger)
	}
}

func TestTestLogger_Capture(t *testing.T) {
	testLog := NewTestLogger()

	testLog.Infof("info message")
	testLog.Debugf("debug message")
	testLog.Errorf("error message")
	testLog.Codef("code message")
	testLog.Textf("text message")

	if testLog.Count() != 5 {
		t.Errorf("Count() = %v, want 5", testLog.Count())
	}

	if !testLog.Contains("info message") {
		t.Error("Contains() should find 'info message'")
	}

	if !testLog.ContainsLevel("DEBUG", "debug message") {
		t.Error("ContainsLevel() should find debug message")
	}

	if testLog.CountLevel("INFO") != 1 {
		t.Errorf("CountLevel(INFO) = %v, want 1", testLog.CountLevel("INFO"))
	}
}

func TestTestLogger_LogLevelFiltering(t *testing.T) {
	testLog := NewTestLogger()
	testLog.SetLogLevel(InfoLevel)

	testLog.Debugf("should not appear")
	testLog.Infof("should appear")
	testLog.Errorf("should also appear")

	if testLog.Count() != 2 {
		t.Errorf("Count() = %v, want 2 (debug should be filtered)", testLog.Count())
	}

	if testLog.Contains("should not appear") {
		t.Error("Debug message should be filtered at InfoLevel")
	}

	if !testLog.Contains("should appear") {
		t.Error("Info message should be logged")
	}
}

func TestTestLogger_Clear(t *testing.T) {
	testLog := NewTestLogger()

	testLog.Infof("message 1")
	testLog.Infof("message 2")

	if testLog.Count() != 2 {
		t.Errorf("Count() before clear = %v, want 2", testLog.Count())
	}

	testLog.Clear()

	if testLog.Count() != 0 {
		t.Errorf("Count() after clear = %v, want 0", testLog.Count())
	}
}

func TestTestLogger_Concurrent(t *testing.T) {
	testLog := NewTestLogger()

	// Test concurrent logging (mutex protection)
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			testLog.Infof("message %d", n)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	if testLog.Count() != 10 {
		t.Errorf("Count() after concurrent writes = %v, want 10", testLog.Count())
	}
}

func TestConsoleLogger_AllMethods(t *testing.T) {
	// Test that ConsoleLogger implements all interface methods
	consoleLogger := NewConsoleLogger(InfoLevel)

	// These calls shouldn't panic
	consoleLogger.SetLogLevel(DebugLevel)
	consoleLogger.Infof("info: %s", "test")
	consoleLogger.Debugf("debug: %s", "test")
	consoleLogger.Errorf("error: %s", "test")
	consoleLogger.Textf("text: %s", "test")
	consoleLogger.Codef("code: %s", "test")
	consoleLogger.Mooncake()

	err := consoleLogger.SetLogLevelStr("info")
	if err != nil {
		t.Errorf("SetLogLevelStr() error = %v", err)
	}

	padded := consoleLogger.WithPadLevel(2)
	if padded == nil {
		t.Error("WithPadLevel() returned nil")
	}
}

func TestConsoleLogger_SetLogLevel(t *testing.T) {
	logger := NewConsoleLogger(InfoLevel)

	logger.SetLogLevel(DebugLevel)
	logger.Debugf("test")

	logger.SetLogLevel(ErrorLevel)
	logger.Errorf("test")
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger(InfoLevel)
	if logger == nil {
		t.Error("NewLogger() returned nil")
	}

	var _ Logger = logger
}

func TestTestLogger_GetLogs(t *testing.T) {
	testLog := NewTestLogger()

	testLog.Infof("message 1")
	testLog.Debugf("message 2")

	logs := testLog.GetLogs()

	if len(logs) != 2 {
		t.Errorf("GetLogs() returned %d logs, want 2", len(logs))
	}

	logs[0].Message = "modified"

	if testLog.Logs[0].Message == "modified" {
		t.Error("GetLogs() should return a copy, not original")
	}
}


func TestTestLogger_SetLogLevelStr(t *testing.T) {
	testLog := NewTestLogger()

	tests := []struct {
		level   string
		wantErr bool
	}{
		{"debug", false},
		{"info", false},
		{"error", false},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			err := testLog.SetLogLevelStr(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetLogLevelStr(%q) error = %v, wantErr %v", tt.level, err, tt.wantErr)
			}
		})
	}
}

func TestTestLogger_Mooncake(t *testing.T) {
	testLog := NewTestLogger()

	testLog.Mooncake()

	if testLog.Count() != 1 {
		t.Errorf("Mooncake() should log one entry, got %d", testLog.Count())
	}

	if !testLog.Contains("Mooncake") {
		t.Error("Mooncake() should log message containing 'Mooncake'")
	}
}

func TestTestLogger_WithPadLevel(t *testing.T) {
	testLog := NewTestLogger()
	testLog.Infof("original message")

	paddedLogger := testLog.WithPadLevel(2)

	// Verify it returns a TestLogger
	if paddedLogger == nil {
		t.Error("WithPadLevel() returned nil")
	}

	// Verify it's a different instance
	if paddedLogger == testLog {
		t.Error("WithPadLevel() should return a different instance")
	}

	// Cast to TestLogger to check padLevel
	if tl, ok := paddedLogger.(*TestLogger); ok {
		if tl.padLevel != 2 {
			t.Errorf("WithPadLevel(2) padLevel = %v, want 2", tl.padLevel)
		}
	} else {
		t.Errorf("WithPadLevel() returned wrong type: %T", paddedLogger)
	}
}

func TestTestLogger_ContainsLevel_NotFound(t *testing.T) {
	testLog := NewTestLogger()

	testLog.Infof("info message")
	testLog.Debugf("debug message")

	// Test case where level doesn't match
	if testLog.ContainsLevel("ERROR", "info message") {
		t.Error("ContainsLevel() should return false for non-matching level")
	}

	// Test case where substring doesn't match
	if testLog.ContainsLevel("INFO", "nonexistent") {
		t.Error("ContainsLevel() should return false for non-matching substring")
	}

	// Test case where both level and substring don't match
	if testLog.ContainsLevel("ERROR", "nonexistent") {
		t.Error("ContainsLevel() should return false when nothing matches")
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		want    int
		wantErr bool
	}{
		{
			name:    "debug lowercase",
			level:   "debug",
			want:    DebugLevel,
			wantErr: false,
		},
		{
			name:    "debug uppercase",
			level:   "DEBUG",
			want:    DebugLevel,
			wantErr: false,
		},
		{
			name:    "info lowercase",
			level:   "info",
			want:    InfoLevel,
			wantErr: false,
		},
		{
			name:    "info uppercase",
			level:   "INFO",
			want:    InfoLevel,
			wantErr: false,
		},
		{
			name:    "error lowercase",
			level:   "error",
			want:    ErrorLevel,
			wantErr: false,
		},
		{
			name:    "error uppercase",
			level:   "ERROR",
			want:    ErrorLevel,
			wantErr: false,
		},
		{
			name:    "invalid level",
			level:   "invalid",
			want:    0,
			wantErr: true,
		},
		{
			name:    "empty string",
			level:   "",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLevel(tt.level)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLevel() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseLogLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConsoleLogger_AddPaddingToLines(t *testing.T) {
	tests := []struct {
		name     string
		padLevel int
		input    string
		expected string
	}{
		{
			name:     "no padding",
			padLevel: 0,
			input:    "single line",
			expected: "single line",
		},
		{
			name:     "single line with padding",
			padLevel: 2,
			input:    "single line",
			expected: "    single line",
		},
		{
			name:     "multiple lines with padding",
			padLevel: 1,
			input:    "line 1\nline 2\nline 3",
			expected: "  line 1\n  line 2\n  line 3",
		},
		{
			name:     "empty lines with padding",
			padLevel: 1,
			input:    "line 1\n\nline 3",
			expected: "  line 1\n  \n  line 3",
		},
		{
			name:     "trailing newline",
			padLevel: 1,
			input:    "line 1\n",
			expected: "  line 1\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := NewConsoleLogger(InfoLevel)
			paddedLogger := logger.WithPadLevel(tt.padLevel).(*ConsoleLogger)

			result := paddedLogger.addPaddingToLines(tt.input)
			if result != tt.expected {
				t.Errorf("addPaddingToLines() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestConsoleLogger_LogStep(t *testing.T) {
	logger := NewConsoleLogger(InfoLevel)

	tests := []struct {
		name string
		info StepInfo
	}{
		{
			name: "running status",
			info: StepInfo{
				Name:       "Test Step",
				Level:      0,
				GlobalStep: 1,
				Status:     StatusRunning,
			},
		},
		{
			name: "success status",
			info: StepInfo{
				Name:       "Test Step",
				Level:      1,
				GlobalStep: 2,
				Status:     StatusSuccess,
			},
		},
		{
			name: "error status",
			info: StepInfo{
				Name:       "Test Step",
				Level:      2,
				GlobalStep: 3,
				Status:     StatusError,
			},
		},
		{
			name: "skipped status",
			info: StepInfo{
				Name:       "Test Step",
				Level:      0,
				GlobalStep: 4,
				Status:     StatusSkipped,
			},
		},
		{
			name: "unknown status",
			info: StepInfo{
				Name:       "Test Step",
				Level:      0,
				GlobalStep: 5,
				Status:     "unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			logger.LogStep(tt.info)
		})
	}

	// Test that it doesn't log at ErrorLevel
	errorLogger := NewConsoleLogger(ErrorLevel)
	errorLogger.LogStep(StepInfo{
		Name:   "Should not log",
		Status: StatusRunning,
	})
}

func TestConsoleLogger_Complete(t *testing.T) {
	logger := NewConsoleLogger(InfoLevel)

	tests := []struct {
		name  string
		stats ExecutionStats
	}{
		{
			name: "successful execution",
			stats: ExecutionStats{
				Duration: 1000000000, // 1 second
				Executed: 5,
				Skipped:  0,
				Failed:   0,
			},
		},
		{
			name: "execution with skipped",
			stats: ExecutionStats{
				Duration: 2000000000, // 2 seconds
				Executed: 3,
				Skipped:  2,
				Failed:   0,
			},
		},
		{
			name: "failed execution",
			stats: ExecutionStats{
				Duration: 500000000, // 0.5 seconds
				Executed: 2,
				Skipped:  1,
				Failed:   1,
			},
		},
		{
			name: "all failed",
			stats: ExecutionStats{
				Duration: 100000000, // 0.1 seconds
				Executed: 0,
				Skipped:  0,
				Failed:   3,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just ensure it doesn't panic
			logger.Complete(tt.stats)
		})
	}
}

func TestTestLogger_LogStep(t *testing.T) {
	testLog := NewTestLogger()

	info := StepInfo{
		Name:       "Test Step",
		Level:      1,
		GlobalStep: 1,
		Status:     StatusRunning,
	}

	testLog.LogStep(info)

	// TestLogger should capture LogStep calls
	if testLog.Count() != 1 {
		t.Errorf("LogStep() should log one entry, got %d", testLog.Count())
	}

	if !testLog.Contains("Test Step") {
		t.Error("LogStep() should log message containing step name")
	}

	if !testLog.Contains("running") {
		t.Error("LogStep() should log message containing status")
	}
}

func TestTestLogger_Complete(t *testing.T) {
	testLog := NewTestLogger()

	stats := ExecutionStats{
		Duration: 1000000000,
		Executed: 5,
		Skipped:  2,
		Failed:   1,
	}

	testLog.Complete(stats)

	// TestLogger should capture Complete calls
	if testLog.Count() != 1 {
		t.Errorf("Complete() should log one entry, got %d", testLog.Count())
	}

	if !testLog.Contains("Complete") {
		t.Error("Complete() should log message containing 'Complete'")
	}
}
