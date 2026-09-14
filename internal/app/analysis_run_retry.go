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
	retained := make(map[string]AnalysisPlannedFile)
	if run := s.analysisRun.run; run != nil && run.Identity.ProjectID == preview.Identity.ProjectID {
		for _, file := range run.Plan.Files {
			retained[file.Path] = file
		}
	}
	evidence := selectionEvidence(s.analysisRun.run, *analysis)
	selected := []AnalysisPlannedFile{}
	for _, file := range preview.Files {
		if err := ctx.Err(); err != nil {
			return err
		}
		previous, captured := retained[file.Path]
		include := captured
		if resume == nil {
			include = s.planAnalysisRetryFile(*analysis, indexed[file.Path], policy, &file, evidence, preview.Limits)
		}
		if resume != nil && include {
			for j := range file.Stages {
				if !previous.Stages[j].Cached {
					s.planAnalysisRetryStage(&file.Stages[j], preview.Limits)
				}
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

func (s *Service) planAnalysisRetryFile(analysis project.Analysis, indexed project.IndexFile, policy *project.ContextPolicy, file *AnalysisPlannedFile, evidence analysisSelectionEvidence, limits AnalysisRunLimits) bool {
	include := false
	for i := range file.Stages {
		stage := &file.Stages[i]
		if !stage.Eligible {
			continue
		}
		status := s.analysisSelectionStage(analysis, indexed, policy, stage.Stage, evidence).Status
		if status == "stale" || status == "failed" {
			include = true
			s.planAnalysisRetryStage(stage, limits)
		}
	}
	return include
}

func (s *Service) planAnalysisRetryStage(stage *AnalysisStagePlan, limits AnalysisRunLimits) {
	// Retain this cache bypass in the admitted plan, including across resume.
	stage.Cached = false
	if !stage.Eligible || stage.Stage == AnalysisStageSecurityRules || !s.analysisStageModelAvailable(stage.Stage) {
		return
	}
	runtime := s.runtimes.analyze
	if stage.Stage == AnalysisStageSemantic {
		runtime = s.runtimes.bug
	}
	stage.MaxModelRequests = min(limits.MaxAttemptsPerStage, runtime.effective.MaxRetries+1)
}
