package app

import "context"

// Reindex owns jobLifecycleMu. A progress write fault remains available through
// run reads and controls without hiding the successfully refreshed index.
func (s *Service) refreshAnalysisRunAfterReindex() {
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.run != nil && c.root == s.manager.Root() {
		_ = s.refreshAnalysisRunFreshnessLocked(context.Background())
	}
}

// The caller holds both lifecycle and controller locks.
func (s *Service) refreshAnalysisRunFreshnessLocked(ctx context.Context) error {
	c := s.analysisRun
	stale := c.run.Status == AnalysisRunStale
	if stale && (c.fault != nil || !analysisRunHasTerminalEvidence(c.run)) {
		return nil
	}
	if err := s.validateAnalysisQueue(ctx, c.root, &c.run.Plan, true); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !stale {
			s.staleAnalysisRunLocked()
			return s.saveAnalysisRunLocked(false)
		}
		return nil
	}
	if stale {
		// Older versions marked finished runs stale on restore/reindex even
		// without source changes. Recover only terminal evidence whose entire
		// captured queue still matches; incomplete work keeps its lifecycle.
		c.run.Status = analysisFinishedStatus(c.run)
		c.run.Reason = ""
		refreshAnalysisSections(c.run)
	}
	return nil
}

func analysisRunHasTerminalEvidence(run *AnalysisRun) bool {
	evidence := false
	for _, file := range run.Files {
		for _, stage := range file.Stages {
			switch stage.Status {
			case AnalysisStageCompleted, AnalysisStageCompletedEmpty, AnalysisStagePartial:
				evidence = true
			case AnalysisStageFailed, AnalysisStageUnavailable:
			default:
				return false
			}
		}
	}
	return evidence
}
