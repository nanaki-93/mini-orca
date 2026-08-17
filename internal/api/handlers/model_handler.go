package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type ModelHandler struct{ service *app.Service }

func NewModelHandler(service *app.Service) *ModelHandler { return &ModelHandler{service: service} }

// Current handles GET /api/models/current.
func (h *ModelHandler) Current(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	api.WriteJSON(w, http.StatusOK, h.service.EffectiveModel())
}
