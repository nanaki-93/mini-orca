// Package api provides HTTP handlers for the Mini-Orca REST API.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	apperrors "github.com/nanaki-93/mini-orca/internal/errors"
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
	mu       sync.RWMutex
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

	s.mu.Lock()
	defer s.mu.Unlock()

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

	s.sessions[sessionID] = session
	return session, nil
}

// GetSession retrieves a session by its ID.
func (s *SessionStore) GetSession(id string) (*state.Session, error) {
	if id == "" {
		return nil, fmt.Errorf("session: ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session: %q not found", id)
	}

	return copySession(session), nil
}

// UpdateSessionStatus updates the status and phase of a session.
func (s *SessionStore) UpdateSessionStatus(id string, status state.SessionStatus, phase state.Phase) error {
	if id == "" {
		return fmt.Errorf("session: ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("session: %q not found", id)
	}

	session.Status = status
	session.CurrentPhase = phase
	session.UpdatedAt = time.Now()

	return nil
}

// ListSessions returns all sessions.
func (s *SessionStore) ListSessions() []*state.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*state.Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		result = append(result, copySession(session))
	}

	// Sort by creation time, newest first
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// copySession creates a deep copy of a Session.
func copySession(s *state.Session) *state.Session {
	if s == nil {
		return nil
	}

	copy := *s
	if s.Plan != nil {
		planCopy := *s.Plan
		copy.Plan = &planCopy
	}
	if s.AtomicUnits != nil {
		unitsCopy := make([]state.AtomicUnit, len(s.AtomicUnits))
		copy.AtomicUnits = append(unitsCopy, s.AtomicUnits...)
	}
	if s.History != nil {
		historyCopy := make([]state.PhaseHistory, len(s.History))
		copy.History = append(historyCopy, s.History...)
	}
	if s.TestResults != nil {
		testResultsCopy := make([]state.TestResult, len(s.TestResults))
		copy.TestResults = append(testResultsCopy, s.TestResults...)
	}
	if s.ReviewReports != nil {
		reviewReportsCopy := make([]state.ReviewReportEntry, len(s.ReviewReports))
		copy.ReviewReports = append(reviewReportsCopy, s.ReviewReports...)
	}

	return &copy
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
		WriteAppError(w, apperrors.BadRequest("invalid request body", "The request body could not be parsed as JSON.", err))
		return
	}

	if req.Goal == "" {
		WriteAppError(w, apperrors.BadRequest("goal is required", "A session goal must be provided.", nil))
		return
	}

	if req.ProjectPath == "" {
		WriteAppError(w, apperrors.BadRequest("project_path is required", "A project path must be provided.", nil))
		return
	}

	session, err := h.sessionStore.CreateSession(req.Goal, req.ProjectPath, req.ProjectType)
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("session creation failed", "Failed to create new session: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusCreated, SessionResponse{
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
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("session not found", "The specified session could not be found.", err))
		return
	}

	WriteJSON(w, http.StatusOK, SessionResponse{
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
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("session not found", "The specified session could not be found.", err))
		return
	}

	if session.Status != state.SessionStatusPending {
		WriteAppError(w, apperrors.Conflict("invalid session status", "The session must be in pending state to start.", nil))
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusRunning, state.PhasePlanning); err != nil {
		WriteAppError(w, apperrors.Internal("session update failed", "Failed to update session status.", err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "started",
		"session_id": sessionID,
		"phase":      string(state.PhasePlanning),
	})
}

// PauseSession handles POST /api/sessions/:id/pause
// Transitions the session from running to paused.
func (h *SessionHandler) PauseSession(w http.ResponseWriter, r *http.Request) {
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("session not found", "The specified session could not be found.", err))
		return
	}

	if session.Status != state.SessionStatusRunning {
		WriteAppError(w, apperrors.Conflict("invalid session status", "The session must be running to pause it.", nil))
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusPaused, session.CurrentPhase); err != nil {
		WriteAppError(w, apperrors.Internal("session update failed", "Failed to pause session.", err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "paused",
		"session_id": sessionID,
	})
}

// ResumeSession handles POST /api/sessions/:id/resume
// Transitions the session from paused to running.
func (h *SessionHandler) ResumeSession(w http.ResponseWriter, r *http.Request) {
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("session not found", "The specified session could not be found.", err))
		return
	}

	if session.Status != state.SessionStatusPaused {
		WriteAppError(w, apperrors.Conflict("invalid session status", "The session must be paused to resume it.", nil))
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusRunning, session.CurrentPhase); err != nil {
		WriteAppError(w, apperrors.Internal("session update failed", "Failed to resume session.", err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "resumed",
		"session_id": sessionID,
	})
}

// StopSession handles POST /api/sessions/:id/stop
// Transitions the session to completed or cancelled state.
func (h *SessionHandler) StopSession(w http.ResponseWriter, r *http.Request) {
	sessionID := ExtractSessionID(r.URL.Path)
	if sessionID == "" {
		WriteAppError(w, apperrors.BadRequest("session ID is required", "A session ID must be provided in the URL.", nil))
		return
	}

	session, err := h.sessionStore.GetSession(sessionID)
	if err != nil {
		WriteAppError(w, apperrors.NotFound("session not found", "The specified session could not be found.", err))
		return
	}

	if session.Status == state.SessionStatusCompleted || session.Status == state.SessionStatusCancelled {
		WriteAppError(w, apperrors.Conflict("invalid session status", "The session is already in a terminal state.", nil))
		return
	}

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusCompleted, session.CurrentPhase); err != nil {
		WriteAppError(w, apperrors.Internal("session update failed", "Failed to stop session.", err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
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

	WriteJSON(w, http.StatusOK, SessionListResponse{
		Sessions: summaries,
		Total:    len(summaries),
	})
}
