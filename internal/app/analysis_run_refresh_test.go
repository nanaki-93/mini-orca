package app

import (
	"context"
	"errors"
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
