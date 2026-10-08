package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestAnalysisFreshnessSurvivesProjectLifecycle(t *testing.T) {
	for _, transition := range []string{"restore", "restart", "reindex", "switch back"} {
		t.Run(transition, func(t *testing.T) {
			server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
				if stage == AnalysisStageSemantic {
					return strings.Replace(validSemanticAnalysis, `"risks":[]`, `"risks":[{"category":"bugs","severity":"low","summary":"Review error handling."}]`, 1)
				}
				return emptyAnalysisReply(stage)
			})
			s, root := newAnalysisFreshnessService(t, server.URL, false)
			preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			completed := completedAnalysisRun(t, s)
			results, err := s.ReadAnalysisSection(context.Background(), completed.Identity, project.FindingCategoryBugs, "")
			if err != nil || len(results.Semantic) != 1 || results.Semantic[0].Freshness != project.FindingFreshnessFresh {
				t.Fatalf("initial results=%+v error=%v", results, err)
			}
			before, err := os.ReadFile(filepath.Join(root, analysisRunRelativePath))
			if err != nil {
				t.Fatal(err)
			}
			// Timestamps alone are not source changes.
			now := time.Now().Add(time.Hour)
			if err := os.Chtimes(filepath.Join(root, "main.go"), now, now); err != nil {
				t.Fatal(err)
			}
			s = transitionAnalysisProject(t, s, transition)
			overview, err := s.ProjectOverview()
			if err != nil {
				t.Fatal(err)
			}
			if overview.Run == nil {
				t.Fatal("restoration lost the saved analysis")
			}
			if !reflect.DeepEqual(overview.Run, completed) || overview.Coverage.Fresh != 1 || overview.Coverage.Stale != 0 || calls.Load() != 3 {
				t.Fatalf("restored status=%s coverage=%+v calls=%d", overview.Run.Status, overview.Coverage, calls.Load())
			}
			restoredResults, err := s.ReadAnalysisSection(context.Background(), completed.Identity, project.FindingCategoryBugs, "")
			if err != nil || !reflect.DeepEqual(restoredResults, results) {
				t.Fatalf("restored results=%+v error=%v", restoredResults, err)
			}
			after, err := os.ReadFile(filepath.Join(root, analysisRunRelativePath))
			if err != nil || string(after) != string(before) {
				t.Fatalf("unchanged analysis was rewritten: %v", err)
			}
		})
	}
}

