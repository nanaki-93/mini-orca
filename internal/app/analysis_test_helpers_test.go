package app

import "context"

// currentTestAnalysisRun can restore metadata but never starts a worker or reuses consent.
// A recoverable persistence fault returns the retained progress alongside its error.
func (s *Service) currentTestAnalysisRun(ctx context.Context) (*AnalysisRun, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.currentAnalysisRunLocked(ctx)
}
