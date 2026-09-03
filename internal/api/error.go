package api

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorType categorizes a structured transport error.
type ErrorType string

const (
	ErrorInternal     ErrorType = "internal"
	ErrorNotFound     ErrorType = "not_found"
	ErrorBadRequest   ErrorType = "bad_request"
	ErrorUnauthorized ErrorType = "unauthorized"
	ErrorForbidden    ErrorType = "forbidden"
	ErrorConflict     ErrorType = "conflict"
)

// AppError is the API-owned response shape for a domain or transport failure.
type AppError struct {
	Type        ErrorType `json:"type"`
	Message     string    `json:"message"`
	UserMessage string    `json:"user_message"`
	Code        int       `json:"-"`
	Cause       error     `json:"-"`
}

func (e *AppError) Error() string {
	if e.Cause == nil {
		return fmt.Sprintf("%s: %s", e.Type, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Type, e.Message, e.Cause)
}

func (e *AppError) Unwrap() error { return e.Cause }

func NewAppError(kind ErrorType, message, userMessage string, code int, cause error) *AppError {
	return &AppError{Type: kind, Message: message, UserMessage: userMessage, Code: code, Cause: cause}
}

func Internal(message, userMessage string, cause error) *AppError {
	if userMessage == "" {
		userMessage = "An internal server error occurred. Please try again later."
	}
	return NewAppError(ErrorInternal, message, userMessage, http.StatusInternalServerError, cause)
}

func NotFound(message, userMessage string, cause error) *AppError {
	if userMessage == "" {
		userMessage = "The requested resource was not found."
	}
	return NewAppError(ErrorNotFound, message, userMessage, http.StatusNotFound, cause)
}

func BadRequest(message, userMessage string, cause error) *AppError {
	if userMessage == "" {
		userMessage = "The request was invalid."
	}
	return NewAppError(ErrorBadRequest, message, userMessage, http.StatusBadRequest, cause)
}

func Forbidden(message, userMessage string, cause error) *AppError {
	if userMessage == "" {
		userMessage = "You do not have permission to access this resource."
	}
	return NewAppError(ErrorForbidden, message, userMessage, http.StatusForbidden, cause)
}

func Conflict(message, userMessage string, cause error) *AppError {
	if userMessage == "" {
		userMessage = "A conflict occurred with the current state of the resource."
	}
	return NewAppError(ErrorConflict, message, userMessage, http.StatusConflict, cause)
}

func AsAppError(err error) (*AppError, bool) {
	var appError *AppError
	if errors.As(err, &appError) {
		return appError, true
	}
	return nil, false
}