func TestAnalysisCoverageWithResolvedModelSurvivesResumeAndRestart(t *testing.T) {
	server, calls := analysisModelResponseServer(t, "resolved-model-version", emptyAnalysisReply)
	s, _ := newAnalysisFreshnessService(t, server.URL, true)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{1, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused {
		t.Fatalf("first batch status=%s", paused.Status)
	}
	overview, err := s.ProjectOverview()
	if err != nil {
		t.Fatal(err)
	}
	if overview.Coverage != (AnalysisCoverage{Total: 2, Fresh: 1, Partial: 1}) {
		t.Errorf("first batch coverage=%+v", overview.Coverage)
	}
	resume := analysisRunPreviewFor(t, s, preview.Limits, &paused.Identity)
	if resume.ExpectedModelRequests != 3 {
		t.Errorf("resume requests=%d, want 3 for the remaining file", resume.ExpectedModelRequests)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 6 {
		t.Fatalf("resumed status=%s calls=%d", completed.Status, calls.Load())
	}
	for _, phase := range []string{"completed", "restarted"} {
		if phase == "restarted" {
			s = transitionAnalysisProject(t, s, "restart")
		}
		overview, err = s.ProjectOverview()
		if err != nil {
			t.Fatal(err)
		}
		if overview.Coverage != (AnalysisCoverage{Total: 2, Fresh: 2}) {
			t.Errorf("%s coverage=%+v", phase, overview.Coverage)
		}
	}
	selection := readSelectionFor(t, s)
	assertSelectionStages(t, selection, "helper.go", "fresh", "up to date")
	assertSelectionStages(t, selection, "main.go", "fresh", "up to date")
	cached := analysisRunPreviewFor(t, s, preview.Limits, nil)
	if cached.ExpectedModelRequests != 0 || calls.Load() != 6 {
		t.Fatalf("fresh evidence was not reused: requests=%d calls=%d", cached.ExpectedModelRequests, calls.Load())
	}
	results, err := s.ReadAnalysisSection(context.Background(), completed.Identity, project.FindingCategoryPerformance, "main.go")
	if err != nil || len(results.Performance) != 1 {
		t.Fatalf("performance results=%+v error=%v", results, err)
	}
	report := results.Performance[0]
	if report.Model != "resolved-model-version" || report.ConfiguredModel != s.runtimes.analyze.profile.Model || report.Status != "completed" {
		t.Fatalf("lost requested or returned model provenance: %+v", report)
	}
	// A real model-selection change still invalidates the reviews, even when
	// its new name happens to match the previous provider's reported model.
	s.runtimes.analyze.profile.Model = "resolved-model-version"
	s.runtimes.analyze.effective.Model = "resolved-model-version"
	overview, err = s.ProjectOverview()
	if err != nil {
		t.Fatal(err)
	}
	if overview.Coverage != (AnalysisCoverage{Total: 2, Stale: 2}) || calls.Load() != 6 {
		t.Fatalf("changed configuration coverage=%+v calls=%d", overview.Coverage, calls.Load())
	}
}

func TestAnalysisCoverageReadsLegacyPerformanceModelIdentity(t *testing.T) {
	for _, model := range []string{"", "resolved-model-version"} {
		t.Run("returned model="+model, func(t *testing.T) {
			server, calls := analysisModelResponseServer(t, model, emptyAnalysisReply)
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			run := completedAnalysisRun(t, s)
			results, err := s.ReadAnalysisSection(context.Background(), run.Identity, project.FindingCategoryPerformance, "main.go")
			if err != nil || len(results.Performance) != 1 {
				t.Fatalf("performance results=%+v error=%v", results, err)
			}
			// Schema 1 reports predating configured_model remain readable.
			legacy := results.Performance[0]
			legacy.ConfiguredModel = ""
			if err := project.StorePerformanceFileReport(root, legacy); err != nil {
				t.Fatal(err)
			}
			want := AnalysisCoverage{Total: 1, Fresh: 1}
			requests := 0
			if model != "" {
				want = AnalysisCoverage{Total: 1, Stale: 1}
				requests = 1 // The original selection cannot be recovered from an alias alone.
			}
			overview, err := s.ProjectOverview()
			if err != nil || overview.Coverage != want {
				t.Fatalf("legacy coverage=%+v error=%v", overview, err)
			}
			preview = analysisRunPreviewFor(t, s, preview.Limits, nil)
			if preview.ExpectedModelRequests != requests || calls.Load() != 3 {
				t.Fatalf("legacy requests=%d calls=%d", preview.ExpectedModelRequests, calls.Load())
			}
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			completedAnalysisRun(t, s)
			overview, err = s.ProjectOverview()
			if err != nil || overview.Coverage != (AnalysisCoverage{Total: 1, Fresh: 1}) || calls.Load() != int32(3+requests) {
				t.Fatalf("legacy recovery=%+v calls=%d error=%v", overview, calls.Load(), err)
			}
		})
	}
}

func TestAnalysisFreshnessDetectsChangesAcrossProjectLifecycle(t *testing.T) {
	for _, transition := range []string{"restore", "restart", "reindex", "switch back"} {
		for _, change := range []string{"edit", "add", "delete"} {
			t.Run(transition+"/"+change, func(t *testing.T) {
				server, calls := analysisResponseServer(t, emptyAnalysisReply)
				s, root := newAnalysisFreshnessService(t, server.URL, true)
				preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
				if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
					t.Fatal(err)
				}
				completed := completedAnalysisRun(t, s)
				var err error
				switch change {
				case "edit":
					err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600)
				case "add":
					err = os.WriteFile(filepath.Join(root, "added.go"), []byte("package main\n"), 0600)
				case "delete":
					err = os.Remove(filepath.Join(root, "main.go"))
				}
				if err != nil {
					t.Fatal(err)
				}
				s = transitionAnalysisProject(t, s, transition)
				overview, err := s.ProjectOverview()
				if err != nil {
					t.Fatal(err)
				}
				if overview.Run == nil || overview.Run.Status != AnalysisRunStale || overview.Run.Identity != completed.Identity || calls.Load() != 6 {
					t.Fatalf("changed run=%+v calls=%d", overview.Run, calls.Load())
				}
				// File-local semantic descriptions remain reusable, but full-stage
				// coverage includes reports tied to the changed project revision.
				helper, err := s.CachedFileAnalysis("helper.go")
				if err != nil || helper.Status != project.AnalysisStatusFresh {
					t.Fatalf("unchanged semantic description invalidated: %+v %v", helper, err)
				}
				want := AnalysisCoverage{Total: 2, Stale: 2}
				if change == "add" {
					want.Total, want.Missing = 3, 1
				}
				if change == "delete" {
					want.Total, want.Stale = 1, 1
				}
				if overview.Coverage != want {
					t.Fatalf("full-stage coverage=%+v, want %+v", overview.Coverage, want)
				}
			})
		}
	}
}

