package logging

import (
	"os"

	"github.com/sirupsen/logrus"
)

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the message passed to Debug, Info, ...
	PanicLevel Level = iota
	// FatalLevel level. Logs and then calls `logger.Exit(1)`. It will exit even if the logging level is set to Panic.
	FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the application.
	InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel
	// TraceLevel level. Designates finer-grained informational events than the Debug.
	TraceLevel
)

// Level represents the logger's logging severity level.
type Level int

// Logger is a wrapper around logrus. It is used by all micro-services for logging purposes.
type Logger struct {
	*logrus.Logger
}

// NewLogger returns a new logger
func NewLogger() *Logger {
	logrusLogger := &logrus.Logger{
		Out: os.Stderr,
		Formatter: &Formatter{
			LogFormat: richLogFormat,
		},
		Level:        logrus.InfoLevel,
		ReportCaller: true,
	}
	return &Logger{logrusLogger}
}
