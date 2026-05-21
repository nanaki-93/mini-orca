package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu       sync.RWMutex
	filePath string
}

func NewFileStore(path string) *Store {
	if path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, path[2:])
	}
	return &Store{
		filePath: path,
	}
}

func (s *Store) Save(ctx *ProjectContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create store directories: %w", err)
	}

	data, err := json.MarshalIndent(ctx, "", "	")
	if err != nil {
		return fmt.Errorf("failed to marshal project state context: %w", err)
	}
	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write project state to disk: %w", err)
	}
	return nil
}

func (s *Store) Load() (*ProjectContext, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		return &ProjectContext{CurrentState: StateAnalysis, Tasks: make([]MicroTask, 0)}, nil
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project state database file: %w", err)
	}
	var ctx ProjectContext
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("failed to unmarshal project state file database context: %w", err)
	}
	return &ctx, nil
}
