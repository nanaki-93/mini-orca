package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
)

type AgentRunHandler struct{ service *app.Service }

func NewAgentRunHandler(service *app.Service) *AgentRunHandler {
	return &AgentRunHandler{service: service}
}

func agentRunQuery(w http.ResponseWriter, r *http.Request, optional ...string) (app.AgentRunQuery, bool) {
	query, ok := analysisQuery(w, r, []string{"project_id", "project_revision"}, optional)
	if !ok {
		return app.AgentRunQuery{}, false
	}
	result := app.AgentRunQuery{ProjectID: query.Get("project_id"), ProjectRevision: query.Get("project_revision"), Cursor: query.Get("cursor")}
	if query.Has("limit") {
		limit, err := strconv.Atoi(query.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			api.WriteError(w, http.StatusBadRequest, "limit must be between 1 and 100")
			return result, false
		}
		result.Limit = limit
	}
	return result, true
}

func (h *AgentRunHandler) List(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r, "cursor", "limit")
	if !ok {
		return
	}
	value, err := h.service.ListAgentRuns(r.Context(), query)
	respondAgentRun(w, value, err)
}

func (h *AgentRunHandler) Detail(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r, "include_text")
	if !ok {
		return
	}
	text := r.URL.Query().Get("include_text")
	if text != "" && text != "true" && text != "false" {
		api.WriteError(w, http.StatusBadRequest, "include_text must be true or false")
		return
	}
	value, err := h.service.AgentRunDetail(r.Context(), query, r.PathValue("runID"), text == "true")
	respondAgentRun(w, value, err)
}

func (h *AgentRunHandler) Control(w http.ResponseWriter, r *http.Request) {
	var request app.AgentRunControl
	if !decodeStrictJSON(w, r, &request, "invalid agent run control", "Provide the current project identity and cancel action.") {
		return
	}
	value, err := h.service.ControlAgentRun(r.Context(), r.PathValue("runID"), request)
	respondAgentRun(w, value, err)
}

func (h *AgentRunHandler) Input(w http.ResponseWriter, r *http.Request) {
	var request app.ChangeInputResponse
	if !decodeWorkflowBody(w, r, &request) {
		return
	}
	value, err := h.service.RespondAgentRunInput(r.Context(), r.PathValue("runID"), request)
	respondAgentRun(w, value, err)
}

func respondAgentRun(w http.ResponseWriter, value any, err error) {
	if err != nil {
		if errors.Is(err, app.ErrAgentRunNotFound) {
			api.WriteAppError(w, api.NotFound("agent run unavailable", "Select a retained invocation in this checkout.", err))
			return
		}
		if errors.Is(err, app.ErrAgentHistoryUnavailable) {
			api.WriteAppError(w, api.Internal("agent history unavailable", "The private run journal could not be read.", err))
			return
		}
		if !writeContextError(w, "agent run request", err) {
			writeProjectError(w, "agent run request failed", err)
		}
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	api.WriteJSON(w, http.StatusOK, value)
}

// Ordinary GET returns one replay page. SSE uses the same durable cursor; the
// broker only wakes readers and never carries private model text.
func (h *AgentRunHandler) Events(w http.ResponseWriter, r *http.Request) {
	query, ok := agentRunQuery(w, r, "cursor")
	if !ok {
		return
	}
	if header := r.Header.Get("Last-Event-ID"); header != "" {
		if query.Cursor != "" && query.Cursor != header {
			api.WriteError(w, http.StatusBadRequest, "conflicting progress cursors")
			return
		}
		query.Cursor = header
	}
	updates, unsubscribe := h.service.SubscribeEvents()
	defer unsubscribe()
	batch, err := h.service.AgentRunEvents(r.Context(), query)
	if err != nil {
		respondAgentRun(w, nil, err)
		return
	}
	if !strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		respondAgentRun(w, batch, nil)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	controller := http.NewResponseController(w)
	defer controller.SetWriteDeadline(time.Time{})
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		if err := writeAgentBatch(w, controller, batch); err != nil {
			return
		}
		query.Cursor = batch.Cursor
		if !batch.HasMore {
			select {
			case <-r.Context().Done():
				return
			case <-updates:
			case <-ticker.C:
			}
			drainAgentUpdates(updates)
		}
		batch, err = h.service.AgentRunEvents(r.Context(), query)
		if err != nil {
			return
		} // Reconnect revalidates scope and returns a structured error.
	}
}

func drainAgentUpdates(updates <-chan string) {
	for range 16 {
		select {
		case <-updates:
		default:
			return
		}
	}
}

func writeAgentBatch(w http.ResponseWriter, controller *http.ResponseController, batch *app.AgentEventBatch) error {
	data, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	frame := fmt.Sprintf("event: agent_progress\nid: %s\ndata: %s\n\n", batch.Cursor, data)
	if len(frame) > 1<<20 {
		return fmt.Errorf("agent progress frame exceeds 1 MiB")
	}
	if err := controller.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return err
	}
	if _, err := w.Write([]byte(frame)); err != nil {
		return err
	}
	return controller.Flush()
}
