package app

import (
	"context"
	"fmt"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// ErrAnalysisLegacyMigration prevents old file-call counters from being treated
// as fresh transport authority. Historical files remain readable and untouched.
var ErrAnalysisLegacyMigration = fmt.Errorf("%w: saved legacy job requires an explicit new start; its progress is retained", project.ErrRevisionConflict)

func compatibilityProviders(providers []AnalysisProviderRequirement, stage AnalysisStage) []AnalysisProviderRequirement {
	if stage == "" {
		return providers
	}
	result := []AnalysisProviderRequirement{}
	for _, provider := range providers {
		for _, candidate := range provider.Stages {
			if candidate == stage {
				provider.Stages = []AnalysisStage{stage}
				result = append(result, provider)
				break
			}
		}
	}
	return result
}

func (s *Service) scopeCompatibilityPreview(plan *AnalysisRunPreview, resume *AnalysisRunIdentity) error {
	stage := plan.CompatibilityStage
	if stage != AnalysisStageSemantic && stage != AnalysisStagePerformance {
		return fmt.Errorf("invalid compatibility stage")
	}
	captured := map[string]bool{}
	if resume != nil {
		for _, file := range s.analysisRun.run.Plan.Files {
			captured[file.Path] = true
		}
	}
	selected := []AnalysisPlannedFile{}
	for _, file := range plan.Files {
		var requested AnalysisStagePlan
		for _, candidate := range file.Stages {
			if candidate.Stage == stage {
				requested = candidate
			}
		}
		include := requested.Eligible && len(selected) < plan.Limits.BatchFiles && (stage != AnalysisStageSemantic || !requested.Cached)
		if resume != nil {
			include = captured[file.Path]
		}
		if !include {
			plan.Excluded = append(plan.Excluded, AnalysisExcludedFile{Path: file.Path, Reason: "Outside this compatibility queue."})
			continue
		}
		for i := range file.Stages {
			if file.Stages[i].Stage != stage {
				file.Stages[i] = AnalysisStagePlan{Stage: file.Stages[i].Stage, Reason: "Not requested by this compatibility action."}
			}
		}
		selected = append(selected, file)
	}
	plan.Files = selected
	plan.Providers = compatibilityProviders(plan.Providers, stage)
	fingerprint, err := analysisFingerprint(plan.Providers)
	plan.Identity.ProviderFingerprint = fingerprint
	return err
}

func validAnalysisCompatibility(run *AnalysisRun) bool {
	plan := run.Plan
	if plan.CompatibilityStage == "" {
		return plan.CompatibilityBudget == 0 && run.CompatibilityElapsed == 0
	}
	if plan.CompatibilityStage != AnalysisStageSemantic && plan.CompatibilityStage != AnalysisStagePerformance {
		return false
	}
	if len(plan.Files) > plan.Limits.BatchFiles {
		return false
	}
	if plan.CompatibilityStage == AnalysisStageSemantic && (plan.CompatibilityBudget != 0 || run.CompatibilityElapsed != 0) {
		return false
	}
	if plan.CompatibilityStage == AnalysisStagePerformance && (plan.CompatibilityBudget <= 0 || plan.CompatibilityBudget > maxPerformanceRunBudget || run.CompatibilityElapsed < 0 || run.CompatibilityElapsed > plan.CompatibilityBudget) {
		return false
	}
	for _, file := range plan.Files {
		for _, stage := range file.Stages {
			if stage.Stage != plan.CompatibilityStage && (stage.Eligible || stage.Cached || stage.ProviderID != "" || stage.MaxModelRequests != 0) {
				return false
			}
		}
	}
	return true
}

func legacyRunActive(status AnalysisRunStatus) bool {
	switch status {
	case AnalysisRunQueued, AnalysisRunRunning, AnalysisRunPausing, AnalysisRunPaused, AnalysisRunInterrupted, AnalysisRunCanceling:
		return true
	}
	return false
}

// Both adapters use the same admission lock, durable reservations and worker.
// The legacy boolean is translated only to the one provider scope it names.
func (s *Service) startCompatibilityRun(ctx context.Context, stage AnalysisStage, limits AnalysisRunLimits, budget time.Duration, options PerformanceJobOptions, confirmed bool) (*AnalysisRun, error) {
	s.jobLifecycleMu.Lock()
	defer s.jobLifecycleMu.Unlock()
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := s.restoreAnalysisRunLocked(); err != nil {
		return nil, err
	}
	analysis, err := s.manager.Analysis()
	if err != nil {
		return nil, err
	}
	if options.ProjectRevision != "" && options.ProjectRevision != analysis.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	if c.run != nil && legacyRunActive(c.run.Status) {
		if c.run.Plan.CompatibilityStage == stage && c.fault == nil && c.run.Status != AnalysisRunCanceling {
			if stage == AnalysisStagePerformance {
				current := performanceProjection(c.run, c.root)
				if options.QueueID != "" && options.QueueID != current.QueueID || options.PolicyFingerprint != "" && options.PolicyFingerprint != current.PolicyFingerprint {
					return nil, project.ErrRevisionConflict
				}
			}
			return cloneAnalysisRun(c.run), nil
		}
		return nil, errAnalysisRunBusy
	}
	preview, err := s.analysisPreviewLocked(ctx, AnalysisPreviewRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Scope: AnalysisRunScopeProject, Limits: limits, compatibilityStage: stage, compatibilityBudget: budget})
	if err != nil {
		return nil, err
	}
	if stage == AnalysisStagePerformance {
		queue := performanceProjection(newAnalysisRun(*preview), s.manager.Root())
		if (options.QueueID != "" && options.QueueID != queue.QueueID) || (options.PolicyFingerprint != "" && options.PolicyFingerprint != queue.PolicyFingerprint) {
			return nil, project.ErrRevisionConflict
		}
	}
	confirmations := compatibilityConfirmations(preview, confirmed)
	return s.startAnalysisRunLocked(ctx, AnalysisRunStartRequest{Identity: preview.Identity, PreviewID: preview.PreviewID, Limits: preview.Limits, Confirmations: confirmations}, stage, budget)
}

