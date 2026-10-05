package app

import (
	"context"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const analysisRetryExclusion = "No stale or failed analysis for this file."

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
			include = s.planAnalysisRetryFile(*analysis, indexed[file.Path], policy, &file, evidence, preview.Limits, preview.Models)
		}
		if resume != nil && include {
			for j := range file.Stages {
				if !previous.Stages[j].Cached {
					s.planAnalysisRetryStage(&file.Stages[j], preview.Limits, preview.Models)
				}
			}
		}
		if include {
			selected = append(selected, file)
		} else {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: analysisRetryExclusion})
		}
	}
	preview.Files = selected
	return nil
}

func (s *Service) planAnalysisRetryFile(analysis project.Analysis, indexed project.IndexFile, policy *project.ContextPolicy, file *AnalysisPlannedFile, evidence analysisSelectionEvidence, limits AnalysisRunLimits, models *AnalysisModels) bool {
	include := false
	for i := range file.Stages {
		stage := &file.Stages[i]
		if !stage.Eligible {
			continue
		}
		status := s.analysisSelectionStageForModels(analysis, indexed, policy, stage.Stage, evidence, models).Status
		if status == "stale" || status == "failed" {
			include = true
			s.planAnalysisRetryStage(stage, limits, models)
		}
	}
	return include
}

func (s *Service) planAnalysisRetryStage(stage *AnalysisStagePlan, limits AnalysisRunLimits, models *AnalysisModels) {
	// Retain this cache bypass in the admitted plan, including across resume.
	stage.Cached = false
	if !stage.Eligible || stage.Stage == AnalysisStageSecurityRules || s.analysisModelRuntime(stage.Stage, models).client == nil {
		return
	}
	runtime := s.analysisModelRuntime(stage.Stage, models)
	stage.MaxModelRequests = min(limits.MaxAttemptsPerStage, runtime.effective.MaxRetries+1)
}
