package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type AgentAPIsHandler struct{ service *app.Service }

func NewAgentAPIsHandler(service *app.Service) *AgentAPIsHandler {
	return &AgentAPIsHandler{service: service}
}

func (h *AgentAPIsHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListAPIs(r.Context(), query)
	respondAPI(w, value, err)
}

func (h *AgentAPIsHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.IntegrationControl
	if !decodeStrictJSON(w, r, &request, "invalid API inventory action", "Provide current project/configuration identity and an explicit discover or revoke action.") {
		return
	}
	value, err := h.service.ControlAPI(r.Context(), r.PathValue("integrationID"), request)
	respondAPI(w, value, err)
}

func (h *AgentAPIsHandler) Actions(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListAPIActions(r.Context(), query)
	respondAPI(w, value, err)
}

func (h *AgentAPIsHandler) ControlAction(w http.ResponseWriter, r *http.Request) {
	var request app.APIActionControl
	if !decodeStrictJSON(w, r, &request, "invalid external action", "Review the exact request, then provide its version/hash and explicit approval or reported reconciliation.") {
		return
	}
	value, err := h.service.ControlAPIAction(r.Context(), r.PathValue("actionID"), request)
	respondAPI(w, value, err)
}

func respondAPI(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, agentstate.ErrNotFound):
		api.WriteAppError(w, api.NotFound("external action unavailable", "Reload the current project's action receipts.", err))
	case errors.Is(err, agentstate.ErrActionConflict), errors.Is(err, agentstate.ErrActionUncertain):
		api.WriteAppError(w, api.Conflict("external action requires review", "Reload its current state. Reconcile an uncertain outcome with the external service before proposing another write.", err))
	case errors.Is(err, agentstate.ErrPersistence), errors.Is(err, agentstate.ErrCorrupt):
		api.WriteAppError(w, api.Internal("external action receipt unavailable", "Existing recovery data was preserved. Do not repeat an uncertain external write.", err))
	default:
		respondIntegration(w, value, err)
	}
}
