package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestAnalysisRefreshReanalyzesIncludedFilesAcrossWindows(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	ctx := context.Background()
	initial := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 900, 2}, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(initial)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	if calls.Load() != 6 {
		t.Fatalf("initial requests=%d", calls.Load())
	}

	selectFiles := func(excluded []string) {
		t.Helper()
		selection := readSelectionFor(t, s)
		if _, err := s.SaveAnalysisSelection(ctx, AnalysisSelectionRequest{selection.ProjectID, selection.ProjectRevision, selection.SelectionID, excluded}); err != nil {
			t.Fatal(err)
		}
	}
	previewRefresh := func(resume *AnalysisRunIdentity) *AnalysisRunPreview {
		t.Helper()
		analysis, _ := s.manager.Analysis()
		preview, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{
			ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
			Scope: AnalysisRunScopeProject, Refresh: true, Limits: AnalysisRunLimits{1, 900, 2}, ResumeRun: resume,
		})
		if err != nil {
			t.Fatal(err)
		}
		return preview
	}
	selectFiles([]string{"helper.go"})
	single := previewRefresh(nil)
	if len(single.Files) != 1 || single.Files[0].Path != "main.go" || single.ExpectedModelRequests != 3 || calls.Load() != 6 {
		t.Fatalf("refresh ignored selection or reused model results: %+v, calls=%d", single, calls.Load())
	}
	for _, stage := range single.Files[0].Stages {
		if stage.Cached != (stage.Stage == AnalysisStageSecurityRules) {
			t.Fatalf("refresh cache plan: %+v", stage)
		}
	}
	changed := analysisStartFor(single)
	changed.Refresh = false
	if _, err := s.StartAnalysisRun(ctx, changed); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 6 {
		t.Fatalf("refresh consent changed: %v, calls=%d", err, calls.Load())
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(single)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	if calls.Load() != 9 {
		t.Fatalf("included file was not refreshed: requests=%d", calls.Load())
	}

	selectFiles([]string{})
	all := previewRefresh(nil)
	if len(all.Files) != 2 || all.ExpectedModelRequests != 6 || calls.Load() != 9 {
		t.Fatalf("whole refresh scope: %+v", all)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(all)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused || calls.Load() != 12 {
		t.Fatalf("window=%s requests=%d", paused.Status, calls.Load())
	}
	resume := previewRefresh(&paused.Identity)
	if resume.ExpectedModelRequests != 3 {
		t.Fatalf("resume did not retain completed work: %+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(ctx, AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 15 {
		t.Fatalf("whole refresh=%s requests=%d", completed.Status, calls.Load())
	}
	for _, file := range completed.Files {
		for _, stage := range file.Stages {
			if stage.Stage != AnalysisStageSecurityRules && (stage.Cached || stage.Attempts != 1) {
				t.Fatalf("model stage was reused or repeated on resume: %s %+v", file.Path, stage)
			}
		}
	}
}

func TestAnalysisClearResultsReplacesPausedRunAndRestartsFromZero(t *testing.T) {
	var emptyReply atomic.Bool
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if emptyReply.Load() {
			if stage == AnalysisStageFeatures {
				return `{"suggestions":[]}`
			}
			return emptyAnalysisReply(stage)
		}
		if stage == AnalysisStageFeatures {
			return `{"suggestions":[{"title":"Add batch export","benefit":"Helps users export reports.","evidence":"main.go defines Run.","paths":["main.go"],"effort":"small","acceptance_criteria":["Exports JSON report."]}]}`
		}
		return analysisReply(stage)
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "z.go"), []byte("package main\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reindex(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	analysis, err := s.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	initialFeatures, err := s.Features(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveFeatureGoals(ctx, FeatureRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, ExpectedHash: initialFeatures.Hash, Goals: "Keep the CLI simple."}); err != nil {
		t.Fatal(err)
	}
	firstPreview, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		Scope: AnalysisRunScopeProject, IncludeFeatures: true, Limits: AnalysisRunLimits{1, 30, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(firstPreview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused || paused.WindowFilesCompleted != 1 {
		t.Fatalf("expected paused run after 1 file, got %+v", paused)
	}
	assertSavedAnalysisCounts(t, s, paused, 1, 1, 2)
	featuresAfterFirst, err := s.Features(ctx)
	if err != nil || len(featuresAfterFirst.Suggestions) != 1 || featuresAfterFirst.Goals != "Keep the CLI simple." {
		t.Fatalf("expected 1 feature suggestion and preserved goals, got %+v %v", featuresAfterFirst, err)
	}

	clearPreview, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		Scope: AnalysisRunScopeProject, Refresh: true, ClearResults: true, IncludeFeatures: true,
		Limits: AnalysisRunLimits{1, 30, 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range clearPreview.Files {
		for _, stage := range file.Stages {
			if stage.Cached {
				t.Fatalf("clear_results preview left stage cached: %s %+v", file.Path, stage)
			}
		}
	}
	// Preview is strictly read-only: old findings and feature suggestions remain until StartAnalysisRun.
	assertSavedAnalysisCounts(t, s, paused, 1, 1, 2)

	emptyReply.Store(true)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(clearPreview)); err != nil {
		t.Fatalf("StartAnalysisRun with ClearResults failed to replace paused run: %v", err)
	}
	restartedPaused := completedAnalysisRun(t, s)
	if restartedPaused.Status != AnalysisRunPaused || restartedPaused.Identity.ID == paused.Identity.ID {
		t.Fatalf("restarted run did not replace paused run: %+v", restartedPaused)
	}
	assertSavedAnalysisCounts(t, s, restartedPaused, 0, 0, 0)
	featuresAfterClear, err := s.Features(ctx)
	if err != nil || len(featuresAfterClear.Suggestions) != 0 || featuresAfterClear.Goals != "Keep the CLI simple." {
		t.Fatalf("expected 0 feature suggestions and preserved goals after clear, got %+v %v", featuresAfterClear, err)
	}

	resumePreview, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		Scope: AnalysisRunScopeProject, Refresh: true, Limits: AnalysisRunLimits{1, 30, 1},
		ResumeRun: &restartedPaused.Identity,
	})
	if err != nil {
		t.Fatal(err)
	}
	confirmations := analysisStartFor(resumePreview).Confirmations
	if _, err := s.ControlAnalysisRun(ctx, AnalysisRunControlRequest{
		Identity: restartedPaused.Identity, Action: AnalysisRunResume,
		PreviewID: resumePreview.PreviewID, Confirmations: &confirmations,
	}); err != nil {
		t.Fatal(err)
	}
	finished := completedAnalysisRun(t, s)
	if finished.Status != AnalysisRunCompletedEmpty {
		t.Fatalf("expected completed_empty run after resume, got %+v", finished)
	}
	assertSavedAnalysisCounts(t, s, finished, 0, 0, 0)
}
