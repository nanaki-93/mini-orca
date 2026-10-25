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
	if err := s.stopTriggersForLifecycle(); err != nil {
		return nil, err
	}
	if err := s.stopLiveSessions(); err != nil {
		return nil, err
	}
	// Synchronous Git effects hold jobLifecycleMu. An integration's own reindex
	// must preserve its cancellation context through observation and cleanup.
	s.cancelMissionWaves()
	s.cancelIntegrations()
	s.CancelChangeWorkflows()
	s.cancelGoScan()
	index, err := s.manager.Reindex()
	if err == nil {
		s.resetExecutionTrust(true)
		s.refreshAnalysisRunAfterReindex()
	}
	return index, err
}

func (s *Service) ActivateProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error {
		if err := s.manager.Set(root, analysis); err != nil {
			return err
		}
		s.persistProjectDocs()
		return nil
	})
}

func (s *Service) RestoreActiveProject(root string, analysis *project.Analysis) error {
	return s.replaceActiveProject(func() error { return s.manager.Restore(root, analysis) })
}

func (s *Service) replaceActiveProject(activate func() error) error {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	if err := s.stopTriggersForLifecycle(); err != nil {
		return err
	}
	s.cancelMissionJobs()
	if err := s.closeWorktreeServices(); err != nil {
		return err
	}
	if err := s.stopLiveSessions(); err != nil {
		return err
	}
	s.invalidateJobsForProjectChange()
	if err := activate(); err != nil {
		return err
	}
	s.resetExecutionTrust(true)
	return nil
}

func (s *Service) invalidateJobsForProjectChange() {
	s.cancelMissionJobs()
	s.cancelIntegrations()
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
