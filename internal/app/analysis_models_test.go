package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func analysisModelsPreviewFor(t *testing.T, s *Service, models *AnalysisModels, limits AnalysisRunLimits) *AnalysisRunPreview {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAnalysisRun(context.Background(), AnalysisPreviewRequest{Models: models, IncludeFeatures: true,
		ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Scope: AnalysisRunScopeProject, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func TestAnalysisModelsDispatchChosenProvidersAndRetainDefaults(t *testing.T) {
	reply := func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return validFeatureResponse
		}
		return analysisReply(stage)
	}
	analyze, analyzeCalls := analysisResponseServer(t, reply)
	bug, bugCalls := analysisResponseServer(t, reply)
	function, functionCalls := analysisResponseServer(t, reply)
	base, _ := newSemanticAnalysisService(t, analyze.URL, 0)
	cfg := scopedTestConfig(analyze.URL)
	cfg.ModelScopes.Analyze.Model = "review-model"
	cfg.ModelScopes.Bug.APIBaseURL, cfg.ModelScopes.Bug.Model = bug.URL, "code-model"
	cfg.ModelScopes.Function.APIBaseURL, cfg.ModelScopes.Function.Model = function.URL, "feature-model"
	s, err := New(cfg, base.manager)
	if err != nil {
		t.Fatal(err)
	}
	original := s.CurrentModelCatalog()
	models := &AnalysisModels{Code: "function", Performance: "bug", Security: "analyze", Features: "analyze"}
	preview := analysisModelsPreviewFor(t, s, models, AnalysisRunLimits{100, 30, 2})
	if analyzeCalls.Load()+bugCalls.Load()+functionCalls.Load() != 0 || len(preview.Providers) != 3 {
		t.Fatalf("model preview dispatched or lost a provider: %+v", preview)
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	models.Code = "bug" // Caller changes cannot retarget the admitted run.
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunCompleted || functionCalls.Load() != 1 || bugCalls.Load() != 1 || analyzeCalls.Load() != 2 {
		t.Fatalf("chosen providers were not used: status=%s code=%d review=%d features=%d", run.Status, functionCalls.Load(), bugCalls.Load(), analyzeCalls.Load())
	}
	if !reflect.DeepEqual(original, s.CurrentModelCatalog()) || run.Plan.Models.Code != "function" {
		t.Fatal("run choices mutated configured defaults or captured input")
	}
	selection, err := s.ReadAnalysisSelection(context.Background(), run.Identity.ProjectID, run.Identity.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range selection.Files {
		for _, stage := range file.Stages {
			if stage.Status != "fresh" {
				t.Fatalf("selected-model file evidence lost freshness: %s %+v", file.Path, stage)
			}
		}
	}
	for _, category := range []project.FindingCategory{project.FindingCategoryBugs, project.FindingCategoryPerformance, project.FindingCategorySecurity} {
		results, err := s.ReadAnalysisSection(context.Background(), run.Identity, category, "")
		if err != nil || results.SavedFindingCount == nil || *results.SavedFindingCount == 0 {
			t.Fatalf("chosen-model evidence missing for %s: %+v, %v", category, results, err)
		}
		if category == project.FindingCategoryPerformance && (results.Performance[0].Profile != "bug" || results.Performance[0].Scope != "analyze") {
			t.Fatalf("review provenance lost chosen profile: %+v", results.Performance[0])
		}
	}
	cached := analysisModelsPreviewFor(t, s, run.Plan.Models, preview.Limits)
	if cached.ExpectedModelRequests != 1 { // Only new feature discovery; all file stages reuse evidence.
		t.Fatalf("selected model caches were not reused: %+v", cached)
	}
	defaults := analysisModelsPreviewFor(t, s, nil, preview.Limits)
	if defaults.ExpectedModelRequests != 3 || defaults.Identity.ProviderFingerprint == preview.Identity.ProviderFingerprint {
		t.Fatalf("default profiles reused another model's evidence: %+v", defaults)
	}
}

func TestAnalysisModelsUseSelectedTimeouts(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	s.runtimes.analyze.effective.Timeout = "30m"
	s.runtimes.function.effective.Timeout = "20m"
	s.runtimes.bug.effective.Timeout = "1m"
	models := &AnalysisModels{Code: "analyze", Review: "function", Features: "bug"}
	prepared, err := s.prepareFileAnalysis("main.go")
	if err != nil {
		t.Fatal(err)
	}
	prepared.runtime = s.analysisModelRuntime(AnalysisStageSemantic, models)
	stopped := errors.New("deadline observed before dispatch")
	attempts := 0
	dispatchFor := func(want time.Duration) *analysisModelDispatch {
		return &analysisModelDispatch{maxAttempts: 1, beforeAttempt: func(ctx context.Context) error {
			attempts++
			deadline, ok := ctx.Deadline()
			remaining := time.Until(deadline)
			if !ok || remaining > want || remaining < want-time.Second {
				t.Fatalf("selected model deadline=%s, want %s", remaining, want)
			}
			return stopped
		}}
	}
	if _, err := s.requestFileAnalysis(context.Background(), prepared, dispatchFor(30*time.Minute)); !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	performance, err := s.preparePerformanceReview("main.go")
	if err != nil {
		t.Fatal(err)
	}
	performance.runtime = s.analysisModelRuntime(AnalysisStagePerformance, models)
	performance.modelProfile = models.Review
	if _, err := s.requestPerformanceReview(context.Background(), performance, dispatchFor(20*time.Minute)); !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	security, err := s.prepareSecurityReview(SecurityReviewRequest{ProjectID: prepared.analysis.ProjectID,
		ProjectRevision: prepared.analysis.ProjectRevision, Path: "main.go", BaseFileHash: prepared.input.ContentHash})
	if err != nil {
		t.Fatal(err)
	}
	security.runtime = s.analysisModelRuntime(AnalysisStageSecurityAI, models)
	security.modelProfile = models.Review
	if _, _, err := s.executeSecurityReview(context.Background(), security, dispatchFor(20*time.Minute)); !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	// Legacy standalone reviews retain the existing Analysis timeout.
	performance.runtime, performance.modelProfile = s.runtimes.analyze, ""
	if _, err := s.requestPerformanceReview(context.Background(), performance, dispatchFor(s.analysisTimeout)); !errors.Is(err, stopped) {
		t.Fatal(err)
	}
	if attempts != 4 || calls.Load() != 0 {
		t.Fatalf("deadline checks=%d provider calls=%d", attempts, calls.Load())
	}
}

func TestAnalysisModelsRejectInvalidRetargetingAndRequireActualProviderConsent(t *testing.T) {
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return `{"suggestions":[]}`
		}
		return emptyAnalysisReply(stage)
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := s.manager.Index()
	request := AnalysisPreviewRequest{Models: &AnalysisModels{Code: "unknown", Review: "analyze", Features: "analyze"},
		ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Scope: AnalysisRunScopeProject, Limits: AnalysisRunLimits{100, 30, 2}}
	if _, err := s.PreviewAnalysisRun(context.Background(), request); err == nil || calls.Load() != 0 {
		t.Fatalf("invalid profile was admitted: %v", err)
	}
	s.runtimes.function.effective.RemoteProvider = true
	models := &AnalysisModels{Code: "bug", Review: "analyze", Features: "function"}
	preview := analysisModelsPreviewFor(t, s, models, request.Limits)
	start := analysisStartFor(preview)
	start.Models.Features = "analyze"
	if _, err := s.StartAnalysisRun(context.Background(), start); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 0 {
		t.Fatalf("retargeted feature model was admitted: %v", err)
	}
	start = analysisStartFor(preview)
	start.Confirmations.ProviderIDs = nil
	if _, err := s.StartAnalysisRun(context.Background(), start); err == nil || calls.Load() != 0 {
		t.Fatalf("selected remote provider lacked consent: %v", err)
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	if run := completedAnalysisRun(t, s); run.Status != AnalysisRunCompletedEmpty || calls.Load() != 4 {
		t.Fatalf("confirmed selected model failed: %+v calls=%d", run, calls.Load())
	}
}

func TestAnalysisModelsPersistAcrossResumeAndRejectChangedProvider(t *testing.T) {
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return validFeatureResponse
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	models := &AnalysisModels{Code: "function", Review: "bug", Features: "function"}
	preview := analysisModelsPreviewFor(t, s, models, AnalysisRunLimits{1, 30, 2})
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	restored, err := loadAnalysisRun(root)
	if err != nil || !reflect.DeepEqual(restored.Plan.Models, models) {
		t.Fatalf("model selections were not persisted: %+v, %v", restored, err)
	}
	resume := featureAnalysisPreviewFor(t, s, preview.Limits, &paused.Identity)
	if !reflect.DeepEqual(resume.Models, models) || resume.ExpectedModelRequests != 3 {
		t.Fatalf("resume lost selected models: %+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	if run := completedAnalysisRun(t, s); run.Status != AnalysisRunCompleted || calls.Load() != 7 {
		t.Fatalf("resumed selected models did not complete: %+v calls=%d", run, calls.Load())
	}
	latest := analysisModelsPreviewFor(t, s, models, AnalysisRunLimits{100, 30, 2})
	s.runtimes.function = newModelRuntime(s.runtimes.function.profile, s.importTimeout, 4)
	s.runtimes.function.effective.ReasoningEffort = "changed"
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(latest)); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 7 {
		t.Fatalf("changed selected provider was admitted: %v", err)
	}
	if runtime := s.analysisModelRuntime(AnalysisStageSemantic, nil); runtime.profile.Scope != config.BugModelScope {
		t.Fatal("legacy default scope changed")
	}
}

func TestAnalysisIndependentModelsRejectInvalidSelections(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := s.manager.Index()
	for _, models := range []AnalysisModels{
		{Code: "bug", Performance: "function", Features: "analyze"},
		{Code: "bug", Performance: "invalid", Security: "analyze", Features: "analyze"},
		{Code: "bug", Performance: "function", Security: "configured:missing", Features: "analyze"},
	} {
		_, err := s.PreviewAnalysisRun(context.Background(), AnalysisPreviewRequest{
			Models: &models, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
			Scope: AnalysisRunScopeProject, Limits: AnalysisRunLimits{100, 30, 2},
		})
		if err == nil || calls.Load() != 0 {
			t.Fatalf("invalid selection admitted: %+v, %v", models, err)
		}
	}
}
