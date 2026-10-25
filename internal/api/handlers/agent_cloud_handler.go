package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type AgentCloudHandler struct{ service *app.Service }

func NewAgentCloudHandler(service *app.Service) *AgentCloudHandler {
	return &AgentCloudHandler{service}
}

func mainCloudScope(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Has("worktree_path") {
		api.WriteAppError(w, api.BadRequest("cloud copies require the main checkout", "Select the main project scope before inspecting cloud copies.", nil))
		return false
	}
	return true
}
func (h *AgentCloudHandler) List(w http.ResponseWriter, r *http.Request) {
	if !mainCloudScope(w, r) {
		return
	}
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.CloudCopies(r.Context(), query)
	respondCloud(w, value, err)
}
func (h *AgentCloudHandler) Preview(w http.ResponseWriter, r *http.Request) {
	if !mainCloudScope(w, r) {
		return
	}
	var request app.CloudSelection
	if !decodeStrictJSON(w, r, &request, "invalid cloud selection", "Provide the main checkout, configured destination and selected completed data.") {
		return
	}
	value, err := h.service.PreviewCloudCopy(r.Context(), request)
	respondCloud(w, value, err)
}
func (h *AgentCloudHandler) Control(w http.ResponseWriter, r *http.Request) {
	if !mainCloudScope(w, r) {
		return
	}
	var request app.CloudControl
	if !decodeStrictJSON(w, r, &request, "invalid cloud control", "Confirm the exact preview or retained transfer version. A lost receipt may have a remote outcome.") {
		return
	}
	value, err := h.service.ControlCloudCopy(r.Context(), request)
	respondCloud(w, value, err)
}
func respondCloud(w http.ResponseWriter, value any, err error) {
	if errors.Is(err, agentstate.ErrCloudConflict) {
		api.WriteAppError(w, api.Conflict("cloud copy changed", "Refresh the local receipt. Reconcile or delete uncertain remote copies explicitly; do not replay an upload.", err))
		return
	}
	respondAgentSettings(w, value, err)
}
