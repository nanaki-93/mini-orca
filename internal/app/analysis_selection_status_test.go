package app

import (
	"context"
	"errors"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSelectionFor(t *testing.T, s *Service) *AnalysisFileSelection {
	t.Helper()
	analysis, err := s.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.ReadAnalysisSelection(context.Background(), analysis.ProjectID, analysis.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertSelectionStages(t *testing.T, selection *AnalysisFileSelection, path, status, reason string) {
	t.Helper()
	for _, file := range selection.Files {
		if file.Path != path {
			continue
		}
		if len(file.Stages) != 4 {
			t.Fatalf("stage coverage missing: %+v", file)
		}
		for _, stage := range file.Stages {
			if stage.Status != status || !strings.Contains(stage.Reason, reason) {
				t.Fatalf("%s: %+v expected %s / %s", path, stage, status, reason)
			}
		}
		return
	}
	t.Fatalf("file not in checklist: %s", path)
}

func TestAnalysisSelectionReportsMissingFreshAndUnindexedChanges(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "missing", "No saved analysis")
	if calls.Load() != 0 {
		t.Fatal("status read dispatched")
	}
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 900, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "fresh", "up to date")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "stale", "changed since indexing")
	if calls.Load() != 3 {
		t.Fatalf("unexpected calls %d", calls.Load())
	}
}

func TestAnalysisSelectionExplainsPausedCanceledAndIgnoredFiles(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{1, 900, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	assertSelectionStages(t, readSelectionFor(t, s), "helper.go", "fresh", "up to date")
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "paused", "Resume")
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunCancel}); err != nil {
		t.Fatal(err)
	}
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "canceled", "canceled")
	selection := readSelectionFor(t, s)
	if _, err := s.SaveAnalysisSelection(context.Background(), AnalysisSelectionRequest{selection.ProjectID, selection.ProjectRevision, selection.SelectionID, []string{"main.go"}}); err != nil {
		t.Fatal(err)
	}
	next := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 900, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(next)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	assertSelectionStages(t, readSelectionFor(t, s), "main.go", "missing", "Excluded from the previous run")
	if calls.Load() != 3 {
		t.Fatalf("status reads dispatched: %d", calls.Load())
	}
}

func TestAnalysisSelectionExplainsStageFailuresAndUnavailableProviders(t *testing.T) {
	server, _ := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	s.runtimes.bug.client = nil
	selection := readSelectionFor(t, s)
	for _, file := range selection.Files {
		if file.Path == "main.go" {
			if file.Stages[0].Status != "unavailable" || !strings.Contains(file.Stages[0].Reason, "not configured") {
				t.Fatalf("no provider explanation: %+v", file.Stages[0])
			}
		}
	}
	for _, test := range []struct {
		stage AnalysisStageStatus
		run   AnalysisRunStatus
		want  string
	}{
		{AnalysisStageFailed, AnalysisRunFailed, "failed"},
		{AnalysisStagePartial, AnalysisRunPartial, "incomplete"},
		{AnalysisStageInterrupted, AnalysisRunInterrupted, "interrupted"},
		{AnalysisStageCanceled, AnalysisRunCanceled, "canceled"},
		{AnalysisStageUnavailable, AnalysisRunUnavailable, "unavailable"},
		{AnalysisStageRunning, AnalysisRunRunning, "processing"},
	} {
		status, reason := selectionRunStage(AnalysisStageProgress{Status: test.stage}, &AnalysisRun{Status: test.run})
		if status != string(test.stage) || !strings.Contains(reason, test.want) {
			t.Fatalf("%s: %s %s", test.stage, status, reason)
		}
	}
	status, reason := selectionRunStage(AnalysisStageProgress{Status: AnalysisStageFailed, Reason: "The stage needs an additional attempt allowance."}, &AnalysisRun{Status: AnalysisRunFailed})
	if status != "failed" || reason != "The stage needs an additional attempt allowance." {
		t.Fatal("lost saved failure reason")
	}
}

func TestAnalysisSelectionExplainsNewFilesAndUnreadableStageEvidence(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "new.go"), []byte("package main"), 0600); err != nil {
		t.Fatal(err)
	}
	selection := readSelectionFor(t, s)
	found := false
	for _, file := range selection.Files {
		if file.Path == "new.go" {
			found = true
			if !strings.Contains(file.Reason, "Not indexed yet") {
				t.Fatalf("new file: %+v", file)
			}
		}
	}
	if !found {
		t.Fatal("new file omitted")
	}
	s.loadSecurityFileReport = func(string, project.SecurityReportInput) (*project.SecurityFileReport, error) {
		return nil, errors.New("unreadable evidence")
	}
	selection = readSelectionFor(t, s)
	for _, file := range selection.Files {
		if file.Path == "main.go" {
			for _, stage := range file.Stages[2:] {
				if stage.Status != "unavailable" || !strings.Contains(stage.Reason, "could not be read") {
					t.Fatalf("read error hidden: %+v", stage)
				}
			}
		}
	}
	if calls.Load() != 0 {
		t.Fatal("inspection dispatched analysis")
	}
}

func TestAnalysisSelectionLimitedRunSkipsDoNotClaimCompleteCoverage(t *testing.T) {
	server, _ := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	analysis, index, policy, err := s.performanceInputs()
	if err != nil {
		t.Fatal(err)
	}
	file := index.Files[0]
	evidence := analysisSelectionEvidence{files: map[string]AnalysisRunFile{file.Path: {AnalysisFileIdentity: AnalysisFileIdentity{Path: file.Path, ContentHash: file.ContentHash}, Stages: []AnalysisStageProgress{{Stage: AnalysisStagePerformance, Status: AnalysisStageSkipped, Reason: "Not requested by this compatibility action."}}}}}
	stage := s.analysisSelectionStage(*analysis, file, policy, AnalysisStagePerformance, evidence)
	if stage.Status != "missing" || !strings.Contains(stage.Reason, "Not requested") {
		t.Fatalf("omitted stage hidden: %+v", stage)
	}
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 900, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	stage = s.analysisSelectionStage(*analysis, file, policy, AnalysisStagePerformance, evidence)
	if stage.Status != "fresh" {
		t.Fatalf("current cache hidden by old skipped stage: %+v", stage)
	}
}
