package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func assertSavedAnalysisCounts(t *testing.T, s *Service, run *AnalysisRun, want ...int) {
	t.Helper()
	for i, category := range []project.FindingCategory{project.FindingCategoryBugs, project.FindingCategoryPerformance, project.FindingCategorySecurity} {
		result, err := s.ReadAnalysisSection(context.Background(), run.Identity, category, "")
		if err != nil {
			t.Fatal(err)
		}
		if result.SavedFindingCount == nil || *result.SavedFindingCount != want[i] {
			t.Fatalf("%s saved count: %+v, want %d", category, result, want[i])
		}
	}
}

func TestAnalysisResultsKeepCanceledRefreshEvidenceAndUnknownCounts(t *testing.T) {
	var block atomic.Bool
	started, release := make(chan struct{}), make(chan struct{})
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if block.Load() {
			close(started)
			<-release
		}
		return analysisReply(stage)
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	ctx := context.Background()
	initial := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(initial)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	block.Store(true)
	refresh, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{ProjectID: run.Identity.ProjectID, ProjectRevision: run.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: true, Limits: initial.Limits})
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := s.StartAnalysisRun(ctx, analysisStartFor(refresh))
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "refresh request")
	_, err = s.ControlAnalysisRun(ctx, AnalysisRunControlRequest{Identity: admitted.Identity, Action: AnalysisRunCancel})
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	run = completedAnalysisRun(t, s)
	if run.Status != AnalysisRunCanceled {
		t.Fatalf("canceled run: %s", run.Status)
	}
	assertSavedAnalysisCounts(t, s, run, 1, 1, 2)

	missing, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := retryPreviewFor(t, missing, nil)
	if _, err := missing.StartAnalysisRun(ctx, analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	empty := completedAnalysisRun(t, missing)
	for _, category := range []project.FindingCategory{project.FindingCategoryBugs, project.FindingCategoryPerformance, project.FindingCategorySecurity} {
		result, err := missing.ReadAnalysisSection(ctx, empty.Identity, category, "")
		if err != nil || result.SavedFindingCount != nil {
			t.Fatalf("missing evidence became zero: %+v %v", result, err)
		}
	}
}

