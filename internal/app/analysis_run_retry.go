package app

import (
	"context"
	"fmt"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const analysisStaleExclusion = "No stale analysis for this file."
const analysisStalePathExclusion = "Outside the selected stale file."
const analysisRetryExclusion = "No stale or failed analysis for this file."
const analysisRecoveryExclusion = "No incomplete analysis for this file."

// analysisScopedSelection narrows a new admission to files with earlier work to finish.
type analysisScopedSelection uint8

const (
	analysisSelectAll analysisScopedSelection = iota
	analysisSelectStale
	// analysisSelectStaleFailed keeps the established retry_stale_failed contract.
	analysisSelectStaleFailed
	// analysisSelectIncomplete selects every recoverable stage for recover_incomplete.
	analysisSelectIncomplete
)

func analysisSelectionMode(retryStaleFailed, recoverIncomplete, staleOnly bool) analysisScopedSelection {
	switch {
	case staleOnly:
		return analysisSelectStale
	case recoverIncomplete:
		return analysisSelectIncomplete
	case retryStaleFailed:
		return analysisSelectStaleFailed
	default:
		return analysisSelectAll
	}
}

// exclusion is the reason recorded for inventory files that the selection omits.
func (mode analysisScopedSelection) exclusion() string {
	switch mode {
	case analysisSelectStale:
		return analysisStaleExclusion
	case analysisSelectStaleFailed:
		return analysisRetryExclusion
	case analysisSelectIncomplete:
		return analysisRecoveryExclusion
	default:
		return ""
	}
}

func (mode analysisScopedSelection) selects(status string, configured bool) bool {
	switch mode {
	case analysisSelectStale:
		return status == "stale"
	case analysisSelectStaleFailed:
		return status == "stale" || status == "failed"
	case analysisSelectIncomplete:
		return analysisStageRecoveryFor(status, configured) == analysisStageRecoverable
	default:
		return false
	}
}

// analysisStageRecovery classifies an eligible stage observation for recovery.
// Source changes and unreadable files block the whole file before classification.
type analysisStageRecovery uint8

const (
	// analysisStageSettled is current evidence or work owned by an active/continuable run.
	analysisStageSettled analysisStageRecovery = iota
	analysisStageRecoverable
	// analysisStageNeedsModel is unfinished work that a retry cannot dispatch.
	analysisStageNeedsModel
)

// analysisStageRecoveryFor is the single recovery rule shared by admission and the
// selection summary. Pending, running and paused stages belong to their run.
func analysisStageRecoveryFor(status string, configured bool) analysisStageRecovery {
	switch status {
	case "stale", "failed", "partial", "missing", "canceled", "interrupted", "unavailable":
		if !configured {
			return analysisStageNeedsModel
		}
		return analysisStageRecoverable
	default:
		return analysisStageSettled
	}
}

// Passive Security rules need no model; every other stage needs a configured client.
func (s *Service) analysisStageConfigured(stage AnalysisStage, models *AnalysisModels) bool {
	return stage == AnalysisStageSecurityRules || s.analysisModelRuntime(stage, models).client != nil
}

// Resume keeps the admitted file set even as this run replaces stale reports.
// New starts recompute selection, so changed evidence invalidates an old preview.
func (s *Service) scopeAnalysisRetryPreview(ctx context.Context, mode analysisScopedSelection, preview *AnalysisRunPreview, resume *AnalysisRunIdentity) error {
	if err := scopeAnalysisStalePath(preview); err != nil {
		return err
	}
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
			include = s.planAnalysisRetryFile(mode, *analysis, indexed[file.Path], policy, &file, evidence, preview.Limits, preview.Models)
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
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: mode.exclusion()})
		}
	}
	preview.Files = selected
	return nil
}

func scopeAnalysisStalePath(preview *AnalysisRunPreview) error {
	if preview.StalePath == "" {
		return nil
	}
	selected := []AnalysisPlannedFile{}
	for _, file := range preview.Files {
		if file.Path == preview.StalePath {
			selected = append(selected, file)
		} else {
			preview.Excluded = append(preview.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: analysisStalePathExclusion})
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("%w: stale file is not in the eligible saved selection", project.ErrRevisionConflict)
	}
	preview.Files = selected
	return nil
}

func (s *Service) planAnalysisRetryFile(mode analysisScopedSelection, analysis project.Analysis, indexed project.IndexFile, policy *project.ContextPolicy, file *AnalysisPlannedFile, evidence analysisSelectionEvidence, limits AnalysisRunLimits, models *AnalysisModels) bool {
	include := false
	for i := range file.Stages {
		stage := &file.Stages[i]
		if !stage.Eligible {
			continue
		}
		status := s.analysisSelectionStageForModels(analysis, indexed, policy, stage.Stage, evidence, models).Status
		if mode.selects(status, s.analysisStageConfigured(stage.Stage, models)) {
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
