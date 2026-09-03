package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

type ContextHandler struct{ service *app.Service }

func NewContextHandler(service *app.Service) *ContextHandler {
	return &ContextHandler{service: service}
}

// Preview handles GET /api/projects/current/context?path=... for the inspector.
func (h *ContextHandler) Preview(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	var manifest project.ContextManifest
	var err error
	switch r.URL.Query().Get("action") {
	case "", "fix":
		manifest, err = h.service.ContextManifest(path)
	case "analyze_file":
		manifest, err = h.service.AnalysisContextManifest(path)
	default:
		api.WriteAppError(w, api.BadRequest("invalid context action", "Choose fix or analyze_file.", nil))
		return
	}
	if err != nil {
		api.WriteAppError(w, api.BadRequest("context preview failed", "Choose an eligible project file and try again.", err))
		return
	}
	api.WriteJSON(w, http.StatusOK, manifest)
}
