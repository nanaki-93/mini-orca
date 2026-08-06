package api

import (
	"encoding/json"
	"net/http"

	apperrors "github.com/nanaki-93/mini-orca/v2/internal/errors"
	"github.com/nanaki-93/mini-orca/v2/internal/orchestrator"
	"github.com/nanaki-93/mini-orca/v2/internal/state"
)

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

	if req.FeatureRequest == "" {
		WriteAppError(w, apperrors.BadRequest("feature_request is required", "A feature request must be provided.", nil))
		return
	}

	if req.ProjectPath == "" {
		WriteAppError(w, apperrors.BadRequest("project_path is required", "A project path must be provided.", nil))
		return
	}

	session, err := h.sessionStore.CreateSession(req.FeatureRequest, req.ProjectPath, req.ProjectType)
	if err != nil {
		WriteAppError(w, apperrors.BadRequest("session creation failed", "Failed to create new session: "+err.Error(), err))
		return
	}

	WriteJSON(w, http.StatusCreated, SessionResponse{
		ID:             session.ID,
		FeatureRequest: session.Goal,
		ProjectPath:    session.ProjectPath,
		ProjectType:    session.ProjectType,
		CurrentPhase:   string(state.PhaseCoding),
		Status:         string(session.Status),
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
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
		ID:             session.ID,
		FeatureRequest: session.Goal,
		ProjectPath:    session.ProjectPath,
		ProjectType:    session.ProjectType,
		CurrentPhase:   string(session.CurrentPhase),
		Status:         string(session.Status),
		CreatedAt:      session.CreatedAt,
		UpdatedAt:      session.UpdatedAt,
		Error:          session.Error,
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

	if err := h.sessionStore.UpdateSessionStatus(sessionID, state.SessionStatusRunning, state.PhaseCoding); err != nil {
		WriteAppError(w, apperrors.Internal("session update failed", "Failed to update session status.", err))
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{
		"status":     "started",
		"session_id": sessionID,
		"phase":      string(state.PhaseCoding),
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
			ID:             session.ID,
			FeatureRequest: session.Goal,
			CurrentPhase:   string(session.CurrentPhase),
			Status:         string(session.Status),
			CreatedAt:      session.CreatedAt,
		})
	}

	WriteJSON(w, http.StatusOK, SessionListResponse{
		Sessions: summaries,
		Total:    len(summaries),
	})
}
