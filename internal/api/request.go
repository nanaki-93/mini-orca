package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

const maxRequestBodyBytes = 1 << 20

var (
	ErrRequestTooLarge = errors.New("request body is too large")
	ErrMultipleJSON    = errors.New("request body must contain one JSON object")
	ErrJSONObject      = errors.New("request body must be a JSON object")
)

// DecodeJSON accepts one bounded JSON object and rejects unknown fields and
// trailing values before a handler reaches its domain operation.
func DecodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	decoder := json.NewDecoder(r.Body)
	var object json.RawMessage
	if err := decoder.Decode(&object); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return ErrRequestTooLarge
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ErrMultipleJSON
	}
	if len(object) == 0 || object[0] != '{' {
		return ErrJSONObject
	}
	strict := json.NewDecoder(bytes.NewReader(object))
	strict.DisallowUnknownFields()
	return strict.Decode(destination)
}

func WriteRequestError(w http.ResponseWriter, err error, message, userMessage string) {
	if errors.Is(err, ErrRequestTooLarge) {
		WriteAppError(w, NewAppError(ErrorBadRequest, ErrRequestTooLarge.Error(), "Request body exceeds the supported size.", http.StatusRequestEntityTooLarge, err))
		return
	}
	WriteAppError(w, BadRequest(message, userMessage, err))
}
