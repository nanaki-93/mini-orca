package project

import (
	"fmt"
	"sync"
)

// Manager owns the active project selected by the UI.
type Manager struct {
	mu       sync.RWMutex
	root     string
	analysis *Analysis
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
	canonical, err := CanonicalRoot(root)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.root = canonical
	m.analysis = analysis
	return nil
}

func (m *Manager) Analysis() (*Analysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.analysis == nil {
		return nil, fmt.Errorf("active project has not been analyzed")
	}
	copy := *m.analysis
	copy.Languages = cloneMap(m.analysis.Languages)
	copy.Files = append([]string(nil), m.analysis.Files...)
	return &copy, nil
}

func cloneMap(source map[string]int) map[string]int {
	result := make(map[string]int, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
