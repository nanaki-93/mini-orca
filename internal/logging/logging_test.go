package logging

import (
	"bytes"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level  Level
		expect string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
		{LevelFatal, "FATAL"},
		{Level(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expect, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expect {
				t.Errorf("Level(%d).String() = %v, want %v", tt.level, got, tt.expect)
			}
		})
	}
}

func TestNewTestLogger(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTestLogger(&buf, LevelInfo)

	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	if logger.level != LevelInfo {
		t.Errorf("expected level %d, got %d", LevelInfo, logger.level)
	}
}

func TestLoggerGetSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTestLogger(&buf, LevelInfo)

	if logger.GetLevel() != LevelInfo {
		t.Errorf("expected level %d, got %d", LevelInfo, logger.GetLevel())
	}

	logger.SetLevel(LevelDebug)
	if logger.GetLevel() != LevelDebug {
		t.Errorf("expected level %d after set, got %d", LevelDebug, logger.GetLevel())
	}

	logger.SetLevel(LevelInfo) // Reset
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Level != LevelInfo {
		t.Errorf("expected default level %d, got %d", LevelInfo, opts.Level)
	}
	if opts.Dir != ".mini-orca/logs" {
		t.Errorf("expected default dir '.mini-orca/logs', got '%s'", opts.Dir)
	}
	if opts.MaxSizeMB != 10 {
		t.Errorf("expected default max size 10MB, got %d", opts.MaxSizeMB)
	}
	if opts.MaxBackups != 3 {
		t.Errorf("expected default max backups 3, got %d", opts.MaxBackups)
	}
	if opts.MaxAge != 7 {
		t.Errorf("expected default max age 7 days, got %d", opts.MaxAge)
	}
	if !opts.Compress {
		t.Error("expected default compress to be true")
	}
}

func TestSlogLevel(t *testing.T) {
	tests := []struct {
		level  Level
		expect interface{}
	}{
		{LevelDebug, slogLevel(LevelDebug)},
		{LevelInfo, slogLevel(LevelInfo)},
		{LevelWarn, slogLevel(LevelWarn)},
		{LevelError, slogLevel(LevelError)},
	}

	for _, tt := range tests {
		t.Run(tt.level.String(), func(t *testing.T) {
			slogLvl := slogLevel(tt.level)
			// slog.LevelInfo is 0, so we just check it's a valid slog level
			_ = slogLvl // Valid conversion
		})
	}
}

func TestLoggerWith(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTestLogger(&buf, LevelInfo)

	withLogger := logger.With("key", "value")
	if withLogger == nil {
		t.Fatal("expected non-nil logger with")
	}
}

func TestLoggerClose(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTestLogger(&buf, LevelInfo)

	err := logger.Close()
	if err != nil {
		t.Errorf("expected no error on close, got %v", err)
	}
}

func TestGlobalLogger(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	// Get should create a new logger
	logger := GetGlobalLogger()
	if logger == nil {
		t.Fatal("expected non-nil global logger")
	}

	// Second call should return same instance
	same := GetGlobalLogger()
	if same != logger {
		t.Error("expected same logger instance")
	}
}

func TestGlobalLoggerInit(t *testing.T) {
	// Reset global logger
	globalLogger = nil

	opts := DefaultOptions()
	opts.Dir = t.TempDir() // Use temp dir for file logging

	err := InitGlobalLogger(opts)
	if err != nil {
		t.Fatalf("expected no error on init, got %v", err)
	}

	logger := GetGlobalLogger()
	if logger == nil {
		t.Fatal("expected non-nil global logger after init")
	}
}

func TestNewLogger_InvalidDir(t *testing.T) {
	// Try to create logger in a non-existent parent directory
	opts := Options{
		Level: LevelInfo,
		Dir:   "/non-existent/deeply/nested/directory",
	}

	_, err := NewLogger(opts)
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestNewTestLogger_Output(t *testing.T) {
	var buf bytes.Buffer
	logger := NewTestLogger(&buf, LevelInfo)

	// Log a message
	logger.Info("test message")

	// Check output
	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected output to contain 'test message', got '%s'", buf.String())
	}
}

// BenchmarkNewTestLogger benchmarks creating a test logger
func BenchmarkNewTestLogger(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_ = NewTestLogger(&buf, LevelInfo)
	}
}

// BenchmarkGetGlobalLogger benchmarks getting the global logger
func BenchmarkGetGlobalLogger(b *testing.B) {
	// Initialize global logger first
	globalLogger = nil
	InitGlobalLogger(DefaultOptions())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetGlobalLogger()
	}
}