func TestAnalysisResultsRetainAllProducersThroughFailedRefreshAndRetry(t *testing.T) {
	var mode atomic.Int32
	started, release := make(chan struct{}), make(chan struct{})
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		switch mode.Load() {
		case 1:
			if stage == AnalysisStageSemantic {
				close(started)
				<-release
			}
			return "invalid response"
		case 2:
			return "invalid response"
		case 3:
			return emptyAnalysisReply(stage)
		default:
			return analysisReply(stage)
		}
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	ctx := context.Background()
	initial := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(initial)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	assertSavedAnalysisCounts(t, s, run, 1, 1, 2)
	mode.Store(1)
	refresh, err := s.PreviewAnalysisRun(ctx, AnalysisPreviewRequest{ProjectID: run.Identity.ProjectID, ProjectRevision: run.Identity.ProjectRevision, Scope: AnalysisRunScopeProject, Refresh: true, Limits: initial.Limits})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(refresh)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "refresh request")
	func() {
		defer close(release)
		running, err := s.CurrentAnalysisRun(ctx)
		if err != nil {
			t.Fatal(err)
		}
		assertSavedAnalysisCounts(t, s, running, 1, 1, 2)
	}()
	run = completedAnalysisRun(t, s)
	if run.Files[0].Stages[0].Status != AnalysisStageFailed || run.Sections[0].FindingCount != nil {
		t.Fatalf("old evidence became successful retry coverage: %+v", run)
	}
	assertSavedAnalysisCounts(t, s, run, 1, 1, 2)
	mode.Store(2)
	for attempt := 0; attempt < 2; attempt++ {
		preview := retryPreviewFor(t, s, nil)
		if _, err := s.StartAnalysisRun(ctx, analysisStartFor(preview)); err != nil {
			t.Fatal(err)
		}
		run = completedAnalysisRun(t, s)
		s.analysisRun = &analysisRunController{}
		before := calls.Load()
		assertSavedAnalysisCounts(t, s, run, 1, 1, 2)
		if calls.Load() != before {
			t.Fatal("restoring saved findings dispatched a model request")
		}
	}
	// Historical reports remain readable with their original revision after source changes.
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reindex(); err != nil {
		t.Fatal(err)
	}
	preview := retryPreviewFor(t, s, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run = completedAnalysisRun(t, s)
	assertSavedAnalysisCounts(t, s, run, 1, 1, 2)
	for _, category := range []project.FindingCategory{project.FindingCategoryPerformance, project.FindingCategorySecurity} {
		result, err := s.ReadAnalysisSection(ctx, run.Identity, category, "")
		if err != nil || category == project.FindingCategorySecurity && (len(result.Semantic) != 1 || result.Semantic[0].Freshness != project.FindingFreshnessStale) {
			t.Fatalf("historical evidence lost freshness: %+v %v", result, err)
		}
		for _, report := range result.Performance {
			if report.Status != "stale" {
				t.Fatal("historical performance evidence became current")
			}
		}
		for _, report := range result.Security {
			if report.Source == project.SecuritySourceAI && report.Status != project.SecurityStatusStale {
				t.Fatal("historical security evidence became current")
			}
		}
	}
	mode.Store(3)
	preview = retryPreviewFor(t, s, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	assertSavedAnalysisCounts(t, s, completedAnalysisRun(t, s), 0, 0, 0)
}

func TestAnalysisResultsKeepFilesOutsideRetryWithoutDoubleCountingOrIgnoringExclusions(t *testing.T) {
	var mode atomic.Int32
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if mode.Load() == 1 {
			return "invalid response"
		}
		if stage == AnalysisStageSemantic && mode.Load() == 0 {
			return strings.Replace(validSemanticAnalysis, `"risks":[]`, `"risks":[{"category":"bugs","severity":"low","summary":"Review correctness."},{"category":"performance","severity":"low","summary":"Review repeated work."},{"category":"security","severity":"low","summary":"Review input trust."}]`, 1)
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	ctx := context.Background()
	initial := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(initial)); err != nil {
		t.Fatal(err)
	}
	assertSavedAnalysisCounts(t, s, completedAnalysisRun(t, s), 2, 2, 2)
	cache, err := project.NewFileAnalysisCache(root)
	if err != nil {
		t.Fatal(err)
	}
	report, err := s.CachedFileAnalysis("main.go")
	if err != nil {
		t.Fatal(err)
	}
	report.PromptVersion = "historical-prompt"
	if err := cache.Store(*report); err != nil {
		t.Fatal(err)
	}
	mode.Store(1)
	retry := retryPreviewFor(t, s, nil)
	if len(retry.Files) != 1 || retry.Files[0].Path != "main.go" {
		t.Fatalf("retry scope: %+v", retry.Files)
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	assertSavedAnalysisCounts(t, s, run, 2, 2, 2)
	result, err := s.ReadAnalysisSection(ctx, run.Identity, project.FindingCategoryBugs, "")
	if err != nil || len(result.RetainedFiles) != 1 || result.RetainedFiles[0].Path != "helper.go" {
		t.Fatalf("retained inventory: %+v %v", result, err)
	}
	filtered, err := s.ReadAnalysisSection(ctx, run.Identity, project.FindingCategoryBugs, "main.go")
	if err != nil || filtered.SavedFindingCount == nil || *filtered.SavedFindingCount != 1 || len(filtered.RetainedFiles) != 0 {
		t.Fatalf("file filter widened: %+v %v", filtered, err)
	}
	mode.Store(2)
	retry = retryPreviewFor(t, s, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	assertSavedAnalysisCounts(t, s, completedAnalysisRun(t, s), 1, 1, 1)
	retry = retryPreviewFor(t, s, nil)
	if len(retry.Files) != 0 {
		t.Fatal("successful retries still need work")
	}
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	run = completedAnalysisRun(t, s)
	assertSavedAnalysisCounts(t, s, run, 1, 1, 1)
	if run.Status != AnalysisRunUnavailable {
		t.Fatal("retained findings changed empty retry coverage")
	}
	selection := readSelectionFor(t, s)
	if _, err := s.SaveAnalysisSelection(ctx, AnalysisSelectionRequest{selection.ProjectID, selection.ProjectRevision, selection.SelectionID, []string{"helper.go"}}); err != nil {
		t.Fatal(err)
	}
	retry = retryPreviewFor(t, s, nil)
	if _, err := s.StartAnalysisRun(ctx, analysisStartFor(retry)); err != nil {
		t.Fatal(err)
	}
	assertSavedAnalysisCounts(t, s, completedAnalysisRun(t, s), 0, 0, 0)
}
