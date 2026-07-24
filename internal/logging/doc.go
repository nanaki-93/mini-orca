// Package logging provides structured logging for Mini-Orca.
//
// The logging package provides dual output to console and file with
// configurable log levels. It wraps Go's log/slog package with
// Mini-Orca-specific features.
//
// # Log Levels
//
// - DEBUG: Detailed debugging information
// - INFO: General operational messages
// - WARN: Warning conditions that may need attention
// - ERROR: Error conditions that prevent normal operation
// - FATAL: Fatal errors that cause immediate termination
//
// # Configuration
//
// Options control logging behavior:
//
//	opts := logging.DefaultOptions()
//	opts.Level = logging.LevelDebug
//	opts.Dir = ".mini-orca/logs"
//	opts.MaxSizeMB = 10    // Rotate after 10MB
//	opts.MaxBackups = 3    // Keep 3 backup files
//	opts.MaxAge = 7        // Delete logs older than 7 days
//	opts.Compress = true   // Compress rotated logs
//
//	logger, err := logging.NewLogger(opts)
//
// # Global Logger
//
// A global logger is available for convenience:
//
//	logging.InitGlobalLogger(opts)
//	logging.Info("Application started")
//
// # Test Logger
//
// For testing, use NewTestLogger to capture output:
//
//	var buf bytes.Buffer
//	logger := logging.NewTestLogger(&buf, logging.LevelInfo)
//	logger.Info("test message")
//
// # Example
//
//	logger, err := logging.NewLogger(logging.DefaultOptions())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	logger.Info("Processing request", "session_id", session.ID, "phase", session.Phase)
package logging
