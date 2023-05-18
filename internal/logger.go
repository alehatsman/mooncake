package internal

import (
	"github.com/fatih/color"
)

type Colors struct {
	Red   func(...interface{})
	Green func(...interface{})
	Grey  func(...interface{})
}

type Logger struct {
	colors *Colors
}

func (l *Logger) Info(v ...interface{}) {
	l.colors.Green(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.colors.Red(v...)
}

func (l *Logger) Debug(v ...interface{}) {
	l.colors.Grey(v...)
}

func NewLogger() *Logger {
	return &Logger{
		colors: &Colors{
			Red:   color.New(color.FgRed).PrintlnFunc(),
			Green: color.New(color.FgGreen).PrintlnFunc(),
			Grey:  color.New(color.FgHiBlack).PrintlnFunc(),
		},
	}
}
