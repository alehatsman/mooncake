package logger

import (
	"fmt"
	"strings"
	"sync"
)

// LogEntry represents a single log entry
type LogEntry struct {
	Level   string
	Message string
}

// TestLogger implements Logger interface and captures log output for testing
type TestLogger struct {
	mu       sync.Mutex
	Logs     []LogEntry
	logLevel int
	padLevel int
}

// NewTestLogger creates a new TestLogger for use in tests
func NewTestLogger() *TestLogger {
	return &TestLogger{
		Logs:     make([]LogEntry, 0),
		logLevel: DebugLevel, // Capture everything in tests
		padLevel: 0,
	}
}

func (t *TestLogger) Infof(format string, v ...interface{}) {
	if t.logLevel <= InfoLevel {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.Logs = append(t.Logs, LogEntry{"INFO", fmt.Sprintf(format, v...)})
	}
}

func (t *TestLogger) Debugf(format string, v ...interface{}) {
	if t.logLevel <= DebugLevel {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.Logs = append(t.Logs, LogEntry{"DEBUG", fmt.Sprintf(format, v...)})
	}
}

func (t *TestLogger) Errorf(format string, v ...interface{}) {
	if t.logLevel <= ErrorLevel {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.Logs = append(t.Logs, LogEntry{"ERROR", fmt.Sprintf(format, v...)})
	}
}

func (t *TestLogger) Codef(format string, v ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := strings.Split(format, "\n")
	for _, line := range lines {
		t.Logs = append(t.Logs, LogEntry{"CODE", fmt.Sprintf(line, v...)})
	}
}

func (t *TestLogger) Textf(format string, v ...interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = append(t.Logs, LogEntry{"TEXT", fmt.Sprintf(format, v...)})
}

func (t *TestLogger) Mooncake() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = append(t.Logs, LogEntry{"INFO", "Mooncake banner displayed"})
}

func (t *TestLogger) SetLogLevel(logLevel int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logLevel = logLevel
}

func (t *TestLogger) SetLogLevelStr(logLevel string) error {
	switch logLevel {
	case "debug":
		t.SetLogLevel(DebugLevel)
	case "info":
		t.SetLogLevel(InfoLevel)
	case "error":
		t.SetLogLevel(ErrorLevel)
	default:
		return fmt.Errorf("invalid logLevel: %s", logLevel)
	}
	return nil
}

func (t *TestLogger) WithPadLevel(padLevel int) Logger {
	return &TestLogger{
		Logs:     t.Logs, // Share the same log slice
		logLevel: t.logLevel,
		padLevel: padLevel,
	}
}

func (t *TestLogger) LogStep(info StepInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	message := fmt.Sprintf("[%s] %s (level:%d, step:%d)", info.Status, info.Name, info.Level, info.GlobalStep)
	t.Logs = append(t.Logs, LogEntry{"STEP", message})
}

func (t *TestLogger) Complete(stats ExecutionStats) {
	t.mu.Lock()
	defer t.mu.Unlock()
	message := fmt.Sprintf("Completed: %d executed, %d skipped, %d failed, duration: %v",
		stats.Executed, stats.Skipped, stats.Failed, stats.Duration)
	t.Logs = append(t.Logs, LogEntry{"COMPLETE", message})
}

// Helper methods for assertions in tests

// Contains checks if any log message contains the substring
func (t *TestLogger) Contains(substr string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, log := range t.Logs {
		if strings.Contains(log.Message, substr) {
			return true
		}
	}
	return false
}

// ContainsLevel checks if any log at the specified level contains the substring
func (t *TestLogger) ContainsLevel(level, substr string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, log := range t.Logs {
		if log.Level == level && strings.Contains(log.Message, substr) {
			return true
		}
	}
	return false
}

// Count returns the number of log entries
func (t *TestLogger) Count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.Logs)
}

// CountLevel returns the number of log entries at the specified level
func (t *TestLogger) CountLevel(level string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	count := 0
	for _, log := range t.Logs {
		if log.Level == level {
			count++
		}
	}
	return count
}

// Clear removes all log entries
func (t *TestLogger) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Logs = make([]LogEntry, 0)
}

// GetLogs returns a copy of all log entries
func (t *TestLogger) GetLogs() []LogEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	logs := make([]LogEntry, len(t.Logs))
	copy(logs, t.Logs)
	return logs
}
