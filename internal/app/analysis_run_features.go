package app

import (
	"context"
	"errors"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

// Goals and suggestion prose remain in the feature store; the run captures identities.
type AnalysisFeaturePlan struct {
	ExpectedHash     string   `json:"expected_hash"`
	GoalsHash        string   `json:"goals_hash"`
	WorkspaceHash    string   `json:"workspace_hash"`
	ExcludedPaths    []string `json:"excluded_paths"`
	ProviderID       string   `json:"provider_id"`
	MaxModelRequests int      `json:"max_model_requests"`
	Reason           string   `json:"reason,omitempty"`
}

type AnalysisFeatureProgress struct {
	Status          AnalysisStageStatus `json:"status"`
	Attempts        int                 `json:"attempts"`
	SuggestionCount *int                `json:"suggestion_count"`
	ReportHash      string              `json:"report_hash,omitempty"`
	Reason          string              `json:"reason,omitempty"`
}

func validateStoredAnalysisFeatures(run *AnalysisRun) error {
	plan, progress := run.Plan.Features, run.Features
	if plan == nil && progress == nil {
		return nil
	}
	if plan == nil || progress == nil || !validAnalysisFeaturePlan(plan, run.Plan.CompatibilityStage) {
		return errAnalysisRunCorrupt
	}
	if validateAnalysisExcludedPaths(plan.ExcludedPaths) != nil {
		return errAnalysisRunCorrupt
	}
	stage := AnalysisStageProgress{Stage: AnalysisStageFeatures, Status: progress.Status, Attempts: progress.Attempts,
		FindingCount: progress.SuggestionCount, ReportID: strings.TrimPrefix(progress.ReportHash, "sha256:"), Reason: progress.Reason}
	stagePlan := AnalysisStagePlan{Stage: AnalysisStageFeatures, Eligible: true, ProviderID: plan.ProviderID, MaxModelRequests: plan.MaxModelRequests, Reason: plan.Reason}
	count, err := validateStoredAnalysisStage(stage, stagePlan, AnalysisStageFeatures, run.Plan.Limits.MaxAttemptsPerStage)
	if err != nil || count > 5 || progress.Status == AnalysisStagePartial || progress.Status == AnalysisStageSkipped || progress.ReportHash != "" && !analysisFeatureHash(progress.ReportHash) {
		return errAnalysisRunCorrupt
	}
	return nil
}

func analysisFeatureHash(hash string) bool {
	return strings.HasPrefix(hash, "sha256:") && len(hash) == 71
}

func validAnalysisFeaturePlan(plan *AnalysisFeaturePlan, compatibility AnalysisStage) bool {
	return compatibility == "" && analysisFeatureHash(plan.GoalsHash) && analysisFeatureHash(plan.WorkspaceHash) && len(plan.ProviderID) == 64 &&
		(plan.ExpectedHash == "empty" || analysisFeatureHash(plan.ExpectedHash)) && len(plan.ExcludedPaths) <= maxAnalysisInventoryFiles
}

func (s *Service) planAnalysisFeatures(ctx context.Context, root string, preview *AnalysisRunPreview) error {
	report, err := s.Features(ctx)
	if err != nil {
		return err
	}
	fingerprint, err := benchmarkSourceFingerprint(ctx, root)
	if err != nil {
		return err
	}
	excluded, err := loadAnalysisSelection(root)
	if err != nil {
		return err
	}
	plan := &AnalysisFeaturePlan{ExpectedHash: report.Hash, GoalsHash: contentHash([]byte(report.Goals)), WorkspaceHash: fingerprint, ExcludedPaths: excluded,
		MaxModelRequests: min(preview.Limits.MaxAttemptsPerStage, s.runtimes.analyze.effective.MaxRetries+1)}
	for _, provider := range preview.Providers {
		for _, stage := range provider.Stages {
			if stage == AnalysisStageFeatures {
				plan.ProviderID = provider.ID
			}
		}
	}
	if s.runtimes.analyze.client == nil {
		plan.MaxModelRequests = 0
		plan.Reason = "The model for this stage is not configured."
	}
	preview.Features = plan
	return nil
}

func (s *Service) validateAnalysisFeatureInputs(ctx context.Context, root string, plan *AnalysisFeaturePlan, sources bool) error {
	if plan == nil {
		return nil
	}
	index, err := s.manager.Index()
	if err != nil {
		return err
	}
	report, err := readFeatures(root, index)
	if err != nil {
		return err
	}
	if contentHash([]byte(report.Goals)) != plan.GoalsHash {
		return project.ErrRevisionConflict
	}
	if sources {
		return (benchmarkFixture{fingerprint: plan.WorkspaceHash}).verifyCurrent(ctx, root)
	}
	return nil
}

func (s *Service) runAnalysisFeatureWindow(ctx context.Context, identity AnalysisRunIdentity, done chan struct{}) {
	defer close(done)
	c := s.analysisRun
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.run == nil || c.run.Identity != identity || c.run.Status != AnalysisRunRunning || c.fault != nil {
		return
	}
	s.runAnalysisFeaturesLocked(ctx, identity)
}

// The caller owns c.mu; provider I/O runs outside it and publication reacquires authority.
func (s *Service) runAnalysisFeaturesLocked(ctx context.Context, identity AnalysisRunIdentity) bool {
	c := s.analysisRun
	progress, plan := c.run.Features, c.admission.Features
	if progress.Attempts >= c.run.Plan.Limits.MaxAttemptsPerStage {
		progress.Status, progress.Reason = AnalysisStageFailed, "The stage exhausted its total attempt allowance."
		return s.saveAnalysisRunLocked(false) == nil
	}
	progress.Status, progress.Reason = AnalysisStageRunning, ""
	if err := s.saveAnalysisRunLocked(false); err != nil {
		return false
	}
	root := c.root
	index, err := s.manager.Index()
	var report *FeatureReport
	if err == nil {
		report, err = readFeatures(root, index)
	}
	if err != nil {
		return s.recordAnalysisFeaturesLocked(ctx, identity, nil, err)
	}
	request := FeatureRequest{ProjectID: identity.ProjectID, ProjectRevision: identity.ProjectRevision, ExpectedHash: plan.ExpectedHash, Goals: report.Goals,
		ConfirmRemoteProvider: analysisProviderConfirmed(c.confirmations, plan.ProviderID)}
	runtime := s.runtimes.analyze
	runtime.effective.MaxRetries = min(plan.MaxModelRequests, c.run.Plan.Limits.MaxAttemptsPerStage-progress.Attempts) - 1
	authority := s.analysisFeatureAuthority(identity)
	c.mu.Unlock()
	result, err := s.generateFeatures(ctx, request, runtime, plan.ExcludedPaths, authority)
	c.mu.Lock()
	return s.recordAnalysisFeaturesLocked(ctx, identity, result, err)
}

func (s *Service) checkAnalysisFeatureAuthorityLocked(ctx context.Context, identity AnalysisRunIdentity) error {
	c := s.analysisRun
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.run == nil || c.run.Identity != identity || c.run.Features == nil || c.run.Features.Status != AnalysisStageRunning {
		return project.ErrRevisionConflict
	}
	if c.fault != nil {
		return errAnalysisRunPersistence
	}
	if c.run.Status != AnalysisRunRunning && c.run.Status != AnalysisRunPausing {
		return project.ErrRevisionConflict
	}
	return s.validateAnalysisQueue(ctx, c.root, &c.run.Plan, false)
}

func (s *Service) analysisFeatureAuthority(identity AnalysisRunIdentity) featureGenerationAuthority {
	c := s.analysisRun
	return featureGenerationAuthority{
		BeforeAttempt: func(ctx context.Context) error {
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := s.checkAnalysisFeatureAuthorityLocked(ctx, identity); err != nil {
				return err
			}
			if c.run.Features.Attempts >= c.run.Plan.Limits.MaxAttemptsPerStage {
				return errAnalysisAttemptBudget
			}
			c.run.Features.Attempts++
			return s.saveAnalysisRunLocked(false)
		},
		Publish: func(write func() error) error {
			// Apply also reaches the run controller while holding changesMu.
			// Keep that lock order so concurrent source writes can invalidate this result.
			s.changesMu.Lock()
			defer s.changesMu.Unlock()
			c.mu.Lock()
			defer c.mu.Unlock()
			if err := s.checkAnalysisFeatureAuthorityLocked(context.Background(), identity); err != nil {
				return err
			}
			if err := write(); err != nil {
				if errors.Is(err, project.ErrRevisionConflict) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return err
				}
				s.failAnalysisPersistenceLocked()
				return errAnalysisRunPersistence
			}
			return nil
		},
	}
}

func (s *Service) recordAnalysisFeaturesLocked(ctx context.Context, identity AnalysisRunIdentity, report *FeatureReport, err error) bool {
	c := s.analysisRun
	if c.run == nil || c.run.Identity != identity || c.fault != nil {
		return false
	}
	if c.run.Status != AnalysisRunRunning && c.run.Status != AnalysisRunPausing {
		return false
	}
	progress := c.run.Features
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, project.ErrRevisionConflict) {
		progress.Status = AnalysisStageInterrupted
		if errors.Is(err, project.ErrRevisionConflict) {
			s.staleAnalysisRunLocked()
		} else {
			c.featureStopReason = "Feature discovery reached its allowance; preview and resume to try again."
		}
		_ = s.saveAnalysisRunLocked(false)
		return false
	}
	if err != nil {
		progress.Status, progress.Reason = AnalysisStageFailed, "The model request or response failed. Other analysis results remain available."
	} else {
		count := len(report.Suggestions)
		progress.Status, progress.SuggestionCount, progress.ReportHash = AnalysisStageCompleted, &count, report.Hash
		if count == 0 {
			progress.Status = AnalysisStageCompletedEmpty
		}
	}
	return s.saveAnalysisRunLocked(false) == nil
}
