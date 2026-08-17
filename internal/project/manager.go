package project

import (
	"fmt"
	"path/filepath"
	"sync"
)

// Manager owns the active project selected by the UI.
type Manager struct {
	mu       sync.RWMutex
	root     string
	analysis *Analysis
	sessions *sessionStore
	index    *ProjectIndex
}

func NewManager(initialRoot string) (*Manager, error) {
	root, err := CanonicalRoot(initialRoot)
	if err != nil {
		return nil, err
	}
	return &Manager{root: root}, nil
}

func (m *Manager) Root() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.root
}

func (m *Manager) Set(root string, analysis *Analysis) error {
	if analysis == nil {
		return fmt.Errorf("project analysis is required")
	}
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return err
	}
	id := projectID(canonical)
	revision, err := projectRevision(canonical)
	if err != nil {
		return err
	}
	sessions, err := newSessionStore(canonical, id, revision)
	if err != nil {
		return err
	}
	index, err := BuildIndex(canonical, id, revision)
	if err != nil {
		return err
	}
	analysis.ProjectID = id
	analysis.ProjectRevision = revision
	m.mu.Lock()
	defer m.mu.Unlock()
	m.root = canonical
	m.analysis = analysis
	m.sessions = sessions
	m.index = index
	return nil
}

// Reindex refreshes deterministic facts without contacting the model.
func (m *Manager) Reindex() (*ProjectIndex, error) {
	m.mu.RLock()
	if m.analysis == nil {
		m.mu.RUnlock()
		return nil, ErrNoActiveProject
	}
	root := m.root
	id := m.analysis.ProjectID
	m.mu.RUnlock()
	revision, err := projectRevision(root)
	if err != nil {
		return nil, err
	}
	index, err := BuildIndex(root, id, revision)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.root != root || m.analysis == nil || m.analysis.ProjectID != id {
		return nil, ErrRevisionConflict
	}
	m.analysis.ProjectRevision = revision
	m.index = index
	return cloneIndex(index), nil
}

// Index returns a copy of the active deterministic project index.
func (m *Manager) Index() (*ProjectIndex, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.index == nil {
		return nil, ErrNoActiveProject
	}
	return cloneIndex(m.index), nil
}

// IndexedFile returns one active-project file after canonical-path and policy
// checks. The final lock check prevents a file from a previously active project
// being returned after a concurrent import or reindex.
func (m *Manager) IndexedFile(relative string) (*IndexFile, error) {
	m.mu.RLock()
	if m.index == nil || m.analysis == nil {
		m.mu.RUnlock()
		return nil, ErrNoActiveProject
	}
	root := m.root
	revision := m.index.ProjectRevision
	m.mu.RUnlock()

	policy, err := NewContextPolicy(root)
	if err != nil {
		return nil, err
	}
	decision := policy.Decide(relative)
	if decision.Reason == "unsafe path" {
		return nil, fmt.Errorf("invalid indexed file path: %s", relative)
	}
	if !decision.Include {
		return nil, fmt.Errorf("%w: %s", ErrExcludedFile, decision.Reason)
	}
	resolved, err := ResolveFile(root, relative)
	if err != nil {
		return nil, err
	}
	normalized, err := filepath.Rel(root, resolved)
	if err != nil {
		return nil, fmt.Errorf("normalize indexed file path: %w", err)
	}
	normalized = filepath.ToSlash(normalized)

	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.root != root || m.index == nil || m.index.ProjectRevision != revision {
		return nil, ErrRevisionConflict
	}
	for _, file := range m.index.Files {
		if file.Path == normalized {
			copy := file
			copy.Imports = append([]string(nil), file.Imports...)
			copy.Symbols = append([]SymbolInfo(nil), file.Symbols...)
			copy.Diagnostics = append([]Diagnostic(nil), file.Diagnostics...)
			return &copy, nil
		}
	}
	return nil, fmt.Errorf("indexed file not found: %s", normalized)
}

func (m *Manager) Analysis() (*Analysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.analysis == nil {
		return nil, ErrNoActiveProject
	}
	copy := *m.analysis
	copy.Languages = cloneMap(m.analysis.Languages)
	copy.Files = append([]string(nil), m.analysis.Files...)
	return &copy, nil
}

// ValidateMutableRequest rejects stale projects and files before candidate work.
func (m *Manager) ValidateMutableRequest(id, revision, relativePath, baseFileHash string) error {
	if id == "" || revision == "" || baseFileHash == "" {
		return fmt.Errorf("project_id, project_revision, and base_file_hash are required")
	}
	m.mu.RLock()
	if m.analysis == nil {
		m.mu.RUnlock()
		return ErrNoActiveProject
	}
	root := m.root
	currentID := m.analysis.ProjectID
	currentRevision := m.analysis.ProjectRevision
	m.mu.RUnlock()
	if id != currentID || revision != currentRevision {
		return ErrRevisionConflict
	}
	fullPath, err := ResolveFile(root, relativePath)
	if err != nil {
		return err
	}
	currentHash, err := hashFile(fullPath)
	if err != nil {
		return err
	}
	if currentHash != baseFileHash {
		return ErrRevisionConflict
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.root != root || m.analysis == nil || m.analysis.ProjectID != id || m.analysis.ProjectRevision != revision {
		return ErrRevisionConflict
	}
	return nil
}

// RecordActivity writes sanitized activity only for the active project.
func (m *Manager) RecordActivity(activity Activity) error {
	m.mu.RLock()
	sessions := m.sessions
	m.mu.RUnlock()
	if sessions == nil {
		return ErrNoActiveProject
	}
	return sessions.append(activity)
}

// RecordActivityFor keeps a request's activity attached to the project revision
// that accepted it, even if another project is imported concurrently.
func (m *Manager) RecordActivityFor(id, revision string, activity Activity) error {
	m.mu.RLock()
	if m.analysis == nil || m.analysis.ProjectID != id || m.analysis.ProjectRevision != revision {
		m.mu.RUnlock()
		return ErrRevisionConflict
	}
	sessions := m.sessions
	m.mu.RUnlock()
	if sessions == nil {
		return ErrNoActiveProject
	}
	return sessions.append(activity)
}

// Activity returns only the active project's durable activity.
func (m *Manager) Activity() ([]Activity, error) {
	m.mu.RLock()
	sessions := m.sessions
	m.mu.RUnlock()
	if sessions == nil {
		return nil, ErrNoActiveProject
	}
	return sessions.history(), nil
}

func cloneMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func cloneIndex(source *ProjectIndex) *ProjectIndex {
	copy := *source
	copy.Files = make([]IndexFile, len(source.Files))
	for i, file := range source.Files {
		copy.Files[i] = file
		copy.Files[i].Imports = append([]string(nil), file.Imports...)
		copy.Files[i].Symbols = append([]SymbolInfo(nil), file.Symbols...)
		copy.Files[i].Diagnostics = append([]Diagnostic(nil), file.Diagnostics...)
	}
	return &copy
}
