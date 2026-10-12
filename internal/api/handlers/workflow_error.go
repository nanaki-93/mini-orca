package handlers

import (
	"context"
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func writeWorkflowError(w http.ResponseWriter, operation string, err error) {
	if errors.Is(err, project.ErrRevisionConflict) {
		api.WriteError(w, http.StatusConflict, "project or file changed; refresh the proposal")
		return
	}
	if errors.Is(err, context.Canceled) {
		api.WriteError(w, http.StatusRequestTimeout, "generation canceled")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) {
		api.WriteError(w, http.StatusGatewayTimeout, "generation timed out")
		return
	}
	api.WriteAppError(w, api.BadRequest(operation, "Review the request and try again.", err))
}
