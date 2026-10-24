package handlers

import (
	"errors"
	"net/http"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentMissionHandler struct{ service *app.Service }

func NewAgentMissionHandler(service *app.Service) *AgentMissionHandler {
	return &AgentMissionHandler{service: service}
}

func (h *AgentMissionHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.ListMissions(r.Context(), query)
	respondMission(w, value, err)
}

func (h *AgentMissionHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.MissionDetail(r.Context(), query, r.PathValue("missionID"))
	respondMission(w, value, err)
}

func (h *AgentMissionHandler) Mutate(w http.ResponseWriter, r *http.Request) {
	var request app.MissionMutation
	if !decodeStrictJSON(w, r, &request, "invalid mission action", "Provide the current project identity, catalog revision, plan hash and explicit action.") {
		return
	}
	value, err := h.service.MutateMission(r.Context(), r.PathValue("missionID"), request)
	respondMission(w, value, err)
}

func (h *AgentMissionHandler) Plan(w http.ResponseWriter, r *http.Request) {
	var request app.MissionPlanningRequest
	if !decodeStrictJSON(w, r, &request, "invalid mission planning request", "Provide captured sources, mission and planning budgets, current project and catalog identities, and provider confirmation when required.") {
		return
	}
	value, err := h.service.PlanMission(r.Context(), request)
	respondMission(w, value, err)
}

func (h *AgentMissionHandler) Execution(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r)
	if !ok {
		return
	}
	value, err := h.service.MissionExecution(r.Context(), query, r.PathValue("missionID"))
	respondMission(w, value, err)
}

func (h *AgentMissionHandler) ControlExecution(w http.ResponseWriter, r *http.Request) {
	var request app.MissionExecutionRequest
	if !decodeStrictJSON(w, r, &request, "invalid mission execution request", "Confirm the concrete operation and exact execution revision; starting children also requires project execution and selected provider consent.") {
		return
	}
	value, err := h.service.ControlMissionExecution(r.Context(), r.PathValue("missionID"), request)
	respondMission(w, value, err)
}

func respondMission(w http.ResponseWriter, value any, err error) {
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case errors.Is(err, app.ErrMissionNotFound):
		api.WriteAppError(w, api.NotFound("mission unavailable", "Select a retained mission in this checkout.", err))
	case errors.Is(err, app.ErrMissionConflict):
		api.WriteAppError(w, api.Conflict("mission changed", "Reload the catalog and review the current plan before editing or approving.", err))
	case errors.Is(err, app.ErrMissionUnavailable):
		api.WriteAppError(w, api.Internal("mission unavailable", "The private mission catalog could not be read or written.", err))
	case errors.Is(err, app.ErrMissionPlanningIncomplete):
		api.WriteAppError(w, api.BadRequest("mission planning incomplete", "The provider did not return a complete valid mission. Existing plans were preserved; review the selected scope and budget before trying again.", err))
	case err != nil:
		if !writeContextError(w, "mission request", err) {
			writeProjectError(w, "mission request failed", err)
		}
	default:
		api.WriteJSON(w, http.StatusOK, value)
	}
}
