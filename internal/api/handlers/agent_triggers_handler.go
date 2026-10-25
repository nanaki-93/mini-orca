package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

type AgentTriggersHandler struct{ service *app.Service }

func NewAgentTriggersHandler(service *app.Service) *AgentTriggersHandler {
	return &AgentTriggersHandler{service: service}
}

func (h *AgentTriggersHandler) Get(w http.ResponseWriter, r *http.Request) {
	if !mainTriggerScope(w, r) {
		return
	}
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.AgentTriggers(r.Context(), query)
	respondTriggers(w, value, err)
}

func (h *AgentTriggersHandler) Control(w http.ResponseWriter, r *http.Request) {
	if !mainTriggerScope(w, r) {
		return
	}
	var request app.TriggerControl
	if !decodeStrictJSON(w, r, &request, "invalid ambient analysis control", "Provide the exact checkout, configuration and control revision. Watch setup and provider permission require separate confirmations.") {
		return
	}
	value, err := h.service.ControlAgentTriggers(r.Context(), request)
	respondTriggers(w, value, err)
}

func mainTriggerScope(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Has("worktree_path") {
		api.WriteAppError(w, api.BadRequest("ambient analysis requires the main checkout", "Return to the main project scope before inspecting or controlling its watch.", nil))
		return false
	}
	return true
}

func respondTriggers(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	if errors.Is(err, agentstate.ErrTriggerConflict) {
		api.WriteAppError(w, api.Conflict("ambient analysis state changed", "Refresh local state before another control. A lost receipt may have a retained outcome.", err))
		return
	}
	respondAgentSettings(w, value, err)
}
