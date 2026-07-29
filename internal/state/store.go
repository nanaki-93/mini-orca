// Package state provides persistence and retrieval of orchestrator state.
package state

import (
	"fmt"
	"sync"
	"time"
)

// Store manages the state of the orchestrator pipeline.
type Store struct {
	mu      sync.RWMutex
	session *Session
}

// NewStore creates a new Store instance with the given session.
func NewStore(session *Session) *Store {
	return &Store{
		session: session,
	}
}

// GetSession returns a copy of the current session.
func (s *Store) GetSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.copySession()
}

// UpdateSession updates the session with the provided session.
func (s *Store) UpdateSession(session *Session) error {
	if session == nil {
		return fmt.Errorf("store: session is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.session = s.copySession()
	s.session.ID = session.ID
	s.session.Goal = session.Goal
	s.session.ProjectPath = session.ProjectPath
	s.session.ProjectType = session.ProjectType
	s.session.CurrentPhase = session.CurrentPhase
	s.session.Status = session.Status
	s.session.UpdatedAt = time.Now()

	return nil
}

// SavePlan saves the plan to the session.
func (s *Store) SavePlan(plan *Plan) error {
	if plan == nil {
		return fmt.Errorf("store: plan is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.session.Plan = plan
	s.session.UpdatedAt = time.Now()

	return nil
}

// GetPlan returns the current plan from the session.
func (s *Store) GetPlan() *Plan {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.session == nil || s.session.Plan == nil {
		return nil
	}

	return s.session.Plan
}

// AddPhaseHistory adds a phase history entry to the session.
func (s *Store) AddPhaseHistory(history *PhaseHistory) error {
	if history == nil {
		return fmt.Errorf("store: phase history is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.session.History = append(s.session.History, *history)
	s.session.UpdatedAt = time.Now()

	return nil
}

// GetPhaseHistory returns all phase history entries.
func (s *Store) GetPhaseHistory() []PhaseHistory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.session == nil {
		return nil
	}

	return s.session.History
}

// UpdatePhaseStatus updates the session's current phase and status.
func (s *Store) UpdatePhaseStatus(phase Phase, status SessionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.session.CurrentPhase = phase
	s.session.Status = status
	s.session.UpdatedAt = time.Now()

	return nil
}

// SetError sets the session error.
func (s *Store) SetError(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err != nil {
		s.session.Error = err.Error()
		s.session.Status = SessionStatusFailed
	}
	s.session.UpdatedAt = time.Now()
}

// copySession creates a shallow copy of the session for safe read access.
func (s *Store) copySession() *Session {
	if s.session == nil {
		return nil
	}

	copy := *s.session
	if s.session.Plan != nil {
		planCopy := *s.session.Plan
		copy.Plan = &planCopy
	}
	if s.session.AtomicUnits != nil {
		unitsCopy := make([]AtomicUnit, len(s.session.AtomicUnits))
		copy.AtomicUnits = append(unitsCopy, s.session.AtomicUnits...)
	}
	if s.session.History != nil {
		historyCopy := make([]PhaseHistory, len(s.session.History))
		copy.History = append(historyCopy, s.session.History...)
	}

	return &copy
}
