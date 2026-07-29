// Package handlers provides HTTP handlers for the Mini-Orca REST API.
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/internal/orchestrator"
	"github.com/nanaki-93/mini-orca/internal/state"
)

// ─── Request/Response Types ──────────────────────────────────────────────────

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

// SessionDeleteResponse represents the response body for session deletion.
type SessionDeleteResponse struct {
	// Message is the operation result message.
	Message string `json:"message"`
	// SessionID is the ID of the deleted session.
	SessionID string `json:"session_id"`
}

// SessionHistoryResponse represents the response body for phase history.
type SessionHistoryResponse struct {
	// SessionID is the unique identifier for the session.
	SessionID string `json:"session_id"`
	// History is the list of phase execution history entries.
	History []PhaseHistoryEntry `json:"history"`
	// Total is the total number of history entries.
	Total int `json:"total"`
}

// PhaseHistoryEntry represents a single phase execution history entry.
type PhaseHistoryEntry struct {
	// ID is the unique identifier for the history entry.
	ID string `json:"id"`
	// SessionID is the ID of the session this phase belongs to.
	SessionID string `json:"session_id"`
	// Phase is the phase that was executed.
	Phase string `json:"phase"`
	// StartedAt is when the phase execution started.
	StartedAt time.Time `json:"started_at"`
	// CompletedAt is when the phase execution completed.
	CompletedAt time.Time `json:"completed_at"`
	// Status is the result status of the phase execution.
	Status string `json:"status"`
	// Output contains the output of the phase execution.
	Output string `json:"output,omitempty"`
	// Error contains any error that occurred during the phase execution.
	Error string `json:"error,omitempty"`
	// RetryCount is the number of times this phase was retried.
	RetryCount int `json:"retry_count"`
}

// ─── Store ────────────────────────────────────────────────────────────────────

// SessionStore provides thread-safe in-memory storage for sessions.
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

	s.mu.Lock()
	defer s.mu.Unlock()
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

// DeleteSession removes a session from the store.
func (s *SessionStore) DeleteSession(id string) error {
	if id == "" {
		return fmt.Errorf("session: ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return fmt.Errorf("session: %q not found", id)
	}

	delete(s.sessions, id)
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

// GetSessionHistory retrieves all phase history entries for a specific session.
func (s *SessionStore) GetSessionHistory(sessionID string) ([]state.PhaseHistory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("session: ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session: %q not found", sessionID)
	}

	if len(session.History) == 0 {
		return nil, nil
	}

	// Return a copy to prevent external mutation
	result := make([]state.PhaseHistory, len(session.History))
	copy(result, session.History)

	return result, nil
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

// ─── Handler ──────────────────────────────────────────────────────────────────

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

// GetSession handles GET /api/sessions/:id
// Returns the current status of the specified session.
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
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

// DeleteSession handles DELETE /api/sessions/:id
// Removes the specified session from the store.
func (h *SessionHandler) DeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	if err := h.sessionStore.DeleteSession(sessionID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SessionDeleteResponse{
		Message:   "session deleted successfully",
		SessionID: sessionID,
	})
}

// GetSessionHistory handles GET /api/sessions/:id/history
// Returns the phase execution history for the specified session.
func (h *SessionHandler) GetSessionHistory(w http.ResponseWriter, r *http.Request) {
	sessionID := extractSessionID(r.URL.Path)
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session ID is required")
		return
	}

	history, err := h.sessionStore.GetSessionHistory(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	entries := make([]PhaseHistoryEntry, 0, len(history))
	for _, h := range history {
		entries = append(entries, PhaseHistoryEntry{
			ID:          h.ID,
			SessionID:   h.SessionID,
			Phase:       h.Phase,
			StartedAt:   h.StartedAt,
			CompletedAt: h.CompletedAt,
			Status:      string(h.Status),
			Output:      h.Output,
			Error:       h.Error,
			RetryCount:  h.RetryCount,
		})
	}

	writeJSON(w, http.StatusOK, SessionHistoryResponse{
		SessionID: sessionID,
		History:   entries,
		Total:     len(entries),
	})
}
