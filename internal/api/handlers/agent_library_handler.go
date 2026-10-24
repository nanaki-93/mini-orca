package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentLibraryHandler struct{ service *app.Service }

func NewAgentLibraryHandler(service *app.Service) *AgentLibraryHandler {
	return &AgentLibraryHandler{service: service}
}

func (h *AgentLibraryHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r, "cursor", "limit")
	if !ok {
		return
	}
	value, err := h.service.ListArtifacts(r.Context(), query)
	respondArtifact(w, value, err)
}

func (h *AgentLibraryHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ArtifactDetail(r.Context(), query, r.PathValue("artifactID"))
	respondArtifact(w, value, err)
}

func (h *AgentLibraryHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.ArtifactControl
	if !decodeStrictJSON(w, r, &request, "invalid artifact action", "Provide current project identity, export/delete action, artifact version and hash.") {
		return
	}
	value, err := h.service.ControlArtifact(r.Context(), r.PathValue("artifactID"), request)
	respondArtifact(w, value, err)
}

func respondArtifact(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, app.ErrArtifactNotFound):
		api.WriteAppError(w, api.NotFound("artifact unavailable", "Select a retained version owned by this project.", err))
	case errors.Is(err, app.ErrArtifactProtected):
		api.WriteAppError(w, api.Conflict("artifact retained", "Recovery evidence and unfinished run references cannot be deleted.", err))
	case errors.Is(err, app.ErrArtifactConflict):
		api.WriteAppError(w, api.Conflict("artifact selection changed", "Refresh the catalog and review the current version and hash.", err))
	case errors.Is(err, app.ErrArtifactUnavailable):
		api.WriteAppError(w, api.Internal("artifact library unavailable", "The private catalog or export could not be read or written.", err))
	case err != nil:
		if !writeContextError(w, "artifact request", err) {
			writeProjectError(w, "artifact request failed", err)
		}
	default:
		api.WriteJSON(w, http.StatusOK, value)
	}
}
