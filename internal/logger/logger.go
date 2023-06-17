package logger

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

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

type Logger struct {
	logLevel int
	padLevel int
	pad      string
}

var logger *Logger
var once sync.Once

// start loggeando
func GetInstance() *Logger {
	once.Do(func() {
		logger = NewLogger(InfoLevel)
	})
	return logger
}

func NewLogger(logLevel int) *Logger {
	return &Logger{
		logLevel: logLevel,
		padLevel: 0,
		pad:      "",
	}
}

func (l *Logger) SetLogLevel(logLevel int) {
	l.logLevel = logLevel
}

func (l *Logger) SetLogLevelStr(logLevel string) error {
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

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.logLevel <= InfoLevel {
		color.White(l.pad+format, v...)
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.logLevel <= ErrorLevel {
		color.Red(l.pad+format, v...)
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.logLevel <= DebugLevel {
		color.Yellow(l.pad+format, v...)
	}
}

func (l *Logger) Textf(format string, v ...interface{}) {
	pad := strings.Repeat(" ", l.padLevel)
	color.WhiteString(pad+format, v...)
}

func padEveryLine(pad string, text string) string {
	return strings.Replace(text, "\n", "\n"+pad, -1)
}

func (l *Logger) Codef(format string, v ...interface{}) {
	lines := strings.Split(format, "\n")

	for _, line := range lines {
		color.Yellow("%s %s", l.pad, fmt.Sprintf(line, v...))
	}
}

func (l *Logger) Mooncake() {
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

func Debugf(format string, v ...interface{}) {
	GetInstance().Debugf(format, v...)
}

func Infof(format string, v ...interface{}) {
	GetInstance().Infof(format, v...)
}

func Errorf(format string, v ...interface{}) {
	GetInstance().Errorf(format, v...)
}

func Fatalf(format string, v ...interface{}) {
	GetInstance().Errorf(format, v...)
	os.Exit(1)
}

func Mooncake() {
	GetInstance().Mooncake()
}

func SetLogLevel(logLevel int) {
	GetInstance().SetLogLevel(logLevel)
}

func WithPadLevel(padLevel int) *Logger {
	return &Logger{
		logLevel: GetInstance().logLevel,
		padLevel: padLevel,
		pad:      strings.Repeat("  ", padLevel),
	}
}

func SetLogLevelStr(logLevel string) error {
	return GetInstance().SetLogLevelStr(logLevel)
}
