package app

import (
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func (s *Service) Reindex() (*project.ProjectIndex, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.reindexActiveProject()
}

func (s *Service) reindexActiveProject() (*project.ProjectIndex, error) {
	s.CancelChangeWorkflows()
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.clearExecutionTrust()
		s.refreshAnalysisRunAfterReindex()
	}
	return index, err
}

func (s *Service) ActivateProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Set(root, analysis) })
}

func (s *Service) RestoreActiveProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Restore(root, analysis) })
}

func (s *Service) replaceActiveProject(activate func() error) error {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if err := s.closeWorktreeServices(); err != nil {
		return err
	}
	s.invalidateJobsForProjectChange()
	return activate()
}

func (s *Service) invalidateJobsForProjectChange() {
	s.CancelChangeWorkflows()
	s.interruptAnalysisRunForProjectChange()
	s.cancelGoScan()
	s.clearExecutionTrust()
}

func (s *Service) cancelGoScan() {
	s.goScan.mu.Lock()
	cancel := s.goScan.cancel
	s.goScan.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func isSemanticAnalysisCandidate(file project.IndexFile) bool {
	return !file.Binary && file.Language != "Text" && file.Language != "Markdown"
}
