package api

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes a JSON response to the HTTP writer.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// WriteError writes an error JSON response to the HTTP writer.
func WriteError(w http.ResponseWriter, status int, message string) {
	errType := ErrorInternal
	switch status {
	case http.StatusNotFound:
		errType = ErrorNotFound
	case http.StatusBadRequest:
		errType = ErrorBadRequest
	case http.StatusUnauthorized:
		errType = ErrorUnauthorized
	case http.StatusForbidden:
		errType = ErrorForbidden
	case http.StatusConflict:
		errType = ErrorConflict
	}

	appErr := NewAppError(errType, message, message, status, nil)
	WriteAppError(w, appErr)
}

// WriteAppError writes a centralized error to the HTTP writer.
func WriteAppError(w http.ResponseWriter, err error) {
	appErr, ok := AsAppError(err)
	if !ok {
		appErr = Internal("unhandled error", "", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Code)
	json.NewEncoder(w).Encode(appErr)
}
