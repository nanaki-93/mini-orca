package api

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/state"
)

// SessionCreateRequest represents the request body for creating a new session.
type SessionCreateRequest struct {
	// FeatureRequest is the feature request for the session.
	FeatureRequest string `json:"feature_request"`
	// ProjectPath is the path to the project directory.
	ProjectPath string `json:"project_path"`
	// ProjectType is the type of the project (e.g., "go", "python").
	ProjectType string `json:"project_type,omitempty"`
}

// SessionResponse represents the response body for session data.
type SessionResponse struct {
	// ID is the unique identifier for the session.
	ID string `json:"id"`
	// FeatureRequest is the feature request for the session.
	FeatureRequest string `json:"feature_request"`
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
	// FeatureRequest is the feature request for the session.
	FeatureRequest string `json:"feature_request"`
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
func (s *SessionStore) CreateSession(featureRequest, projectPath, projectType string) (*state.Session, error) {
	if featureRequest == "" {
		return nil, fmt.Errorf("session: feature_request is required")
	}

	if projectPath == "" {
		return nil, fmt.Errorf("session: project_path is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID := fmt.Sprintf("session-%d", time.Now().UnixNano())

	session := &state.Session{
		ID:            sessionID,
		Goal:          featureRequest,
		ProjectPath:   projectPath,
		ProjectType:   projectType,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentPhase:  state.PhaseCoding,
		Status:        state.SessionStatusPending,
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

	cp := *s
	if s.History != nil {
		historyCopy := make([]state.PhaseHistory, len(s.History))
		cp.History = append(historyCopy, s.History...)
	}
	if s.TestResults != nil {
		testResultsCopy := make([]state.TestResult, len(s.TestResults))
		cp.TestResults = append(testResultsCopy, s.TestResults...)
	}
	if s.ReviewReports != nil {
		reviewReportsCopy := make([]state.ReviewReportEntry, len(s.ReviewReports))
		cp.ReviewReports = append(reviewReportsCopy, s.ReviewReports...)
	}

	return &cp
}
