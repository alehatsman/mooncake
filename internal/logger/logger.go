package logger

import (
	"os"
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

// Logger interface defines the logging contract
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
}

// Fatalf logs an error and exits the program
func Fatalf(logger Logger, format string, v ...interface{}) {
	logger.Errorf(format, v...)
	os.Exit(1)
}
