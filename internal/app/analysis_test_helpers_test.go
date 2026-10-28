package app

import (
	"context"
	"testing"
)

// currentTestAnalysisRun can restore metadata but never starts a worker or reuses consent.
// A recoverable persistence fault returns the retained progress alongside its error.
func (s *Service) currentTestAnalysisRun(ctx context.Context) (*AnalysisRun, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	return s.currentAnalysisRunLocked(ctx)
}

// scopedFiles are the files a scoped preview selects; the rest are retained.
func scopedFiles(preview *AnalysisRunPreview) []AnalysisPlannedFile {
	selected := []AnalysisPlannedFile{}
	for _, file := range preview.Files {
		if !file.Retained {
			selected = append(selected, file)
		}
	}
	return selected
}

func previewRetained(preview *AnalysisRunPreview, path string) bool {
	for _, file := range preview.Files {
		if file.Path == path {
			return file.Retained
		}
	}
	return false
}

func analysisRunStage(t *testing.T, run *AnalysisRun, path string, stage AnalysisStage) AnalysisStageProgress {
	t.Helper()
	for _, file := range run.Files {
		if file.Path != path {
			continue
		}
		for _, progress := range file.Stages {
			if progress.Stage == stage {
				return progress
			}
		}
	}
	t.Fatalf("%s %s is not in the run", path, stage)
	return AnalysisStageProgress{}
}
