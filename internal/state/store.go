// Package state provides persistence and retrieval of orchestrator state.
package state

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/logging"
	"github.com/nanaki-93/mini-orca/v2/internal/tools"
)

// Store manages the state of the orchestrator pipeline.
type Store struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	histories     map[string][]PhaseHistory
	testResults   map[string][]TestResult
	reviewReports map[string][]ReviewReportEntry
}

// NewStore creates a new Store instance with the given session.
func NewStore(session *Session) *Store {
	store := &Store{
		sessions:      make(map[string]*Session),
		histories:     make(map[string][]PhaseHistory),
		testResults:   make(map[string][]TestResult),
		reviewReports: make(map[string][]ReviewReportEntry),
	}

	// Save the initial session if provided
	if session != nil {
		store.SaveSession(session)
	}

	return store
}

// SaveSession saves a session to the store.
func (s *Store) SaveSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("store: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = session
	s.sessions[session.ID].UpdatedAt = time.Now()

	return nil
}

// GetSession retrieves a session by its ID.
func (s *Store) GetSession(id string) (*Session, error) {
	if id == "" {
		return nil, fmt.Errorf("store: session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, fmt.Errorf("store: session %q not found", id)
	}

	return s.copySessionData(session), nil
}

// ListSessions returns a list of all sessions in the store.
func (s *Store) ListSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, s.copySessionData(session))
	}

	return sessions
}

// GetCurrentSession returns the first session in the store, or nil if none exist.
func (s *Store) GetCurrentSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		return s.copySessionData(session)
	}

	return nil
}

// SavePhaseHistory saves a phase history entry to the store.
func (s *Store) SavePhaseHistory(history *PhaseHistory) error {
	if history == nil {
		return fmt.Errorf("store: phase history is required")
	}

	if history.ID == "" {
		return fmt.Errorf("store: phase history ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.histories[history.SessionID] = append(s.histories[history.SessionID], *history)

	// Also update the session's history
	if session, ok := s.sessions[history.SessionID]; ok {
		session.History = append(session.History, *history)
		session.UpdatedAt = time.Now()
	}

	return nil
}

// GetSessionHistory retrieves all phase history entries for a specific session.
func (s *Store) GetSessionHistory(sessionID string) ([]PhaseHistory, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("store: session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	history, ok := s.histories[sessionID]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external mutation
	result := make([]PhaseHistory, len(history))
	copy(result, history)

	return result, nil
}

// UpdatePhaseStatus updates the session's current phase and status.
func (s *Store) UpdatePhaseStatus(phase Phase, status SessionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Update the first session found, or the specified session if we have a way to identify it
	// For now, update all sessions (typically there's only one active session)
	for _, session := range s.sessions {
		session.CurrentPhase = phase
		session.Status = status
		session.UpdatedAt = time.Now()
	}

	return nil
}

// SetError sets the session error.
func (s *Store) SetError(sessionID string, err error) error {
	if sessionID == "" {
		return fmt.Errorf("store: session ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return fmt.Errorf("store: session %q not found", sessionID)
	}

	if err != nil {
		session.Error = err.Error()
		session.Status = SessionStatusFailed
	}
	session.UpdatedAt = time.Now()

	return nil
}

// SaveTestResult saves a test result to the store.
func (s *Store) SaveTestResult(result *TestResult) error {
	if result == nil {
		return fmt.Errorf("store: test result is required")
	}

	if result.ID == "" {
		return fmt.Errorf("store: test result ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.testResults[result.SessionID] = append(s.testResults[result.SessionID], *result)

	// Also update the session's test results
	if session, ok := s.sessions[result.SessionID]; ok {
		session.TestResults = append(session.TestResults, *result)
		session.UpdatedAt = time.Now()
	}

	return nil
}

// GetTestResults retrieves all test results for a specific session.
func (s *Store) GetTestResults(sessionID string) ([]TestResult, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("store: session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results, ok := s.testResults[sessionID]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external mutation
	resultCopy := make([]TestResult, len(results))
	copy(resultCopy, results)

	return resultCopy, nil
}

// SaveReviewReport saves a review report to the store.
func (s *Store) SaveReviewReport(report *ReviewReportEntry) error {
	if report == nil {
		return fmt.Errorf("store: review report is required")
	}

	if report.ID == "" {
		return fmt.Errorf("store: review report ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.reviewReports[report.SessionID] = append(s.reviewReports[report.SessionID], *report)

	// Also update the session's review reports
	if session, ok := s.sessions[report.SessionID]; ok {
		session.ReviewReports = append(session.ReviewReports, *report)
		session.UpdatedAt = time.Now()
	}

	return nil
}

// GetReviewReports retrieves all review reports for a specific session.
func (s *Store) GetReviewReports(sessionID string) ([]ReviewReportEntry, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("store: session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	reports, ok := s.reviewReports[sessionID]
	if !ok {
		return nil, nil
	}

	// Return a copy to prevent external mutation
	resultCopy := make([]ReviewReportEntry, len(reports))
	copy(resultCopy, reports)

	return resultCopy, nil
}

// copySessionData creates a deep copy of session data for safe read access.
func (s *Store) copySessionData(session *Session) *Session {
	if session == nil {
		return nil
	}

	sessCopy := *session
	if session.History != nil {
		historyCopy := make([]PhaseHistory, len(session.History))
		copy(historyCopy, session.History)
		sessCopy.History = historyCopy
	}
	if session.TestResults != nil {
		testResultsCopy := make([]TestResult, len(session.TestResults))
		copy(testResultsCopy, session.TestResults)
		sessCopy.TestResults = testResultsCopy
	}
	if session.ReviewReports != nil {
		reviewReportsCopy := make([]ReviewReportEntry, len(session.ReviewReports))
		copy(reviewReportsCopy, session.ReviewReports)
		sessCopy.ReviewReports = reviewReportsCopy
	}

	return &sessCopy
}

// InitStateStore creates a state store with a new session.
func InitStateStore(projectInfo *tools.ProjectInfo) *Store {
	currentDir, _ := os.Getwd()
	if currentDir == "" {
		currentDir = "."
	}

	projectType := "unknown"
	if projectInfo != nil {
		projectType = string(projectInfo.Type)
	}

	session := &Session{
		ID:            generateSessionID(),
		Goal:          "",
		ProjectPath:   currentDir,
		ProjectType:   projectType,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CurrentPhase:  PhaseCoding,
		Status:        SessionStatusPending,
		History:       nil,
		TestResults:   nil,
		ReviewReports: nil,
		Error:         "",
	}

	store := NewStore(session)
	logging.Info("State store initialized", "session_id", session.ID)
	return store
}

// generateSessionID generates a simple session ID.
func generateSessionID() string {
	return fmt.Sprintf("session-%d", time.Now().UnixNano())
}
