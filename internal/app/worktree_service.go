package app

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// WorktreeService keeps proposal authority, jobs and execution trust local to a
// registered checkout without replacing the project selected in the desktop.
func (s *Service) WorktreeService(ctx context.Context, path string) (*Service, *project.Manager, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if _, err := s.manager.Index(); err != nil {
		return nil, nil, err
	}
	if !filepath.IsAbs(path) {
		return nil, nil, fmt.Errorf("choose an absolute worktree path")
	}
	canonical, err := project.CanonicalRoot(path)
	if err != nil {
		return nil, nil, err
	}
	root := s.manager.Root()
	trees, err := readGitWorktrees(ctx, root)
	if err != nil {
		return nil, nil, err
	}
	found := false
	for _, tree := range trees {
		if tree.Path == canonical && !tree.Prunable {
			found = true
		}
	}
	if !found || canonical == root {
		return nil, nil, fmt.Errorf("choose another registered worktree in the current repository")
	}
	s.worktreesMu.Lock()
	defer s.worktreesMu.Unlock()
	if child := s.worktrees[canonical]; child != nil {
		return child, child.manager, nil
	}
	manager, err := project.NewManager(canonical)
	if err != nil {
		return nil, nil, err
	}
	child, err := New(s.config, manager)
	if err != nil {
		return nil, nil, err
	}
	analysis, err := child.RestoreProject(canonical)
	if err != nil {
		return nil, nil, err
	}
	if err := manager.Restore(canonical, analysis); err != nil {
		return nil, nil, err
	}
	s.worktrees[canonical] = child
	return child, manager, nil
}

func (s *Service) worktreeServices() []*Service {
	s.worktreesMu.Lock()
	defer s.worktreesMu.Unlock()
	children := make([]*Service, 0, len(s.worktrees))
	for _, child := range s.worktrees {
		children = append(children, child)
	}
	return children
}

func (s *Service) closeWorktreeServices() error {
	children := s.worktreeServices()
	for _, child := range children {
		child.invalidateJobsForProjectChange()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, child := range children {
		if err := child.ShutdownChangeWorkflows(ctx); err != nil {
			return fmt.Errorf("wait for worktree cleanup before switching projects: %w", err)
		}
	}
	s.worktreesMu.Lock()
	s.worktrees = make(map[string]*Service)
	s.worktreesMu.Unlock()
	return nil
}
