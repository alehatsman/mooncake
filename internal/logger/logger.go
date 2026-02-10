package logger

import (
	"fmt"
	"time"
)

const (
	// DebugLevel logs are typically voluminous, and are usually disabled in
	// production.
	DebugLevel = iota
	// InfoLevel is the default logging priority.
	InfoLevel
	// ErrorLevel logs are high-priority. If an application is running smoothly,
	// it shouldn't generate any error-logLevel logs.
	ErrorLevel
)

// Step status constants used across all logger implementations
const (
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusError   = "error"
	StatusSkipped = "skipped"
)

// ParseLogLevel converts a log level string to its integer constant.
// Valid values are "debug", "info", and "error" (case-insensitive).
// Returns an error if the level string is not recognized.
func ParseLogLevel(level string) (int, error) {
	switch level {
	case "debug", "DEBUG":
		return DebugLevel, nil
	case "info", "INFO":
		return InfoLevel, nil
	case "error", "ERROR":
		return ErrorLevel, nil
	default:
		return 0, fmt.Errorf("invalid log level: %s (valid: debug, info, error)", level)
	}
}

// StepInfo contains structured information about a step execution.
type StepInfo struct {
	Name       string
	Level      int    // Nesting level for indentation
	GlobalStep int    // Cumulative step number
	Status     string // "running", "success", "error", "skipped"
}

// ExecutionStats contains execution statistics.
type ExecutionStats struct {
	Duration time.Duration
	Executed int
	Skipped  int
	Failed   int
}

// Redactor interface for redacting sensitive data in logs.
type Redactor interface {
	Redact(string) string
}

// Logger interface defines the logging contract.
type Logger interface {
	Infof(format string, v ...interface{})
	Debugf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
	Codef(format string, v ...interface{})
	Textf(format string, v ...interface{})
	Mooncake()
	SetLogLevel(logLevel int)
	SetLogLevelStr(logLevel string) error
	WithPadLevel(padLevel int) Logger
	LogStep(info StepInfo)
	Complete(stats ExecutionStats)
	SetRedactor(redactor Redactor)
}
