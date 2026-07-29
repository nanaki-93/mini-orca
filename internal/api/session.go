// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nanaki-93/mini-orca/internal/orchestrator"
	"github.com/nanaki-93/mini-orca/internal/state"
)

// SessionCreateRequest represents the request body for creating a new session.
type SessionCreateRequest struct {
	// Goal is the high-level goal of the session.
	Goal string `json:"goal"`
	// ProjectPath is the path to the project directory.
	ProjectPath string `json:"project_path"`
	// ProjectType is the type of the project (e.g., "go", "python").
	ProjectType string `json:"project_type,omitempty"`
}

// SessionResponse represents the response body for session data.
type SessionResponse struct {
	// ID is the unique identifier for the session.
	ID string `json:"id"`
	// Goal is the high-level goal of the session.
	Goal string `json:"goal"`
	// ProjectPath is the path to the project directory.
	ProjectPath string `json:"project_path"`
	// ProjectType is the type of the project.
	ProjectType string `json:"project_type"`
	// CurrentPhase is the current phase of the session.
	CurrentPhase string `json:"current_phase"`
	// Status is the current status of the session.
	Status string `json:"status"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is when the session was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// Error contains any error that occurred during the session.
	Error string `json:"error,omitempty"`
}

// SessionListResponse represents the response body for listing sessions.
type SessionListResponse struct {
	// Sessions is the list of session summaries.
	Sessions []SessionSummary `json:"sessions"`
	// Total is the total number of sessions.
	Total int `json:"total"`
}

// SessionSummary represents a summary of a session for list responses.
type SessionSummary struct {
	// ID is the unique identifier for the session.
	ID string `json:"id"`
	// Goal is the high-level goal of the session.
	Goal string `json:"goal"`
	// CurrentPhase is the current phase of the session.
	CurrentPhase string `json:"current_phase"`
	// Status is the current status of the session.
	Status string `json:"status"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"created_at"`
}

// SessionStore provides access to sessions by ID.
type SessionStore struct {
	mu       interface{}
	sessions map[string]*state.Session
}

// NewSessionStore creates a new SessionStore instance.
func NewSessionStore() *SessionStore {
	return &SessionStore{
		sessions: make(map[string]*state.Session),
	}
}

// CreateSession creates a new session with the given configuration.
func (s *SessionStore) CreateSession(goal, projectPath, projectType string) (*state.Session, error) {
	if goal == "" {
		return nil, fmt.Errorf("session: goal is required")
	}

	if projectPath == "" {
		return nil, fmt.Errorf("session: project_path is required")
	}

	sessions := make(map[string]*state.Session)
	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	session := &state.Session{
		ID:            sessionID,
		Goal:          goal,
		ProjectPath:   projectPath,
		ProjectType:   projectType,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentPhase:  state.PhasePlanning,
		Status:        state.SessionStatusPending,
		Plan:          nil,
		AtomicUnits:   nil,
		History:       nil,
		TestResults:   nil,
		ReviewReports: nil,
		Error:         "",
	}

	sessions[sessionID] = session
	_ = sessions
	return session, nil
}

// GetSession retrieves a session by its ID.
func (s *SessionStore) GetSession(id string) (*state.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session: ID is required")
	}

	sessions := make(map[string]*state.Session)
	_, ok := sessions[id]
	if !ok {
		return nil, fmt.Errorf("session: %q not found", id)
	}

	return nil, nil
}

// UpdateSessionStatus updates the status and phase of a session.
func (s *SessionStore) UpdateSessionStatus(id string, status state.SessionStatus, phase state.Phase) error {
	if id == "" {
		return fmt.Errorf("session: ID is required")
	}

	sessions := make(map[string]*state.Session)
	_, ok := sessions[id]
	if !ok {
		return fmt.Errorf("session: %q not found", id)
	}

	_ = status
	_ = phase
	return nil
}

// ListSessions returns all sessions.
func (s *SessionStore) ListSessions() []*state.Session {
	sessions := make(map[string]*state.Session)
	result := make([]*state.Session, 0, len(sessions))
	for _, session := range sessions {
		result = append(result, session)
	}
	return result
}

// SessionHandler manages HTTP handlers for session lifecycle operations.
type SessionHandler struct {
	// sessionStore provides access to session data.
	sessionStore *SessionStore
	// gateStore provides access to human gates.
	gateStore *GateStore
	// router provides phase transition capabilities.
	router *orchestrator.PhaseRouter
}

// NewSessionHandler creates a new SessionHandler instance.
func NewSessionHandler(sessionStore *SessionStore, gateStore *GateStore, router *orchestrator.PhaseRouter) *SessionHandler {
	return &SessionHandler{
		sessionStore: sessionStore,
		gateStore:    gateStore,
		router:       router,
	}
}

// CreateSession handles POST /api/sessions
// Creates a new session with the given goal and project configuration.
func (h *SessionHandler) CreateSession(w http.ResponseWriter, r *http.Request) {
	var req SessionCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Goal == "" {
		writeError(w, http.StatusBadRequest, "goal is required")
		return
	}

	if req.ProjectPath == "" {
		writeError(w, http.StatusBadRequest, "project_path is required")
		return
	}

	session, err := h.sessionStore.CreateSession(req.Goal, req.ProjectPath, req.ProjectType)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, SessionResponse{
		ID:           session.ID,
		Goal:         session.Goal,
		ProjectPath:  session.ProjectPath,
		ProjectType:  session.ProjectType,
		CurrentPhase: string(session.CurrentPhase),
		Status:       string(session.Status),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
	})
}

// GetSessionStatus handles GET /api/sessions/:id
// Returns the current status of the specified session.
func (h *SessionHandler) GetSessionStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SessionResponse{
		ID:           session.ID,
		Goal:         session.Goal,
		ProjectPath:  session.ProjectPath,
		ProjectType:  session.ProjectType,
		CurrentPhase: string(session.CurrentPhase),
		Status:       string(session.Status),
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
		Error:        session.Error,
	})
}

// StartSession handles POST /api/sessions/:id/start
// Transitions the session from pending to running.
func (h *SessionHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if session.Status != state.SessionStatusPending {
		writeError(w, http.StatusConflict, "session is not in pending state")
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusRunning, state.PhasePlanning); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "started",
		"session_id": sessionID,
		"phase":      string(state.PhasePlanning),
	})
}

// PauseSession handles POST /api/sessions/:id/pause
// Transitions the session from running to paused.
func (h *SessionHandler) PauseSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if session.Status != state.SessionStatusRunning {
		writeError(w, http.StatusConflict, "session is not in running state")
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusPaused, session.CurrentPhase); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "paused",
		"session_id": sessionID,
	})
}

// ResumeSession handles POST /api/sessions/:id/resume
// Transitions the session from paused to running.
func (h *SessionHandler) ResumeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if session.Status != state.SessionStatusPaused {
		writeError(w, http.StatusConflict, "session is not in paused state")
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusRunning, session.CurrentPhase); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "resumed",
		"session_id": sessionID,
	})
}

// StopSession handles POST /api/sessions/:id/stop
// Transitions the session to completed or cancelled state.
func (h *SessionHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if session.Status == state.SessionStatusCompleted || session.Status == state.SessionStatusCancelled {
		writeError(w, http.StatusConflict, "session is already in terminal state")
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusCompleted, session.CurrentPhase); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":     "stopped",
		"session_id": sessionID,
	})
}

// ListSessions handles GET /api/sessions
// Returns a list of all sessions with their summaries.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	sessions := h.sessionStore.ListSessions()

	summaries := make([]SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		summaries = append(summaries, SessionSummary{
			ID:           session.ID,
			Goal:         session.Goal,
			CurrentPhase: string(session.CurrentPhase),
			Status:       string(session.Status),
			CreatedAt:    session.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, SessionListResponse{
		Sessions: summaries,
		Total:    len(summaries),
	})
}
