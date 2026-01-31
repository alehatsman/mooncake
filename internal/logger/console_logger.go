package logger

import (
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"
)

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
		msg := fmt.Sprintf(format, v...)
		msg = l.addPaddingToLines(msg)
		color.White(msg)
	}
}

func (l *ConsoleLogger) Errorf(format string, v ...interface{}) {
	if l.logLevel <= ErrorLevel {
		msg := fmt.Sprintf(format, v...)
		msg = l.addPaddingToLines(msg)
		color.Red(msg)
	}
}

func (l *ConsoleLogger) Debugf(format string, v ...interface{}) {
	if l.logLevel <= DebugLevel {
		msg := fmt.Sprintf(format, v...)
		msg = l.addPaddingToLines(msg)
		color.Yellow(msg)
	}
}

func (l *ConsoleLogger) addPaddingToLines(msg string) string {
	if l.pad == "" {
		return msg
	}
	lines := strings.Split(msg, "\n")
	for i := range lines {
		if lines[i] != "" || i < len(lines)-1 {
			lines[i] = l.pad + lines[i]
		}
	}
	return strings.Join(lines, "\n")
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

func (l *ConsoleLogger) LogStep(info StepInfo) {
	if l.logLevel > InfoLevel {
		return
	}

	indent := strings.Repeat("  ", info.Level)
	var statusIcon string

	switch info.Status {
	case "running":
		statusIcon = "▶"
	case "success":
		statusIcon = "✓"
	case "error":
		statusIcon = "✗"
	case "skipped":
		statusIcon = "⊘"
	default:
		statusIcon = "•"
	}

	// Icon in cyan, text in white
	icon := color.CyanString(statusIcon)
	output := fmt.Sprintf("%s%s %s", indent, icon, info.Name)
	fmt.Println(output)
}