func transitionAnalysisProject(t *testing.T, s *Service, transition string) *Service {
	t.Helper()
	root := s.manager.Root()
	switch transition {
	case "restart":
		manager, err := project.NewManager(root)
		if err != nil {
			t.Fatal(err)
		}
		restarted, err := New(scopedTestConfig(s.runtimes.bug.profile.APIBaseURL), manager)
		if err != nil {
			t.Fatal(err)
		}
		restarted.runtimes = s.runtimes
		s = restarted
	case "switch back":
		other := t.TempDir()
		if err := s.ActivateProject(other, &project.Analysis{Name: "other"}); err != nil {
			t.Fatal(err)
		}
		if run, err := s.CurrentAnalysisRun(context.Background()); err != nil || run != nil {
			t.Fatalf("other project run=%+v error=%v", run, err)
		}
	case "reindex":
		if _, err := s.Reindex(); err != nil {
			t.Fatal(err)
		}
		return s
	}
	analysis, err := s.RestoreProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RestoreActiveProject(root, analysis); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAnalysisFreshnessPreservesRunningWorkOnUnchangedReindex(t *testing.T) {
	server, calls, started, release := analysisBlockingServer(t)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "analysis before unchanged reindex")
	if _, err := s.Reindex(); err != nil {
		t.Fatal(err)
	}
	release()
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 3 {
		t.Fatalf("reindex interrupted unchanged work: status=%s calls=%d", completed.Status, calls.Load())
	}
}

func TestAnalysisFreshnessRestoreInterruptsWorkWithoutMakingItStale(t *testing.T) {
	server, calls, started, release := analysisBlockingServer(t)
	s, root := newAnalysisFreshnessService(t, server.URL, false)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "analysis before restore")
	s = transitionAnalysisProject(t, s, "restore")
	release()
	interrupted := completedAnalysisRun(t, s)
	if interrupted.Status != AnalysisRunInterrupted || interrupted.Files[0].Stages[0].Status != AnalysisStageInterrupted || calls.Load() != 1 {
		t.Fatalf("restored status=%s stage=%s calls=%d", interrupted.Status, interrupted.Files[0].Stages[0].Status, calls.Load())
	}
	entries, err := os.ReadDir(filepath.Join(root, ".mini-orca/file-analysis"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatal("interrupted response published evidence")
	}
	resume := analysisRunPreviewFor(t, s, preview.Limits, &interrupted.Identity)
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: interrupted.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, s)
	if completed.Status != AnalysisRunCompletedEmpty || calls.Load() != 4 {
		t.Fatalf("resumed status=%s calls=%d", completed.Status, calls.Load())
	}
}

func TestAnalysisFreshnessRecoversPreviouslyStaledFinishedResults(t *testing.T) {
	for _, change := range []string{"unchanged", "source", "policy", "provider", "unfinished", "canceled"} {
		t.Run(change, func(t *testing.T) {
			server, calls := analysisResponseServer(t, emptyAnalysisReply)
			s, root := newAnalysisFreshnessService(t, server.URL, false)
			preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			completed := completedAnalysisRun(t, s)
			old := cloneAnalysisRun(completed)
			old.Status = AnalysisRunStale
			old.Reason = "Project source, policy or provider identity changed; start a new analysis."
			if change == "unfinished" || change == "canceled" {
				stage := &old.Files[0].Stages[0]
				stage.Status = AnalysisStageStale
				if change == "canceled" {
					stage.Status = AnalysisStageCanceled
				}
				stage.FindingCount, stage.ReportID, stage.Cached = nil, "", false
			}
			refreshAnalysisSections(old)
			data, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, analysisRunRelativePath), data, 0600); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "source":
				err = os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600)
			case "policy":
				err = os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("ignored.go\n"), 0600)
			case "provider":
				s.runtimes.analyze.effective.Model = "replacement"
			}
			if err != nil {
				t.Fatal(err)
			}
			s = transitionAnalysisProject(t, s, "restart")
			run, err := s.CurrentAnalysisRun(context.Background())
			if err != nil || run == nil {
				t.Fatalf("restore error=%v", err)
			}
			if change == "unchanged" {
				if !reflect.DeepEqual(run, completed) {
					t.Fatalf("finished analysis did not recover: status=%s", run.Status)
				}
			} else if run.Status != AnalysisRunStale {
				t.Fatalf("recovered invalid or unfinished analysis: status=%s", run.Status)
			}
			if calls.Load() != 3 {
				t.Fatalf("restoration contacted provider: %d calls", calls.Load())
			}
		})
	}
}

func newAnalysisFreshnessService(t *testing.T, baseURL string, includeHelper bool) (*Service, string) {
	t.Helper()
	s, root := newSemanticAnalysisServiceFixture(t, baseURL, 0, includeHelper)
	// Persist the imported summary so restoration exercises the same two service
	// entry points as POST /api/projects/restore, including a fresh disk scan.
	runtime := s.runtimes.analyze
	if _, err := project.NewAnalyzerWithProvenance(nil, runtime.profile.Model, runtime.effective.Profile, runtime.effective.ProviderOrigin, runtime.effective.ReasoningEffort).Analyze(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	return s, root
}
