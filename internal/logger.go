package internal

import (
	"io"
	"log"
	"os"
)

type Logger struct {
	info *log.Logger
	err  *log.Logger
}

func NewLoggerWithHandles(infoHandle, errHandle io.Writer) *Logger {
	return &Logger{
		info: log.New(infoHandle, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		err:  log.New(errHandle, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}

func (l *Logger) Info(v ...interface{}) {
	l.info.Println(v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.err.Println(v...)
}

func NewLogger() *Logger {
	return &Logger{
		info: log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile),
		err:  log.New(os.Stdout, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile),
	}
}
