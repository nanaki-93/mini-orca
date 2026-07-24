package errors

import (
	"errors"
	"testing"
)

func TestNewError(t *testing.T) {
	err := New(ErrSessionNotFound, "Session not found")

	if err.Code != ErrSessionNotFound {
		t.Errorf("expected code %s, got %s", ErrSessionNotFound, err.Code)
	}
	if err.Message != "Session not found" {
		t.Errorf("expected message 'Session not found', got '%s'", err.Message)
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestNewWithCause(t *testing.T) {
	cause := errors.New("underlying cause")
	err := NewWithCause(ErrModelRequestFailed, "Request failed", cause)

	if err.Cause != cause {
		t.Error("expected cause to be set")
	}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

func TestNewWithDetails(t *testing.T) {
	err := NewWithDetails(ErrBadRequest, "Bad request", "Invalid JSON format")

	if err.Details != "Invalid JSON format" {
		t.Errorf("expected details 'Invalid JSON format', got '%s'", err.Details)
	}
}

func TestIsErrorCode(t *testing.T) {
	err := New(ErrSessionNotFound, "Not found")

	if !IsErrorCode(err, ErrSessionNotFound) {
		t.Error("expected IsErrorCode to return true")
	}
	if IsErrorCode(err, ErrAgentNotFound) {
		t.Error("expected IsErrorCode to return false for different code")
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    bool
	}{
		{"session not found", New(ErrSessionNotFound, "Not found"), true},
		{"agent not found", New(ErrAgentNotFound, "Not found"), true},
		{"file not found", New(ErrFileNotFound, "Not found"), true},
		{"api not found", New(ErrNotFound, "Not found"), true},
		{"other error", New(ErrInternalError, "Error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name    string
		code    ErrorCode
		want    bool
	}{
		{"model request failed", ErrModelRequestFailed, true},
		{"model rate limited", ErrModelRateLimited, true},
		{"tool execution failed", ErrToolExecutionFailed, true},
		{"service unavailable", ErrServiceUnavailable, true},
		{"validation failed", ErrBadRequest, false},
		{"invalid transition", ErrInvalidPhaseTransition, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, "test")
			if got := IsRetryable(err); got != tt.want {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTerminal(t *testing.T) {
	tests := []struct {
		name    string
		code    ErrorCode
		want    bool
	}{
		{"invalid transition", ErrInvalidPhaseTransition, true},
		{"approval rejected", ErrHumanApprovalRejected, true},
		{"state corrupted", ErrStateCorrupted, true},
		{"permission denied", ErrFilePermissionDenied, true},
		{"model error", ErrModelRequestFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.code, "test")
			if got := IsTerminal(err); got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatError(t *testing.T) {
	err := NewWithDetails(ErrBadRequest, "Bad request", "Invalid input")
	result := FormatError(err)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result["error"] != true {
		t.Error("expected error=true")
	}
	if result["code"] != "bad_request" {
		t.Errorf("expected code 'bad_request', got '%v'", result["code"])
	}
	if result["details"] != "Invalid input" {
		t.Errorf("expected details 'Invalid input', got '%v'", result["details"])
	}
}

func TestFormatErrorNil(t *testing.T) {
	result := FormatError(nil)
	if result != nil {
		t.Error("expected nil result for nil error")
	}
}

func TestSafeWrap(t *testing.T) {
	cause := errors.New("original error")
	wrapped := SafeWrap(cause, "wrapped")

	if wrapped == nil {
		t.Fatal("expected non-nil wrapped error")
	}
	if wrapped.Error() == "" {
		t.Error("expected non-empty wrapped error message")
	}
}

func TestSafeWrapNil(t *testing.T) {
	wrapped := SafeWrap(nil, "wrapped")
	if wrapped != nil {
		t.Error("expected nil for nil input")
	}
}

func TestNewValidationError(t *testing.T) {
	err := NewValidationError("name", "Name is required")

	if err.Code != ErrBadRequest {
		t.Errorf("expected code %s, got %s", ErrBadRequest, err.Code)
	}
	if err.Details != "Name is required" {
		t.Errorf("expected details 'Name is required', got '%s'", err.Details)
	}
}

func TestMultiError(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	multi := NewMultiError(err1, err2)

	if multi == nil {
		t.Fatal("expected non-nil MultiError")
	}
	if len(multi.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(multi.Errors))
	}
}

func TestMultiErrorNil(t *testing.T) {
	multi := NewMultiError()
	if multi != nil {
		t.Error("expected nil for empty MultiError")
	}
}

func TestMultiErrorAdd(t *testing.T) {
	multi := &MultiError{}
	multi.Add(errors.New("error 1"))
	multi.Add(nil) // Should be ignored
	multi.Add(errors.New("error 2"))

	if len(multi.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(multi.Errors))
	}
}

func TestMultiErrorIs(t *testing.T) {
	multi := &MultiError{}
	if !multi.Is(&MultiError{}) {
		t.Error("expected Is to return true for MultiError")
	}
}

func TestMultiErrorError(t *testing.T) {
	multi := NewMultiError(errors.New("error 1"), errors.New("error 2"))
	errMsg := multi.Error()

	if errMsg == "" {
		t.Error("expected non-empty error message")
	}
}

func TestErrorCodeString(t *testing.T) {
	// Verify all error codes have string representations
	codes := []ErrorCode{
		ErrOrchestratorNotRunning,
		ErrAgentNotFound,
		ErrModelNotConfigured,
		ErrFileNotFound,
		ErrSessionNotFound,
	}

	for _, code := range codes {
		if len(code) == 0 {
			t.Errorf("error code is empty")
		}
	}
}

// BenchmarkNewError benchmarks creating a new error
func BenchmarkNewError(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New(ErrSessionNotFound, "test error")
	}
}

// BenchmarkFormatError benchmarks formatting an error
func BenchmarkFormatError(b *testing.B) {
	err := NewWithDetails(ErrBadRequest, "Bad request", "Invalid input")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatError(err)
	}
}
