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
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	path := r.URL.Query().Get("path")
	var manifest project.ContextManifest
	var err error
	if r.URL.Query().Get("action") == "analyze_file" {
		manifest, err = h.service.AnalysisContextManifest(path)
	} else {
		manifest, err = h.service.ContextManifest(path)
	}
	if err != nil {
		api.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	api.WriteJSON(w, http.StatusOK, manifest)
}
