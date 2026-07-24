package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"mini-orca/internal/orchestrator"
	"mini-orca/internal/state"
)

// ProjectHandler handles project-related API endpoints.
type ProjectHandler struct {
	orchestrator *orchestrator.Orchestrator
	stateStore   *state.Store
}

// NewProjectHandler creates a new project handler.
func NewProjectHandler(orch *orchestrator.Orchestrator, store *state.Store) *ProjectHandler {
	return &ProjectHandler{
		orchestrator: orch,
		stateStore:   store,
	}
}

// GetSession handles GET /api/session
func (h *ProjectHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	session := h.orchestrator.GetSession()
	
	response := map[string]interface{}{
		"session_id": session.ID,
		"name":       session.Name,
		"phase":      session.Phase,
		"created_at": session.CreatedAt,
		"updated_at": session.UpdatedAt,
		"plan": map[string]interface{}{
			"id":               session.Plan.ID,
			"status":           session.Plan.Status,
			"atomic_units":     len(session.Plan.AtomicUnits),
			"completed_units":  len(session.Plan.GetCompletedUnits()),
			"pending_units":    len(session.Plan.GetPendingUnits()),
		},
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListSessions handles GET /api/sessions
func (h *ProjectHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.stateStore.ListSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	response := make([]map[string]interface{}, 0)
	for _, s := range sessions {
		response = append(response, map[string]interface{}{
			"id":       s.ID,
			"name":     s.Name,
			"phase":    s.Phase,
			"created":  s.CreatedAt,
			"updated":  s.UpdatedAt,
		})
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CreateSession handles POST /api/sessions
func (h *ProjectHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		ProjectDir  string `json:"project_dir"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	session := state.NewSession("", req.Name, req.Description, req.ProjectDir)
	
	if err := h.stateStore.CreateSession(session); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session_id": session.ID,
		"name":       session.Name,
		"phase":      session.Phase,
	})
}

// UpdatePhase handles POST /api/session/phase
func (h *ProjectHandler) UpdatePhase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phase state.Phase `json:"phase"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	session := h.orchestrator.GetSession()
	if err := session.TransitionTo(req.Phase); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	
	if err := h.stateStore.UpdateSession(session); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"phase":  session.Phase,
		"updated": time.Now(),
	})
}
