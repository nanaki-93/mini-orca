package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Type defines the category of the error
type Type string

const (
	TypeInternal     Type = "internal"
	TypeNotFound     Type = "not_found"
	TypeBadRequest   Type = "bad_request"
	TypeUnauthorized Type = "unauthorized"
	TypeForbidden    Type = "forbidden"
	TypeConflict     Type = "conflict"
)

// Error represents a centralized error structure
type Error struct {
	Type        Type   `json:"type"`
	Message     string `json:"message"`      // Developer message
	UserMessage string `json:"user_message"` // User-friendly message
	Code        int    `json:"-"`            // HTTP Status Code
	Cause       error  `json:"-"`
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (cause: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// New creates a new Error
func New(errType Type, message string, userMessage string, code int, cause error) *Error {
	return &Error{
		Type:        errType,
		Message:     message,
		UserMessage: userMessage,
		Code:        code,
		Cause:       cause,
	}
}

// Internal creates a new internal error
func Internal(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "An internal server error occurred. Please try again later."
	}
	return New(TypeInternal, message, userMessage, http.StatusInternalServerError, cause)
}

// NotFound creates a new not found error
func NotFound(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "The requested resource was not found."
	}
	return New(TypeNotFound, message, userMessage, http.StatusNotFound, cause)
}

// BadRequest creates a new bad request error
func BadRequest(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "The request was invalid."
	}
	return New(TypeBadRequest, message, userMessage, http.StatusBadRequest, cause)
}

// Unauthorized creates a new unauthorized error
func Unauthorized(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "Authentication is required to access this resource."
	}
	return New(TypeUnauthorized, message, userMessage, http.StatusUnauthorized, cause)
}

// Forbidden creates a new forbidden error
func Forbidden(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "You do not have permission to access this resource."
	}
	return New(TypeForbidden, message, userMessage, http.StatusForbidden, cause)
}

// Conflict creates a new conflict error
func Conflict(message string, userMessage string, cause error) *Error {
	if userMessage == "" {
		userMessage = "A conflict occurred with the current state of the resource."
	}
	return New(TypeConflict, message, userMessage, http.StatusConflict, cause)
}

// Wrap wraps an existing error with context
func Wrap(cause error, errType Type, message string, userMessage string) *Error {
	if cause == nil {
		return nil
	}

	// If it's already an *Error, we wrap it while keeping the type and code if not provided
	var e *Error
	if errors.As(cause, &e) {
		newErr := &Error{
			Type:        e.Type,
			Message:     e.Message,
			UserMessage: e.UserMessage,
			Code:        e.Code,
			Cause:       cause,
		}
		if errType != "" {
			newErr.Type = errType
		}
		if message != "" {
			newErr.Message = message
		}
		if userMessage != "" {
			newErr.UserMessage = userMessage
		}
		return newErr
	}

	code := http.StatusInternalServerError
	if errType == "" {
		errType = TypeInternal
	}

	switch errType {
	case TypeNotFound:
		code = http.StatusNotFound
	case TypeBadRequest:
		code = http.StatusBadRequest
	case TypeUnauthorized:
		code = http.StatusUnauthorized
	case TypeForbidden:
		code = http.StatusForbidden
	case TypeConflict:
		code = http.StatusConflict
	}

	return New(errType, message, userMessage, code, cause)
}

// AsError attempts to convert an error to *Error
func AsError(err error) (*Error, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e, true
	}
	return nil, false
}
