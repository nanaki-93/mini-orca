// Package state provides persistence and retrieval of orchestrator state.
package state

import (
	"sync"
)

// Store manages the state of the orchestrator pipeline.
type Store struct {
	mu           sync.RWMutex
	currentPhase string
	status       string
	error        string
}

// NewStore creates a new Store instance.
func NewStore() *Store {
	return &Store{
		currentPhase: "idle",
		status:       "pending",
	}
}

// InitStateStore creates and initializes a new Store for single-session use.
func InitStateStore() *Store {
	return NewStore()
}

// UpdatePhaseStatus updates the current phase and status.
func (s *Store) UpdatePhaseStatus(phase, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentPhase = phase
	s.status = status
}

// SetError sets the error message on the store.
func (s *Store) SetError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.error = err
}

// GetCurrentPhase returns the current phase.
func (s *Store) GetCurrentPhase() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPhase
}

// GetStatus returns the current status.
func (s *Store) GetStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// GetError returns the current error message.
func (s *Store) GetError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.error
}
