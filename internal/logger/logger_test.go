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
