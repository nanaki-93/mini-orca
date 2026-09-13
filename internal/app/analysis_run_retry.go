package app

import (
	"context"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// Resume keeps the admitted file set even as this run replaces stale reports.
// New starts recompute selection, so changed evidence invalidates an old preview.
func (s *Service) scopeAnalysisRetryPreview(ctx context.Context, preview *AnalysisRunPreview, resume *AnalysisRunIdentity) error {
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		return err
	}
	indexed := make(map[string]project.IndexFile, len(index.Files))
	for _, file := range index.Files {
		indexed[file.Path] = file
	}
	retained := make(map[string]AnalysisRunFile)
	if run := s.analysisRun.run; run != nil && run.Identity.ProjectID == preview.Identity.ProjectID {
		for _, file := range run.Files {
			retained[file.Path] = file
		}
	}
	selected := []AnalysisPlannedFile{}
	for _, file := range preview.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		previous, captured := retained[file.Path]
		include := captured
		if resume == nil {
			include, err = s.analysisFileNeedsRetry(*analysis, indexed[file.Path], policy, file, previous)
			if err != nil {
				return err
			}
		}
		if include {
			selected = append(selected, file)
		} else {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: "No stale or failed analysis for this file."})
		}
	}
	preview.Files = selected
	return nil
}

func (s *Service) analysisFileNeedsRetry(analysis project.Analysis, indexed project.IndexFile, policy *project.ContextPolicy, file AnalysisPlannedFile, previous AnalysisRunFile) (bool, error) {
	for _, stage := range file.Stages {
		if !stage.Eligible || stage.Cached {
			continue
		}
		_, status, err := s.analysisStageCacheState(analysis, indexed, policy, stage.Stage)
		if err != nil {
			return false, err
		}
		if status == "stale" || status == "failed" {
			return true, nil
		}
		if status != "" && status != project.AnalysisStatusMissing {
			continue
		}
		// Transport failures can leave no producer report. Durable stage failures
		// still qualify, while missing, partial and canceled work remain distinct.
		for _, progress := range previous.Stages {
			if progress.Stage == stage.Stage && (progress.Status == AnalysisStageFailed || progress.Status == AnalysisStageStale) {
				return true, nil
			}
		}
	}
	return false, nil
}
