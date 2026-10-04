package handlers

import (
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
)

type FeatureHandler struct {
	service *app.Service
	manager *project.Manager
}

func NewFeatureHandler(service *app.Service, manager *project.Manager) *FeatureHandler {
	return &FeatureHandler{service, manager}
}
func (h *FeatureHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !guardedWorkflowRead(w, r, h.manager) {
		return
	}
	value, err := h.service.Features(r.Context())
	respondWorkflow(w, value, err)
}
func (h *FeatureHandler) Goals(w http.ResponseWriter, r *http.Request) {
	var request app.FeatureRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.SaveFeatureGoals(request)
	respondWorkflow(w, value, err)
}
func (h *FeatureHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var request app.FeatureRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.GenerateFeatures(r.Context(), request)
	respondWorkflow(w, value, err)
}
func (h *FeatureHandler) Status(w http.ResponseWriter, r *http.Request) {
	var request app.FeatureStatusRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.UpdateFeatureStatus(r.PathValue("featureID"), request)
	respondWorkflow(w, value, err)
}
