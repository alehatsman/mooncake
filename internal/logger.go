package internal

import (
	"errors"
	"io"
	"log"
	"os"
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
	// it shouldn't generate any error-level logs.
	ErrorLevel
)

type Logger struct {
	level  int
	l      *log.Logger
	colors bool
	infof  func(format string, v ...interface{})
	debugf func(format string, v ...interface{})
	errorf func(format string, v ...interface{})
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

func NewLogger(level int) *Logger {
	cg := color.New(color.FgGreen)
	cg.EnableColor()

	cy := color.New(color.FgYellow)
	cy.EnableColor()

	cr := color.New(color.FgRed)
	cr.EnableColor()

	return &Logger{
		level:  level,
		l:      log.New(os.Stdout, "", log.LstdFlags),
		colors: true,
		infof:  cg.PrintfFunc(),
		debugf: cy.PrintfFunc(),
		errorf: cr.PrintfFunc(),
	}
}

func (l *Logger) SetOutput(w io.Writer) {
	l.l.SetOutput(w)
}

func (l *Logger) SetLevel(level int) {
	l.level = level
}

func (l *Logger) SetLevelStr(level string) error {
	switch level {
	case "debug":
		l.level = DebugLevel
	case "info":
		l.level = InfoLevel
	case "error":
		l.level = ErrorLevel
	default:
		return errors.New("invalid level")
	}
	return nil
}

func (l *Logger) Infof(format string, v ...interface{}) {
	if l.level <= InfoLevel {
		color.White(format, v...)
	}
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.level <= ErrorLevel {
		color.Red(format, v...)
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.level <= DebugLevel {
		color.Yellow(format, v...)
	}
}
