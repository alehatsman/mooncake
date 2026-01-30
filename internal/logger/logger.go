package logger

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
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

// ConsoleLogger implements Logger interface with colored console output
type ConsoleLogger struct {
	logLevel int
	padLevel int
	pad      string
}

// NewLogger creates a new ConsoleLogger with the specified log level
func NewLogger(logLevel int) Logger {
	return &ConsoleLogger{
		logLevel: logLevel,
		padLevel: 0,
		pad:      "",
	}
}

// NewConsoleLogger creates a ConsoleLogger directly (for type-specific needs)
func NewConsoleLogger(logLevel int) *ConsoleLogger {
	return &ConsoleLogger{
		logLevel: logLevel,
		padLevel: 0,
		pad:      "",
	}
}

func (l *ConsoleLogger) SetLogLevel(logLevel int) {
	l.logLevel = logLevel
}

func (l *ConsoleLogger) SetLogLevelStr(logLevel string) error {
	switch logLevel {
	case "debug":
		l.logLevel = DebugLevel
	case "info":
		l.logLevel = InfoLevel
	case "error":
		l.logLevel = ErrorLevel
	default:
		return errors.New("invalid logLevel")
	}
	return nil
}

func (l *ConsoleLogger) Infof(format string, v ...interface{}) {
	if l.logLevel <= InfoLevel {
		color.White(l.pad+format, v...)
	}
}

func (l *ConsoleLogger) Errorf(format string, v ...interface{}) {
	if l.logLevel <= ErrorLevel {
		color.Red(l.pad+format, v...)
	}
}

func (l *ConsoleLogger) Debugf(format string, v ...interface{}) {
	if l.logLevel <= DebugLevel {
		color.Yellow(l.pad+format, v...)
	}
}

func (l *ConsoleLogger) Textf(format string, v ...interface{}) {
	pad := strings.Repeat(" ", l.padLevel)
	color.WhiteString(pad+format, v...)
}

func (l *ConsoleLogger) Codef(format string, v ...interface{}) {
	lines := strings.Split(format, "\n")

	for _, line := range lines {
		color.Yellow("%s %s", l.pad, fmt.Sprintf(line, v...))
	}
}

func (l *ConsoleLogger) Mooncake() {
	mk1 := color.CyanString(`٩     ۶  `)
	mk2 := color.CyanString(`( ⦿ _ ⦿ )`)
	mk3 := color.CyanString(` ◡   ◡   `)

	fmt.Println()
	fmt.Print(mk1)
	fmt.Println(color.CyanString(`  Mooncake:`))
	fmt.Print(mk2)
	fmt.Println(color.WhiteString("  Lets run some chokity!"))
	fmt.Println(mk3)
	fmt.Println()
}

func (l *ConsoleLogger) WithPadLevel(padLevel int) Logger {
	return &ConsoleLogger{
		logLevel: l.logLevel,
		padLevel: padLevel,
		pad:      strings.Repeat("  ", padLevel),
	}
}

// Fatalf logs an error and exits the program
func Fatalf(logger Logger, format string, v ...interface{}) {
	logger.Errorf(format, v...)
	os.Exit(1)
}
