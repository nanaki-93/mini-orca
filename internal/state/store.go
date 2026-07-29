// Package state provides persistence and retrieval of orchestrator state.
package state

import (
	"fmt"
	"sync"
	"time"
)

// Store manages the state of the orchestrator pipeline.
type Store struct {
	mu            sync.RWMutex
	sessions      map[string]*Session
	plans         map[string]*Plan
	histories     map[string][]PhaseHistory
	testResults   map[string][]TestResult
	reviewReports map[string][]ReviewReportEntry
}

// NewStore creates a new Store instance with the given session.
func NewStore(session *Session) *Store {
	return &Store{
		sessions:      make(map[string]*Session),
		plans:         make(map[string]*Plan),
		histories:     make(map[string][]PhaseHistory),
		testResults:   make(map[string][]TestResult),
		reviewReports: make(map[string][]ReviewReportEntry),
	}
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

// SavePlan saves a plan to the store.
func (s *Store) SavePlan(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("store: plan is required")
	}

	if plan.ID == "" {
		return fmt.Errorf("store: plan ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.plans[plan.ID] = plan

	// Also update the session's plan reference
	if session, ok := s.sessions[plan.SessionID]; ok {
		session.Plan = plan
		session.UpdatedAt = time.Now()
	}

	return nil
}

// GetPlan retrieves a plan by its ID.
func (s *Store) GetPlan(planID string) (*Plan, error) {
	if planID == "" {
		return nil, fmt.Errorf("store: plan ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	plan, ok := s.plans[planID]
	if !ok {
		return nil, fmt.Errorf("store: plan %q not found", planID)
	}

	// Return a copy to prevent external mutation
	return s.copyPlanData(plan), nil
}

// UpdateUnitStatus updates the status of a unit within a plan.
func (s *Store) UpdateUnitStatus(unitID string, status UnitStatus) error {
	if unitID == "" {
		return fmt.Errorf("store: unit ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, plan := range s.plans {
		for i := range plan.Units {
			if plan.Units[i].ID == unitID {
				plan.Units[i].Status = status
				return nil
			}
		}
	}

	return fmt.Errorf("store: unit %q not found in any plan", unitID)
}

// GetPendingUnits retrieves all pending units from a specific plan.
func (s *Store) GetPendingUnits(planID string) ([]PlanUnit, error) {
	if planID == "" {
		return nil, fmt.Errorf("store: plan ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	plan, ok := s.plans[planID]
	if !ok {
		return nil, fmt.Errorf("store: plan %q not found", planID)
	}

	var pending []PlanUnit
	for _, unit := range plan.Units {
		if unit.Status == UnitStatusPending {
			pending = append(pending, unit)
		}
	}

	return pending, nil
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

// GetPlanBySession retrieves a plan by session ID.
func (s *Store) GetPlanBySession(sessionID string) (*Plan, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("store: session ID is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("store: session %q not found", sessionID)
	}

	if session.Plan == nil {
		return nil, nil
	}

	return s.copyPlanData(session.Plan), nil
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

// SaveUnitCode saves the generated code for a specific unit.
func (s *Store) SaveUnitCode(sessionID string, unitID string, code string) error {
	if sessionID == "" {
		return fmt.Errorf("store: session ID is required")
	}

	if unitID == "" {
		return fmt.Errorf("store: unit ID is required")
	}

	if code == "" {
		return fmt.Errorf("store: code is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find the plan and unit, then update the generated code
	for _, plan := range s.plans {
		if plan.SessionID == sessionID {
			for i := range plan.Units {
				if plan.Units[i].ID == unitID {
					plan.Units[i].GeneratedCode = code
					return nil
				}
			}
		}
	}

	return fmt.Errorf("store: unit %q not found in session %q", unitID, sessionID)
}

// copySessionData creates a deep copy of session data for safe read access.
func (s *Store) copySessionData(session *Session) *Session {
	if session == nil {
		return nil
	}

	copy := *session
	if session.Plan != nil {
		planCopy := s.copyPlanData(session.Plan)
		copy.Plan = planCopy
	}
	if session.AtomicUnits != nil {
		unitsCopy := make([]AtomicUnit, len(session.AtomicUnits))
		copy.AtomicUnits = append(unitsCopy, session.AtomicUnits...)
	}
	if session.History != nil {
		historyCopy := make([]PhaseHistory, len(session.History))
		copy.History = append(historyCopy, session.History...)
	}
	if session.TestResults != nil {
		testResultsCopy := make([]TestResult, len(session.TestResults))
		copy.TestResults = append(testResultsCopy, session.TestResults...)
	}
	if session.ReviewReports != nil {
		reviewReportsCopy := make([]ReviewReportEntry, len(session.ReviewReports))
		copy.ReviewReports = append(reviewReportsCopy, session.ReviewReports...)
	}

	return &copy
}

// copyPlanData creates a deep copy of plan data for safe read access.
func (s *Store) copyPlanData(plan *Plan) *Plan {
	if plan == nil {
		return nil
	}

	copy := *plan
	if plan.Units != nil {
		unitsCopy := make([]PlanUnit, len(plan.Units))
		copy.Units = append(unitsCopy, plan.Units...)
	}

	return &copy
}
