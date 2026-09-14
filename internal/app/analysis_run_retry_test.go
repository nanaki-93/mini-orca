package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func seedRetrySemanticReport(t *testing.T, s *Service, path, status string) {
	t.Helper()
	analysis, err := s.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.manager.IndexedFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cache, input, err := s.fileAnalysisCacheInput(analysis, file, file.ContentHash)
	if err != nil {
		t.Fatal(err)
	}
	report, err := cache.Load(input)
	if err != nil {
		t.Fatal(err)
	}
	report.Status = status
	report.Purpose = "Describes the file."
	if status == "stale" {
		report.Status = "fresh"
		report.ContentHash = "sha256:previous"
	}
	if err := cache.Store(*report); err != nil {
		t.Fatal(err)
	}
}

func retryPreviewFor(t *testing.T, s *Service, resume *AnalysisRunIdentity) *AnalysisRunPreview {
	t.Helper()
	analysis, err := s.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAnalysisRun(context.Background(), AnalysisPreviewRequest{
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Scope: AnalysisRunScopeProject,
		Limits: AnalysisRunLimits{1, 30, 2}, RetryStaleFailed: true, ResumeRun: resume,
	})
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func TestAnalysisRetrySelectsStaleAndFailedFilesAndPreservesSelectionOnResume(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	for _, name := range []string{"fresh.go", "failed.go", "stale.go"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("package main\nfunc Run() {}\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Reindex(); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"fresh", "failed", "stale"} {
		seedRetrySemanticReport(t, s, status+".go", status)
	}
	preview := retryPreviewFor(t, s, nil)
	paths := []string{}
	for _, file := range preview.Files {
		paths = append(paths, file.Path)
	}
	if !reflect.DeepEqual(paths, []string{"failed.go", "stale.go"}) || calls.Load() != 0 || preview.ExpectedModelRequests != 6 {
		t.Fatalf("preview paths=%v requests=%d calls=%d", paths, preview.ExpectedModelRequests, calls.Load())
	}
	// Dropping the retry option cannot widen an already reviewed selection.
	start := analysisStartFor(preview)
	start.RetryStaleFailed = false
	if _, err := s.StartAnalysisRun(context.Background(), start); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 0 {
		t.Fatalf("broadened admission: %v", err)
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused || calls.Load() != 3 {
		t.Fatalf("paused=%s calls=%d", paused.Status, calls.Load())
	}
	// Restore durable progress after the first selected file has become fresh.
	s.analysisRun = &analysisRunController{}
	resume := retryPreviewFor(t, s, &paused.Identity)
	if resume.Identity != preview.Identity || len(resume.Files) != 2 || resume.ExpectedModelRequests != 3 {
		t.Fatalf("resume=%+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 6 {
		t.Fatalf("completed=%s calls=%d", completed.Status, calls.Load())
	}
	next := retryPreviewFor(t, s, nil)
	if len(next.Files) != 0 || next.ExpectedModelRequests != 0 || next.SecurityReviewIntentRequired || calls.Load() != 6 {
		t.Fatalf("empty retry=%+v calls=%d", next, calls.Load())
	}
	missing, err := s.CachedFileAnalysis("main.go")
	if err != nil || missing.Status != "missing" {
		t.Fatalf("unanalyzed file was included: %+v %v", missing, err)
	}
}

func TestAnalysisRetryRejectsEvidenceChangesAfterPreview(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	seedRetrySemanticReport(t, s, "main.go", "failed")
	preview := retryPreviewFor(t, s, nil)
	seedRetrySemanticReport(t, s, "main.go", "fresh")
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 0 {
		t.Fatalf("changed evidence admitted: %v calls=%d", err, calls.Load())
	}
}

func TestAnalysisRetryIncludesDurableFailuresAndReusesFreshStages(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if fail.Load() && stage == AnalysisStagePerformance {
			return "invalid model output"
		}
		return emptyAnalysisReply(stage)
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Files[0].Stages[1].Status != AnalysisStageFailed {
		t.Fatalf("failure fixture=%+v", completed)
	}
	fail.Store(false)
	retry := retryPreviewFor(t, s, nil)
	if len(retry.Files) != 1 || retry.ExpectedModelRequests != 1 {
		t.Fatalf("retry=%+v", retry)
	}
	before := calls.Load()
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	completed = completedAnalysisRun(t, s)
	if calls.Load() != before+1 || completed.Status != AnalysisRunCompletedEmpty {
		t.Fatalf("fresh stages re-requested: status=%s calls=%d", completed.Status, calls.Load()-before)
	}
}

func TestAnalysisRetryDoesNotOverrideNewerPartialEvidence(t *testing.T) {
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStagePerformance {
			return "invalid model output"
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Files[0].Stages[1].Status != AnalysisStageFailed {
		t.Fatalf("failure fixture=%+v", completed)
	}
	_, file, policy := storeCachedPerformanceReport(t, s, root)
	report, err := project.LoadPerformanceFileReport(root, file.Path, file.ContentHash, policy)
	if err != nil || report == nil {
		t.Fatalf("performance report=%+v %v", report, err)
	}
	report.Status = "partial"
	if err := project.StorePerformanceFileReport(root, *report); err != nil {
		t.Fatal(err)
	}
	if retry := retryPreviewFor(t, s, nil); len(retry.Files) != 0 {
		t.Fatalf("old failure overrode newer partial evidence: %+v", retry)
	}
	selection := readSelectionFor(t, s)
	if selection.Files[0].Stages[1].Status != "partial" {
		t.Fatalf("partial evidence mislabeled: %+v", selection)
	}
	report.Status = "completed"
	runtime := s.runtimes.analyze
	report.Model = runtime.profile.Model
	report.Profile, report.Scope = runtime.effective.Profile, runtime.effective.Scope
	report.ProviderOrigin, report.ReasoningEffort = runtime.effective.ProviderOrigin, runtime.effective.ReasoningEffort
	report.Findings = []project.PerformanceFinding{}
	if err := project.StorePerformanceFileReport(root, *report); err != nil {
		t.Fatal(err)
	}
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "fresh", "up to date")
	if retry := retryPreviewFor(t, s, nil); len(retry.Files) != 0 {
		t.Fatalf("newer success still requires retry: %+v", retry)
	}
}

func TestAnalysisRetryRefreshFailuresOverrideOlderCachesAcrossResume(t *testing.T) {
	var fail atomic.Bool
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if fail.Load() && (stage == AnalysisStagePerformance || stage == AnalysisStageSecurityAI) {
			return "invalid model output"
		}
		return emptyAnalysisReply(stage)
	})
	s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	ctx := context.Background()
	initial := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(initial)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	fail.Store(true)
	refresh, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{ProjectID: initial.Identity.ProjectID, ProjectRevision: initial.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: true, Limits: initial.Limits})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(refresh)); err != nil {
		t.Fatal(err)
	}
	failed := completedAnalysisRun(t, s)
	if failed.Status != AnalysisRunPartial || calls.Load() != 12 {
		t.Fatalf("refresh=%+v calls=%d", failed, calls.Load())
	}
	s.analysisRun = &analysisRunController{}
	selection := readSelectionFor(t, s)
	for _, file := range selection.Files {
		for _, stage := range file.Stages {
			if (stage.Stage == AnalysisStagePerformance || stage.Stage == AnalysisStageSecurityAI) && stage.Status != "failed" {
				t.Fatalf("older cache hid failed refresh: %+v", stage)
			}
		}
	}
	overview, err := s.ProjectOverview()
	if err != nil || overview.Coverage.Failed != 2 {
		t.Fatalf("summary hid failed refresh: %+v err=%v", overview, err)
	}
	fail.Store(false)
	retry := retryPreviewFor(t, s, nil)
	if len(retry.Files) != 2 || retry.ExpectedModelRequests != 4 {
		t.Fatalf("retry=%+v", retry)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused || calls.Load() != 14 {
		t.Fatalf("paused=%+v calls=%d", paused, calls.Load())
	}
	s.analysisRun = &analysisRunController{}
	resume := retryPreviewFor(t, s, &paused.Identity)
	if resume.ExpectedModelRequests != 2 {
		t.Fatalf("resume=%+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(ctx, AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 16 {
		t.Fatalf("completed=%+v calls=%d", completed, calls.Load())
	}
	for _, path := range []string{"main.go", "helper.go"} {
		assertSelectionStages(t, readSelectionFor(t, s), path, "fresh", "up to date")
	}
	if retry := retryPreviewFor(t, s, nil); len(retry.Files) != 0 {
		t.Fatalf("successful retries still need work: %+v", retry)
	}
}
