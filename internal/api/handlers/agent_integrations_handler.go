package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentIntegrationsHandler struct{ service *app.Service }

func NewAgentIntegrationsHandler(service *app.Service) *AgentIntegrationsHandler {
	return &AgentIntegrationsHandler{service: service}
}

func (h *AgentIntegrationsHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListIntegrations(r.Context(), query)
	respondIntegration(w, value, err)
}

func (h *AgentIntegrationsHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.IntegrationControl
	if !decodeStrictJSON(w, r, &request, "invalid integration action", "Provide current project identity, configuration fingerprint and explicit discovery or revoke action.") {
		return
	}
	value, err := h.service.ControlIntegration(r.Context(), r.PathValue("integrationID"), request)
	respondIntegration(w, value, err)
}

func respondIntegration(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, app.ErrIntegrationConflict):
		api.WriteAppError(w, api.Conflict("integration changed", "Discover and review the current inventory before granting a new investigation.", err))
	case errors.Is(err, app.ErrIntegrationUnavailable):
		api.WriteAppError(w, api.BadRequest("integration unavailable", "Check the configured installed server and credential references.", err))
	case err != nil:
		if !writeContextError(w, "integration request", err) {
			writeProjectError(w, "integration request failed", err)
		}
	default:
		api.WriteJSON(w, http.StatusOK, value)
	}
}
