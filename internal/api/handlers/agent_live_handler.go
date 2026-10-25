package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type AgentLiveHandler struct{ service *app.Service }

func NewAgentLiveHandler(service *app.Service) *AgentLiveHandler {
	return &AgentLiveHandler{service: service}
}

func (h *AgentLiveHandler) Overview(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.LiveSessions(r.Context(), query)
	respondLive(w, value, err)
}
func (h *AgentLiveHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.LiveSessionDetail(r.Context(), query, r.PathValue("liveID"))
	respondLive(w, value, err)
}
func (h *AgentLiveHandler) Open(w http.ResponseWriter, r *http.Request) {
	var request app.LiveOpenRequest
	if !decodeStrictJSON(w, r, &request, "invalid live session request", "Provide the exact checkout, settings, live model, destination and explicit connection/provider confirmations.") {
		return
	}
	value, err := h.service.OpenLiveSession(r.Context(), request)
	respondLive(w, value, err)
}
func (h *AgentLiveHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.LiveControl
	if !decodeStrictJSON(w, r, &request, "invalid live session control", "Use the current control revision and exact session binding; confirm each input or reconnection.") {
		return
	}
	value, err := h.service.ControlLiveSession(r.Context(), r.PathValue("liveID"), request)
	respondLive(w, value, err)
}
func respondLive(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, agentstate.ErrNotFound):
		api.WriteAppError(w, api.NotFound("live session unavailable", "Select a retained session owned by this project.", err))
	case errors.Is(err, agentstate.ErrLiveConflict):
		api.WriteAppError(w, api.Conflict("live session changed", "Refresh local session history before another control. Restarted sessions require a new explicit connection.", err))
	case errors.Is(err, app.ErrLiveUnavailable):
		api.WriteAppError(w, api.NewAppError(api.ErrorInternal, "live connection unavailable", "Inspect retained local history. Input or usage may be partial; no reconnect or resend happens automatically.", http.StatusBadGateway, err))
	default:
		respondArtifact(w, value, err)
	}
}
