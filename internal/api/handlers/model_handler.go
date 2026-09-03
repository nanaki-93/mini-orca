package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type ModelHandler struct{ service *app.Service }

func NewModelHandler(service *app.Service) *ModelHandler { return &ModelHandler{service: service} }

func (h *ModelHandler) Current(w http.ResponseWriter, _ *http.Request) {
	api.WriteJSON(w, http.StatusOK, h.service.CurrentModelCatalog())
}
