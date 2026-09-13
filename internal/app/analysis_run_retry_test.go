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
}
