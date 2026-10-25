package logging

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

var (
	// Default logger is JSON to stdout at Info level
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
)

// DefaultSensitiveKeys is a list of common sensitive keys to filter
var DefaultSensitiveKeys = []string{
	"password", "secret", "token", "api_key", "apikey", "authorization", "auth", "credential",
}

// Config defines the configuration for the logger
type Config struct {
	Level         string
	Format        string // "json" or "text"
	Output        io.Writer
	Filename      string // If set, logs will be written to this file
	SensitiveKeys []string
}

// Init initializes the global logger with the given configuration
func Init(cfg Config) {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	sensitiveKeys := cfg.SensitiveKeys
	if len(sensitiveKeys) == 0 {
		sensitiveKeys = DefaultSensitiveKeys
	}

	opts := &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			return redactAttr(a, sensitiveKeys)
		},
	}

	output := cfg.Output
	if output == nil {
		if cfg.Filename != "" {
			f, err := os.OpenFile(cfg.Filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				// Fallback to stdout if file cannot be opened
				output = os.Stdout
				slog.Error("failed to open log file, falling back to stdout", "error", err, "filename", cfg.Filename)
			} else {
				output = f
			}
		} else {
			output = os.Stdout
		}
	}

	var handler slog.Handler
	if strings.ToLower(cfg.Format) == "text" {
		handler = slog.NewTextHandler(output, opts)
	} else {
		handler = slog.NewJSONHandler(output, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// Info logs a message at info level
func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

// Error logs a message at error level
func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

func redactAttr(a slog.Attr, sensitiveKeys []string) slog.Attr {
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		redacted := make([]slog.Attr, len(attrs))
		for i, child := range attrs {
			redacted[i] = redactAttr(child, sensitiveKeys)
		}
		return slog.Attr{Key: a.Key, Value: slog.GroupValue(redacted...)}
	}
	lowerKey := strings.ToLower(a.Key)
	for _, key := range sensitiveKeys {
		if strings.EqualFold(a.Key, key) || strings.Contains(lowerKey, strings.ToLower(key)) {
			return slog.String(a.Key, "[REDACTED]")
		}
	}
	return a
}
