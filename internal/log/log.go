package log

import (
	"fmt"
	"log/slog"
	"os"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// SetVerbose enables debug-level logging.
func SetVerbose(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	}))
}

func Debug(msg string, args ...any) { logger.Debug(msg, args...) }
func Info(msg string, args ...any)  { logger.Info(msg, args...) }
func Warn(msg string, args ...any)  { logger.Warn(msg, args...) }
func Error(msg string, args ...any) { logger.Error(msg, args...) }

// Op wraps an error with operation context.
type Op string

// Err creates a structured error with operation and path context.
type SageError struct {
	Op   Op
	Path string
	Err  error
}

func (e *SageError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s [%s]: %s", e.Op, e.Path, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Err)
}

func (e *SageError) Unwrap() error { return e.Err }

// E creates a new SageError.
func E(op Op, err error) *SageError {
	return &SageError{Op: op, Err: err}
}

// EP creates a new SageError with a path.
func EP(op Op, path string, err error) *SageError {
	return &SageError{Op: op, Path: path, Err: err}
}
