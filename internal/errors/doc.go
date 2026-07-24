// Package errors provides structured error types for Mini-Orca.
//
// Mini-Orca uses a hierarchical error system with specific error codes
// for different failure scenarios. This allows the API handlers to
// return appropriate HTTP status codes and error messages.
//
// # Error Types
//
// - MiniOrcaError: The main error type with a Code, Message, Details, and optional Cause
// - MultiError: Aggregates multiple errors (e.g., from concurrent operations)
//
// # Error Codes
//
// Error codes are organized by category:
//   - Orchestrator: Phase transitions, timeouts, approvals
//   - Agent: Not found, execution failures, timeouts
//   - Model: Provider not configured, request failures, rate limiting
//   - Tool: Not found, execution failures, project type unknown
//   - File: Not found, read/write failures, permission denied
//   - Git: Not initialized, command failures, conflicts
//   - State: Session/plan not found, validation failures, corruption
//   - API: Bad request, not found, internal error, service unavailable
//
// # Helper Functions
//
// - IsErrorCode(err, code): Check if error matches a specific code
// - IsNotFound(err): Check if error indicates a missing resource
// - IsRetryable(err): Check if operation can be retried
// - IsTerminal(err): Check if error is non-recoverable
// - FormatError(err): Format error for API response
// - SafeWrap(err, msg): Wrap error preserving original type
//
// # Example
//
//	err := errors.New(errors.ErrModelRequestFailed, "LLM call failed")
//	if errors.IsRetryable(err) {
//	    // Retry logic
//	}
//
//	response := errors.FormatError(err)
//	// {"error": true, "code": "model_request_failed", "message": "LLM call failed"}
package errors
