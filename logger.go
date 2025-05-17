package main

import (
	"log"
	"os"
)

const (
	LogLevelDebug = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

type Logger struct {
	debugLogger   *log.Logger
	infoLogger    *log.Logger
	warningLogger *log.Logger
	errorLogger   *log.Logger
	minLevel      int
}

type LoggerInterface interface {
	Debug(format string, v ...interface{})
	Info(format string, v ...interface{})
	Warning(format string, v ...interface{})
	Error(format string, v ...interface{})
}

func NewLogger(minLevel int) *Logger {
	return &Logger{
		debugLogger:   log.New(os.Stdout, "[DEBUG] ", log.Ldate|log.Ltime),
		infoLogger:    log.New(os.Stdout, "[INFO] ", log.Ldate|log.Ltime),
		warningLogger: log.New(os.Stdout, "[WARN] ", log.Ldate|log.Ltime),
		errorLogger:   log.New(os.Stderr, "[ERROR] ", log.Ldate|log.Ltime),
		minLevel:      minLevel,
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.minLevel <= LogLevelDebug {
		l.debugLogger.Printf(format, v...)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	if l.minLevel <= LogLevelInfo {
		l.infoLogger.Printf(format, v...)
	}
}

func (l *Logger) Warning(format string, v ...interface{}) {
	if l.minLevel <= LogLevelWarning {
		l.warningLogger.Printf(format, v...)
	}
}

func (l *Logger) Error(format string, v ...interface{}) {
	if l.minLevel <= LogLevelError {
		l.errorLogger.Printf(format, v...)
	}
}

var _ LoggerInterface = (*Logger)(nil)
