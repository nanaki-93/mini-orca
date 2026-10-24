package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentMemoryHandler struct{ service *app.Service }

func NewAgentMemoryHandler(service *app.Service) *AgentMemoryHandler {
	return &AgentMemoryHandler{service: service}
}

func (h *AgentMemoryHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListMemories(r.Context(), query)
	respondMemory(w, value, err)
}

func (h *AgentMemoryHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.MemoryDetail(r.Context(), query, r.PathValue("memoryID"))
	respondMemory(w, value, err)
}

func (h *AgentMemoryHandler) Mutate(w http.ResponseWriter, r *http.Request) {
	var request app.MemoryMutation
	if !decodeStrictJSON(w, r, &request, "invalid memory action", "Provide current project identity, action and memory version.") {
		return
	}
	value, err := h.service.MutateMemory(r.Context(), r.PathValue("memoryID"), request)
	respondMemory(w, value, err)
}

func respondMemory(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, app.ErrMemoryNotFound):
		api.WriteAppError(w, api.NotFound("memory unavailable", "Select a retained record in this checkout.", err))
	case errors.Is(err, app.ErrMemoryConflict):
		api.WriteAppError(w, api.Conflict("memory changed", "Reload the current version before editing, accepting or forgetting.", err))
	case errors.Is(err, app.ErrMemoryUnavailable):
		api.WriteAppError(w, api.Internal("memory unavailable", "The private memory catalog could not be read or written.", err))
	case err != nil:
		if !writeContextError(w, "memory request", err) {
			writeProjectError(w, "memory request failed", err)
		}
	default:
		api.WriteJSON(w, http.StatusOK, value)
	}
}
