package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type AgentDelegationHandler struct{ service *app.Service }

func NewAgentDelegationHandler(service *app.Service) *AgentDelegationHandler {
	return &AgentDelegationHandler{service: service}
}

func (h *AgentDelegationHandler) Remotes(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListRemoteAgents(r.Context(), query)
	respondDelegation(w, value, err)
}

func (h *AgentDelegationHandler) ControlRemote(w http.ResponseWriter, r *http.Request) {
	var request app.IntegrationControl
	if !decodeStrictJSON(w, r, &request, "invalid remote connection action", "Provide current project/configuration identity and explicit discover or revoke consent.") {
		return
	}
	value, err := h.service.ControlRemoteAgent(r.Context(), r.PathValue("integrationID"), request)
	respondDelegation(w, value, err)
}

func (h *AgentDelegationHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListDelegations(r.Context(), query)
	respondDelegation(w, value, err)
}

func (h *AgentDelegationHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.DelegationDetail(r.Context(), query, r.PathValue("delegationID"))
	respondDelegation(w, value, err)
}

func (h *AgentDelegationHandler) Start(w http.ResponseWriter, r *http.Request) {
	var request app.DelegationRequest
	if !decodeStrictJSON(w, r, &request, "invalid delegation", "Select a current card grant, advertised skill, explicit question/source hashes and delegation consent.") {
		return
	}
	value, err := h.service.StartDelegation(r.Context(), request)
	respondDelegation(w, value, err)
}

func (h *AgentDelegationHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.DelegationControl
	if !decodeStrictJSON(w, r, &request, "invalid delegation action", "Review the receipt version and confirm a refresh, reconnect, input, cancel or reconciliation.") {
		return
	}
	value, err := h.service.ControlDelegation(r.Context(), r.PathValue("delegationID"), request)
	respondDelegation(w, value, err)
}

func respondDelegation(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, agentstate.ErrNotFound):
		api.WriteAppError(w, api.NotFound("delegation unavailable", "Reload this project's delegation receipts.", err))
	case errors.Is(err, agentstate.ErrPersistence), errors.Is(err, agentstate.ErrCorrupt):
		api.WriteAppError(w, api.Internal("delegation receipt unavailable", "Existing recovery data was preserved. Do not resend an uncertain task.", err))
	case errors.Is(err, agentstate.ErrDelegationConflict):
		api.WriteAppError(w, api.Conflict("delegation requires review", "Reload its receipt. Refresh or reconnect to a retained task; never resend an uncertain message.", err))
	case errors.Is(err, agentruntime.ErrRemoteAuth):
		api.WriteAppError(w, api.BadRequest("remote authentication required", "Review the configured credential reference. No additional credentials were acquired.", err))
	case errors.Is(err, agentruntime.ErrRemoteProtocol), errors.Is(err, agentruntime.ErrRemoteTransport), errors.Is(err, agentruntime.ErrPolicyDenied):
		api.WriteAppError(w, api.BadRequest("remote connection unavailable", "Review the configured A2A 1.0 card, endpoint and advertised text skills.", err))
	default:
		respondAPI(w, value, err)
	}
}
