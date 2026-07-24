package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store manages session persistence.
type Store struct {
	baseDir string
	mu      sync.RWMutex
}

// NewStore creates a new state store.
func NewStore(baseDir string) (*Store, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	return &Store{
		baseDir: baseDir,
	}, nil
}

// CreateSession creates a new session and persists it.
func (s *Store) CreateSession(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ID = uuid.New().String()
	session.CreatedAt = time.Now()
	session.UpdatedAt = time.Now()

	if err := session.ValidateSession(); err != nil {
		return err
	}

	return s.saveSession(session)
}

// GetSession retrieves a session by ID.
func (s *Store) GetSession(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session := &Session{}
	data, err := os.ReadFile(s.sessionPath(id))
	if err != nil {
		return nil, fmt.Errorf("failed to read session: %w", err)
	}

	if err := json.Unmarshal(data, session); err != nil {
		return nil, fmt.Errorf("failed to parse session: %w", err)
	}

	return session, nil
}

// UpdateSession updates an existing session.
func (s *Store) UpdateSession(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.UpdatedAt = time.Now()
	return s.saveSession(session)
}

// ListSessions returns all sessions.
func (s *Store) ListSessions() ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read state directory: %w", err)
	}

	var sessions []*Session
	for _, file := range files {
		// Sessions are stored in subdirectories named by session ID
		if file.IsDir() {
			sessionPath := filepath.Join(s.baseDir, file.Name(), "session.json")
			session, err := s.GetSession(file.Name())
			if err != nil {
				continue
			}
			// Verify the session file actually exists
			if _, err := os.Stat(sessionPath); err == nil {
				sessions = append(sessions, session)
			}
		}
	}

	return sessions, nil
}

// DeleteSession deletes a session.
func (s *Store) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.sessionPath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// SavePlan persists the plan to the session directory.
func (s *Store) SavePlan(plan *Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Create session directory if it doesn't exist
	sessionDir := filepath.Join(s.baseDir, plan.SessionID)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	path := filepath.Join(sessionDir, "plan.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	return nil
}

// LoadPlan loads a plan from the session directory.
func (s *Store) LoadPlan(sessionID string) (*Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path := filepath.Join(s.baseDir, sessionID, "plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read plan: %w", err)
	}

	plan := &Plan{}
	if err := json.Unmarshal(data, plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	return plan, nil
}

// sessionPath returns the path to a session file.
func (s *Store) sessionPath(id string) string {
	// Create session directory if it doesn't exist
	dir := filepath.Join(s.baseDir, id)
	os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "session.json")
}

// saveSession saves a session to disk.
func (s *Store) saveSession(session *Session) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	path := s.sessionPath(session.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write session: %w", err)
	}

	return nil
}
