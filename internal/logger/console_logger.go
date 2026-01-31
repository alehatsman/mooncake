// Package logger provides logging interfaces and implementations for mooncake.
package logger

import (
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// ConsoleLogger implements Logger interface with colored console output.
type ConsoleLogger struct {
	logLevel int
	padLevel int
	pad      string
}

// NewLogger creates a new ConsoleLogger with the specified log level.
func NewLogger(logLevel int) Logger {
	return &ConsoleLogger{
		logLevel: logLevel,
		padLevel: 0,
		pad:      "",
	}
}

// NewConsoleLogger creates a ConsoleLogger directly (for type-specific needs).
func NewConsoleLogger(logLevel int) *ConsoleLogger {
	return &ConsoleLogger{
		logLevel: logLevel,
		padLevel: 0,
		pad:      "",
	}
}

// SetLogLevel sets the logging level for the logger.
func (l *ConsoleLogger) SetLogLevel(logLevel int) {
	l.logLevel = logLevel
}

// SetLogLevelStr sets the logging level from a string value.
func (l *ConsoleLogger) SetLogLevelStr(logLevel string) error {
	level, err := ParseLogLevel(logLevel)
	if err != nil {
		return err
	}
	l.logLevel = level
	return nil
}

// Infof logs an informational message.
func (l *ConsoleLogger) Infof(format string, v ...interface{}) {
	if l.logLevel <= InfoLevel {
		msg := fmt.Sprintf(format, v...)
		msg = l.addPaddingToLines(msg)
		color.White(msg)
	}
}

// Errorf logs an error message.
func (l *ConsoleLogger) Errorf(format string, v ...interface{}) {
	if l.logLevel <= ErrorLevel {
		msg := fmt.Sprintf(format, v...)
		msg = l.addPaddingToLines(msg)
		color.Red(msg)
	}
}

// Debugf logs a debug message.
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

// Textf logs a plain text message.
func (l *ConsoleLogger) Textf(format string, v ...interface{}) {
	pad := strings.Repeat(" ", l.padLevel)
	color.WhiteString(pad+format, v...)
}

// Codef logs a code snippet message.
func (l *ConsoleLogger) Codef(format string, v ...interface{}) {
	lines := strings.Split(format, "\n")

	for _, line := range lines {
		color.Yellow("%s %s", l.pad, fmt.Sprintf(line, v...))
	}
}

// Mooncake displays the mooncake banner.
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

// WithPadLevel creates a new logger with the specified padding level.
func (l *ConsoleLogger) WithPadLevel(padLevel int) Logger {
	return &ConsoleLogger{
		logLevel: l.logLevel,
		padLevel: padLevel,
		pad:      strings.Repeat("  ", padLevel),
	}
}

// LogStep logs a step execution with status.
func (l *ConsoleLogger) LogStep(info StepInfo) {
	if l.logLevel > InfoLevel {
		return
	}

	indent := strings.Repeat("  ", info.Level)
	var statusIcon string

	switch info.Status {
	case StatusRunning:
		statusIcon = "▶"
	case StatusSuccess:
		statusIcon = "✓"
	case StatusError:
		statusIcon = "✗"
	case StatusSkipped:
		statusIcon = "⊘"
	default:
		statusIcon = "•"
	}

	// Icon in cyan, text in white
	icon := color.CyanString(statusIcon)
	output := fmt.Sprintf("%s%s %s", indent, icon, info.Name)
	fmt.Println(output)
}

// Complete logs the execution completion summary with statistics.
func (l *ConsoleLogger) Complete(stats ExecutionStats) {
	fmt.Println()
	fmt.Println(color.CyanString("════════════════════════════════════════"))

	if stats.Failed > 0 {
		fmt.Println(color.RedString("✗ Execution failed"))
	} else {
		fmt.Println(color.GreenString("✓ Execution completed successfully"))
	}

	fmt.Println()
	fmt.Printf("  Executed: %s\n", color.GreenString("%d", stats.Executed))
	if stats.Skipped > 0 {
		fmt.Printf("  Skipped:  %s\n", color.YellowString("%d", stats.Skipped))
	}
	if stats.Failed > 0 {
		fmt.Printf("  Failed:   %s\n", color.RedString("%d", stats.Failed))
	}
	fmt.Println()
	fmt.Printf("  Duration: %s\n", color.CyanString("%v", stats.Duration.Round(10*time.Millisecond)))
	fmt.Println(color.CyanString("════════════════════════════════════════"))
	fmt.Println()
}
