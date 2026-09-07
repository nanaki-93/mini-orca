package app

import (
	"fmt"
	"regexp"
	"sync"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ExecutionTrust is the explicit, in-memory authorization to run imported
// project code for one active project revision. It is deliberately unrelated
// to model-provider confirmation.
type ExecutionTrust struct {
	ProjectID       string     `json:"project_id"`
	ProjectRevision string     `json:"project_revision"`
	Trusted         bool       `json:"trusted"`
	Commands        [][]string `json:"commands"`
}

type executionTrustStore struct {
	mu       sync.Mutex
	project  string
	revision string
}

func newExecutionTrustStore() *executionTrustStore { return &executionTrustStore{} }

func projectCodeCommands(taskName string) ([][]string, error) {
	if taskName == "" {
		return [][]string{{"go", "test", "./..."}}, nil
	}
	if !validGoTestName(taskName) {
		return nil, fmt.Errorf("task test name is invalid")
	}
	return [][]string{{"go", "test", "./...", "-run", "^" + regexp.QuoteMeta(taskName) + "$"}}, nil
}

func (s *Service) ExecutionTrust(revision, taskName string) (*ExecutionTrust, error) {
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if revision == "" || revision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	s.executionTrust.mu.Lock()
	trusted := s.executionTrust.project == analysis.ProjectID && s.executionTrust.revision == analysis.ProjectRevision
	s.executionTrust.mu.Unlock()
	commands, err := projectCodeCommands(taskName)
	if err != nil {
		return nil, err
	}
	return &ExecutionTrust{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Trusted: trusted, Commands: commands}, nil
}

// TrustProjectExecution records the user's current-session consent after the
// client has shown the fixed command scope returned by ExecutionTrust.
func (s *Service) TrustProjectExecution(revision string, confirm bool) (*ExecutionTrust, error) {
	if !confirm {
		return nil, fmt.Errorf("trusted local execution requires explicit confirmation")
	}
	trust, err := s.ExecutionTrust(revision, "")
	if err != nil {
		return nil, err
	}
	s.executionTrust.mu.Lock()
	s.executionTrust.project = trust.ProjectID
	s.executionTrust.revision = trust.ProjectRevision
	s.executionTrust.mu.Unlock()
	trust.Trusted = true
	return trust, nil
}

func (s *Service) requireProjectExecutionTrust(revision string) error {
	trust, err := s.ExecutionTrust(revision, "")
	if err != nil {
		return err
	}
	if !trust.Trusted {
		return fmt.Errorf("trusted local execution is required before running project code")
	}
	return nil
}

func (s *Service) clearExecutionTrust() {
	s.executionTrust.mu.Lock()
	s.executionTrust.project = ""
	s.executionTrust.revision = ""
	s.executionTrust.mu.Unlock()
}
