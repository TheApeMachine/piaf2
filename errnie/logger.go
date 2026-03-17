package errnie

import (
	"io"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

var logger *Logger

/*
Logger wraps charmbracelet/log with stderr and optional file output.
*/
type Logger struct {
	handle      log.Logger
	traceHandle log.Logger
}

/*
init sets the package-level logger at startup.
*/
func init() {
	logger = New()
}

/*
New creates a Logger that writes to stderr, and to piaf.log in the current directory when possible.
*/
func New() *Logger {
	var out io.Writer = os.Stderr

	wd, err := os.Getwd()

	// Open the log file for appending using an absolute path
	file, err := os.OpenFile(filepath.Join(wd, "piaf.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	if err == nil {
		// Output to both stderr and the file
		out = io.MultiWriter(os.Stderr, file)
	}

	logger := &Logger{
		handle: *log.NewWithOptions(
			out,
			log.Options{
				ReportTimestamp: true,
				ReportCaller:    true,
			},
		),
		// Always initialize traceHandle to a safe fallback (stderr)
		traceHandle: *log.NewWithOptions(
			os.Stderr,
			log.Options{
				ReportTimestamp: true,
				ReportCaller:    true,
				Level:           log.DebugLevel,
			},
		),
	}

	if err == nil {
		logger.traceHandle = *log.NewWithOptions(
			file,
			log.Options{
				ReportTimestamp: true,
				ReportCaller:    true,
				Level:           log.DebugLevel,
			},
		)
	}

	return logger
}

/*
Info logs an info message and key-value pairs to the logger.
*/
func Info(msg string, keyvals ...any) {
	logger.handle.Info(msg, keyvals...)
}

/*
Trace logs a debug message to the trace handle (file when available).
*/
func Trace(msg string, keyvals ...any) {
	logger.traceHandle.Debug(msg, keyvals...)
}

/*
Error logs the error and returns it unchanged.
Skips logging when err is nil.
*/
func Error(err error, keyvals ...any) error {
	if err == nil {
		return nil
	}

	logger.handle.Error(err, keyvals...)

	return err
}

/*
Warn logs a warning message.
*/
func Warn(msg string) {
	logger.handle.Warn(msg)
}

/*
Debug logs a debug message and key-value pairs.
*/
func Debug(msg string, keyvals ...any) {
	logger.handle.Debug(msg, keyvals...)
}

/*
SetLevel changes the logger's minimum output level.
*/
func SetLevel(level log.Level) {
	logger.handle.SetLevel(level)
}
