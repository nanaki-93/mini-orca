package handlers

import (
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentRemoteServiceHandler struct{ service *app.Service }

func NewAgentRemoteServiceHandler(service *app.Service) *AgentRemoteServiceHandler {
	return &AgentRemoteServiceHandler{service: service}
}

func (h *AgentRemoteServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListRemoteServicePrincipals(r.Context(), query)
	respondAPI(w, value, err)
}

func (h *AgentRemoteServiceHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.RemoteServiceControl
	if !decodeStrictJSON(w, r, &request, "invalid agent access action", "Review the configured principal, current project and source-sharing permission before granting access.") {
		return
	}
	value, err := h.service.ControlRemoteService(r.Context(), r.PathValue("principalID"), request)
	respondAPI(w, value, err)
}
