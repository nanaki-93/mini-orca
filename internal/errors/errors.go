package errors

import (
	"errors"
	"fmt"
	"strings"
)

// ErrorCode represents a specific error type.
type ErrorCode string

const (
	// Orchestrator errors
	ErrOrchestratorNotRunning ErrorCode = "orchestrator_not_running"
	ErrOrchestratorAlreadyRunning ErrorCode = "orchestrator_already_running"
	ErrInvalidPhaseTransition ErrorCode = "invalid_phase_transition"
	ErrPhaseTimeout ErrorCode = "phase_timeout"
	ErrHumanApprovalRejected ErrorCode = "human_approval_rejected"
	ErrMaxRetriesExceeded ErrorCode = "max_retries_exceeded"

	// Agent errors
	ErrAgentNotFound ErrorCode = "agent_not_found"
	ErrAgentExecutionFailed ErrorCode = "agent_execution_failed"
	ErrAgentTimeout ErrorCode = "agent_timeout"

	// Model errors
	ErrModelNotConfigured ErrorCode = "model_not_configured"
	ErrProviderNotFound ErrorCode = "provider_not_found"
	ErrModelRequestFailed ErrorCode = "model_request_failed"
	ErrModelEmptyResponse ErrorCode = "model_empty_response"
	ErrModelRateLimited ErrorCode = "model_rate_limited"

	// Tool errors
	ErrToolNotFound ErrorCode = "tool_not_found"
	ErrToolExecutionFailed ErrorCode = "tool_execution_failed"
	ErrToolTimeout ErrorCode = "tool_timeout"
	ErrProjectTypeUnknown ErrorCode = "project_type_unknown"

	// File errors
	ErrFileNotFound ErrorCode = "file_not_found"
	ErrFileReadFailed ErrorCode = "file_read_failed"
	ErrFileWriteFailed ErrorCode = "file_write_failed"
	ErrFilePermissionDenied ErrorCode = "file_permission_denied"

	// Git errors
	ErrGitNotInitialized ErrorCode = "git_not_initialized"
	ErrGitCommandFailed ErrorCode = "git_command_failed"
	ErrGitConflict ErrorCode = "git_conflict"

	// State errors
	ErrSessionNotFound ErrorCode = "session_not_found"
	ErrSessionValidationFailed ErrorCode = "session_validation_failed"
	ErrPlanNotFound ErrorCode = "plan_not_found"
	ErrStateCorrupted ErrorCode = "state_corrupted"

	// API errors
	ErrBadRequest ErrorCode = "bad_request"
	ErrNotFound ErrorCode = "not_found"
	ErrInternalError ErrorCode = "internal_error"
	ErrServiceUnavailable ErrorCode = "service_unavailable"
)

// MiniOrcaError represents a structured error with context.
type MiniOrcaError struct {
	Code    ErrorCode
	Message string
	Details string
	Cause   error
}

// Error implements the error interface.
func (e *MiniOrcaError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for use with errors.Is/As.
func (e *MiniOrcaError) Unwrap() error {
	return e.Cause
}

// IsErrorCode checks if the error matches the given code.
func IsErrorCode(err error, code ErrorCode) bool {
	var miniErr *MiniOrcaError
	return err != nil && (errors.As(err, &miniErr) && miniErr.Code == code)
}

// New creates a new MiniOrcaError.
func New(code ErrorCode, message string) *MiniOrcaError {
	return &MiniOrcaError{
		Code:    code,
		Message: message,
	}
}

// NewWithCause creates a new MiniOrcaError with a cause.
func NewWithCause(code ErrorCode, message string, cause error) *MiniOrcaError {
	return &MiniOrcaError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// NewWithDetails creates a new MiniOrcaError with details.
func NewWithDetails(code ErrorCode, message, details string) *MiniOrcaError {
	return &MiniOrcaError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

// IsNotFound checks if the error indicates a not found condition.
func IsNotFound(err error) bool {
	return IsErrorCode(err, ErrSessionNotFound) ||
		IsErrorCode(err, ErrAgentNotFound) ||
		IsErrorCode(err, ErrFileNotFound) ||
		IsErrorCode(err, ErrPlanNotFound) ||
		IsErrorCode(err, ErrNotFound)
}

// IsRetryable checks if the error indicates a retryable condition.
func IsRetryable(err error) bool {
	return IsErrorCode(err, ErrModelRequestFailed) ||
		IsErrorCode(err, ErrModelRateLimited) ||
		IsErrorCode(err, ErrToolExecutionFailed) ||
		IsErrorCode(err, ErrServiceUnavailable)
}

// IsTerminal checks if the error indicates a terminal (non-retryable) condition.
func IsTerminal(err error) bool {
	return IsErrorCode(err, ErrInvalidPhaseTransition) ||
		IsErrorCode(err, ErrHumanApprovalRejected) ||
		IsErrorCode(err, ErrStateCorrupted) ||
		IsErrorCode(err, ErrFilePermissionDenied)
}

// FormatError formats an error for API responses.
func FormatError(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	result := map[string]interface{}{
		"error": true,
	}

	var miniErr *MiniOrcaError
	if errors.As(err, &miniErr) {
		result["code"] = string(miniErr.Code)
		result["message"] = miniErr.Message
		if miniErr.Details != "" {
			result["details"] = miniErr.Details
		}
	} else {
		result["message"] = err.Error()
	}

	return result
}

// SafeWrap wraps an error with additional context, preserving the original error type.
func SafeWrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// NewValidationError creates a validation error.
func NewValidationError(field, message string) *MiniOrcaError {
	return &MiniOrcaError{
		Code:    ErrBadRequest,
		Message: fmt.Sprintf("Validation failed for field '%s'", field),
		Details: message,
	}
}

// MultiError represents multiple errors.
type MultiError struct {
	Errors []error
}

// Error implements the error interface.
func (e *MultiError) Error() string {
	parts := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		parts[i] = err.Error()
	}
	return fmt.Sprintf("multiple errors (%d): %s", len(e.Errors), strings.Join(parts, "; "))
}

// Is implements the errors.Is interface.
func (e *MultiError) Is(target error) bool {
	_, ok := target.(*MultiError)
	return ok
}

// Unwrap returns the underlying errors for use with errors.Is/As.
func (e *MultiError) Unwrap() []error {
	return e.Errors
}

// Add adds an error to the MultiError.
func (e *MultiError) Add(err error) {
	if err != nil {
		e.Errors = append(e.Errors, err)
	}
}

// NewMultiError creates a new MultiError.
func NewMultiError(errs ...error) *MultiError {
	var validErrs []error
	for _, err := range errs {
		if err != nil {
			validErrs = append(validErrs, err)
		}
	}
	if len(validErrs) == 0 {
		return nil
	}
	return &MultiError{Errors: validErrs}
}