func compatibilityConfirmations(preview *AnalysisRunPreview, confirmed bool) AnalysisRunConfirmations {
	result := AnalysisRunConfirmations{ProviderIDs: []string{}}
	if confirmed {
		for _, provider := range preview.Providers {
			result.ProviderIDs = append(result.ProviderIDs, provider.ID)
		}
	}
	return result
}

func (s *Service) controlCompatibilityRun(ctx context.Context, stage AnalysisStage, action AnalysisRunAction, expectedID string, confirmed bool, revision ...string) (*AnalysisRun, error) {
	run, err := s.CurrentAnalysisRun(ctx)
	if err != nil && err != errAnalysisRunPersistence {
		return nil, err
	}
	if run == nil || run.Plan.CompatibilityStage != stage {
		return nil, ErrAnalysisLegacyMigration
	}
	if len(revision) > 0 && revision[0] != "" && revision[0] != run.Identity.ProjectRevision {
		return nil, project.ErrRevisionConflict
	}
	if expectedID != "" && run.Identity.ID != expectedID {
		return nil, project.ErrRevisionConflict
	}
	request := AnalysisRunControlRequest{Identity: run.Identity, Action: action}
	if action == AnalysisRunResume {
		preview, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{ProjectID: run.Identity.ProjectID, ProjectRevision: run.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: run.Plan.Refresh, Limits: run.Plan.Limits, ResumeRun: &run.Identity})
		if err != nil {
			return nil, err
		}
		confirmations := compatibilityConfirmations(preview, confirmed)
		request.PreviewID, request.Confirmations = preview.PreviewID, &confirmations
	}
	return s.ControlAnalysisRun(ctx, request)
}

func compatibilityStatus(status AnalysisRunStatus) string {
	switch status {
	case AnalysisRunQueued, AnalysisRunRunning:
		return "running"
	case AnalysisRunPausing, AnalysisRunPaused:
		return "paused"
	case AnalysisRunCanceling, AnalysisRunCanceled:
		return "canceled"
	case AnalysisRunInterrupted:
		return "interrupted"
	case AnalysisRunStale:
		return "stale"
	default:
		return "completed"
	}
}

func compatibilityFileStatus(stage AnalysisStageProgress) string {
	switch stage.Status {
	case AnalysisStageRunning:
		return "running"
	case AnalysisStageCompleted, AnalysisStageCompletedEmpty, AnalysisStagePartial:
		return "completed"
	case AnalysisStageFailed, AnalysisStageUnavailable:
		return "failed"
	default:
		return "pending"
	}
}

func analyzeAllProjection(run *AnalysisRun) *AnalyzeAllJob {
	if run == nil {
		return nil
	}
	job := &AnalyzeAllJob{ProjectID: run.Identity.ProjectID, ProjectRevision: run.Identity.ProjectRevision, Status: compatibilityStatus(run.Status), MaxFiles: run.Plan.Limits.BatchFiles, MaxRetries: run.Plan.Limits.MaxAttemptsPerStage - 1, Files: []AnalyzeAllFileJob{}, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt}
	for _, file := range run.Files {
		stage := file.Stages[0]
		item := AnalyzeAllFileJob{Path: file.Path, Status: compatibilityFileStatus(stage), Attempts: stage.Attempts}
		if item.Status == "failed" {
			item.Error = stage.Reason
		}
		job.Files = append(job.Files, item)
	}
	return job
}

func performanceProjection(run *AnalysisRun, root string) *PerformanceJob {
	if run == nil {
		return nil
	}
	job := &PerformanceJob{ID: run.Identity.ID, Generation: run.Identity.Generation, ProjectID: run.Identity.ProjectID, ProjectRevision: run.Identity.ProjectRevision, Root: root, PolicyFingerprint: run.Identity.PolicyFingerprint, Status: compatibilityStatus(run.Status), MaxFiles: run.Plan.Limits.BatchFiles, RunBudget: run.Plan.CompatibilityBudget, Elapsed: run.CompatibilityElapsed, Files: []PerformanceJobFile{}, CreatedAt: run.CreatedAt, UpdatedAt: run.UpdatedAt}
	for _, file := range run.Files {
		stage := file.Stages[1]
		item := PerformanceJobFile{Path: file.Path, ContentHash: file.ContentHash, Status: compatibilityFileStatus(stage), Attempts: stage.Attempts}
		if stage.Cached {
			item.Status = "cached"
		}
		if item.Status == "failed" {
			item.Error = stage.Reason
		}
		job.Files = append(job.Files, item)
	}
	job.QueueID = performanceQueueID(job.ProjectID, job.ProjectRevision, job.PolicyFingerprint, job.Files)
	return job
}
