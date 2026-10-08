package handlers

import (
	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"net/http"
	"time"
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
	value, err := h.service.SaveFeatureGoals(r.Context(), request)
	respondWorkflow(w, value, err)
}
func (h *FeatureHandler) Generate(w http.ResponseWriter, r *http.Request) {
	var request app.FeatureGenerateRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	// Generation has its own bounded context and can outlive the ordinary
	// server write timeout. Restore that timeout before sending the response.
	server, _ := r.Context().Value(http.ServerContextKey).(*http.Server)
	response := http.NewResponseController(w)
	if server != nil && server.WriteTimeout > 0 {
		if err := response.SetWriteDeadline(time.Time{}); err != nil {
			api.WriteAppError(w, api.Internal("prepare feature response deadline", "Feature generation could not start. Try again.", err))
			return
		}
	}
	value, err := h.service.GenerateFeatures(r.Context(), request)
	if server != nil && server.WriteTimeout > 0 {
		if deadlineErr := response.SetWriteDeadline(time.Now().Add(server.WriteTimeout)); deadlineErr != nil {
			api.WriteAppError(w, api.Internal("restore feature response deadline", "Refresh suggestions to read the feature result.", deadlineErr))
			return
		}
	}
	respondWorkflow(w, value, err)
}
func (h *FeatureHandler) Status(w http.ResponseWriter, r *http.Request) {
	var request app.FeatureStatusRequest
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.UpdateFeatureStatus(r.Context(), r.PathValue("featureID"), request)
	respondWorkflow(w, value, err)
}
