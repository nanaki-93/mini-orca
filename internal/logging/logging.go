package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Level represents log levels.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
	LevelFatal
)

// String returns the string representation of the log level.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger provides structured logging with file and console output.
type Logger struct {
	console *slog.Logger
	file    *slog.Logger
	level   Level
	mu      sync.RWMutex
	dir     string
}

// Options configures the logger.
type Options struct {
	Level      Level
	Dir        string
	MaxSizeMB  int // Maximum log file size in MB before rotation
	MaxBackups int // Maximum number of backup files to keep
	MaxAge     int // Maximum number of days to retain old log files
	Compress   bool // Compress rotated log files
}

// DefaultOptions returns default logger options.
func DefaultOptions() Options {
	return Options{
		Level:      LevelInfo,
		Dir:        ".mini-orca/logs",
		MaxSizeMB:  10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   true,
	}
}

// NewLogger creates a new logger with console and optional file output.
func NewLogger(opts Options) (*Logger, error) {
	l := &Logger{
		level: opts.Level,
		dir:   opts.Dir,
	}

	// Console logger
	consoleHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel(opts.Level),
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05"))
			}
			return a
		},
	})
	l.console = slog.New(consoleHandler)

	// File logger
	if opts.Dir != "" {
		if err := os.MkdirAll(opts.Dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		logFile, err := os.OpenFile(filepath.Join(opts.Dir, "mini-orca.log"),
			os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}

		fileHandler := slog.NewTextHandler(logFile, &slog.HandlerOptions{
			Level: slogLevel(opts.Level),
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Value = slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000"))
				}
				return a
			},
		})
		l.file = slog.New(fileHandler)
	}

	return l, nil
}

// NewTestLogger creates a logger that writes to a writer (useful for testing).
func NewTestLogger(w io.Writer, level Level) *Logger {
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{
		Level: slogLevel(level),
	})
	return &Logger{
		console: slog.New(handler),
		level:   level,
	}
}

// slogLevel converts our Level to slog.Level.
func slogLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// With returns a new logger that includes the given key-value pairs.
func (l *Logger) With(keyvals ...any) *slog.Logger {
	return l.console.With(keyvals...)
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, keyvals ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.console.Debug(msg, keyvals...)
	if l.file != nil {
		l.file.Debug(msg, keyvals...)
	}
}

// Info logs an info message.
func (l *Logger) Info(msg string, keyvals ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.console.Info(msg, keyvals...)
	if l.file != nil {
		l.file.Info(msg, keyvals...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, keyvals ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.console.Warn(msg, keyvals...)
	if l.file != nil {
		l.file.Warn(msg, keyvals...)
	}
}

// Error logs an error message.
func (l *Logger) Error(msg string, keyvals ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.console.Error(msg, keyvals...)
	if l.file != nil {
		l.file.Error(msg, keyvals...)
	}
}

// Fatal logs a fatal message and exits.
func (l *Logger) Fatal(msg string, keyvals ...any) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	l.console.Error(msg, keyvals...)
	if l.file != nil {
		l.file.Error(msg, keyvals...)
	}
	os.Exit(1)
}

// SetLevel changes the minimum log level.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel returns the current minimum log level.
func (l *Logger) GetLevel() Level {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// Close closes the logger and releases resources.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	// File logger will be closed by the OS when the process exits
	return nil
}

// Global logger instance
var globalLogger *Logger

// GetGlobalLogger returns the global logger instance.
func GetGlobalLogger() *Logger {
	if globalLogger == nil {
		opts := DefaultOptions()
		globalLogger, _ = NewLogger(opts)
	}
	return globalLogger
}

// InitGlobalLogger initializes the global logger.
func InitGlobalLogger(opts Options) error {
	var err error
	globalLogger, err = NewLogger(opts)
	return err
}

// Convenience functions that use the global logger
func Debug(msg string, keyvals ...any) {
	GetGlobalLogger().Debug(msg, keyvals...)
}

func Info(msg string, keyvals ...any) {
	GetGlobalLogger().Info(msg, keyvals...)
}

func Warn(msg string, keyvals ...any) {
	GetGlobalLogger().Warn(msg, keyvals...)
}

func Error(msg string, keyvals ...any) {
	GetGlobalLogger().Error(msg, keyvals...)
}

func Fatal(msg string, keyvals ...any) {
	GetGlobalLogger().Fatal(msg, keyvals...)
}
