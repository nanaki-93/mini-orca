package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

func featureAnalysisPreviewFor(t *testing.T, s *Service, limits AnalysisRunLimits, resume *AnalysisRunIdentity) *AnalysisRunPreview {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewAnalysisRun(context.Background(), AnalysisPreviewRequest{IncludeFeatures: true, ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
		Scope: AnalysisRunScopeProject, Limits: limits, ResumeRun: resume})
	if err != nil {
		t.Fatal(err)
	}
	return preview
}

func TestAnalysisRunFeaturesPreviewAndAdmissionCaptureGoals(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if calls.Load() != 0 || preview.Features == nil || preview.ExpectedModelRequests != 4 || preview.MaxModelRequests != 8 {
		t.Fatalf("feature preview=%+v calls=%d", preview, calls.Load())
	}
	start := analysisStartFor(preview)
	start.IncludeFeatures = false
	if _, err := s.StartAnalysisRun(context.Background(), start); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 0 {
		t.Fatalf("changed feature scope=%v calls=%d", err, calls.Load())
	}
	if _, err := s.SaveFeatureGoals(context.Background(), featureRequestFor(t, s, "Changed goals after preview.")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); !errors.Is(err, project.ErrRevisionConflict) || calls.Load() != 0 {
		t.Fatalf("changed goals=%v calls=%d", err, calls.Load())
	}
	remote, _ := newSemanticAnalysisService(t, "https://provider.invalid", 0)
	excludeFeatureAnalysisFiles(t, remote, []string{"main.go"})
	remotePreview := featureAnalysisPreviewFor(t, remote, AnalysisRunLimits{100, 30, 2}, nil)
	if len(remotePreview.Files) != 0 || remotePreview.ExpectedModelRequests != 1 || remotePreview.SecurityReviewIntentRequired {
		t.Fatalf("features-only preview=%+v", remotePreview)
	}
	remoteStart := analysisStartFor(remotePreview)
	remoteStart.Confirmations.ProviderIDs = nil
	if _, err := remote.StartAnalysisRun(context.Background(), remoteStart); err == nil || !strings.Contains(err.Error(), "confirmation") {
		t.Fatalf("unconfirmed provider=%v", err)
	}
}

func excludeFeatureAnalysisFiles(t *testing.T, s *Service, paths []string) {
	t.Helper()
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	selection, err := s.ReadAnalysisSelection(context.Background(), index.ProjectID, index.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveAnalysisSelection(context.Background(), AnalysisSelectionRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision,
		SelectionID: selection.SelectionID, ExcludedPaths: paths}); err != nil {
		t.Fatal(err)
	}
}

func analysisBlockingFeatureServer(t *testing.T) (*httptest.Server, *atomic.Int32, chan struct{}, func()) {
	t.Helper()
	started, released := make(chan struct{}, 1), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(released) }) }
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			started <- struct{}{}
			<-released
			return validFeatureResponse
		}
		return emptyAnalysisReply(stage)
	})
	t.Cleanup(unblock)
	return server, calls, started, unblock
}

func waitForAnalysisFileCount(t *testing.T, s *Service, count int) *AnalysisRun {
	t.Helper()
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		run, err := s.CurrentAnalysisRun(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if run.WindowFilesCompleted == count {
			return run
		}
		select {
		case <-deadline:
			t.Fatalf("file results did not reach %d: %+v", count, run)
		case <-ticker.C:
		}
	}
}

func TestAnalysisRunPublishesBatchResultsWhileFeatureRequestRuns(t *testing.T) {
	for _, test := range []struct {
		name   string
		batch  int
		cached bool
	}{
		{name: "whole project", batch: 100},
		{name: "batch limit", batch: 1},
		{name: "cached results", batch: 100, cached: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, _, started, release := analysisBlockingFeatureServer(t)
			s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
			limits := AnalysisRunLimits{test.batch, 30, 2}
			if test.cached {
				preview := analysisRunPreviewFor(t, s, limits, nil)
				if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
					t.Fatal(err)
				}
				completedAnalysisRun(t, s)
			}
			preview := featureAnalysisPreviewFor(t, s, limits, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			waitForTestSignal(t, started, "independent feature request")
			completed := min(test.batch, len(preview.Files))
			run := waitForAnalysisFileCount(t, s, completed)
			if run.Status != AnalysisRunRunning || run.Features.Status != AnalysisStageRunning || run.WindowFilesCompleted != completed {
				t.Fatalf("file results delayed by feature request: files=%d status=%s features=%+v", run.WindowFilesCompleted, run.Status, run.Features)
			}
			for i, section := range run.Sections {
				result, err := s.ReadAnalysisSection(context.Background(), run.Identity, section.Category, "")
				if err != nil {
					t.Fatal(err)
				}
				if result.SavedFindingCount == nil || result.Progress.Coverage.Succeeded != completed*(i+1) {
					t.Fatalf("%s results unavailable during feature request: %+v", section.Category, result)
				}
			}
			release()
			settled := completedAnalysisRun(t, s)
			want := AnalysisRunCompleted
			if test.batch < len(preview.Files) {
				want = AnalysisRunPaused
			}
			if settled.Status != want || settled.Features.Status != AnalysisStageCompleted {
				t.Fatalf("file and feature work did not settle: %+v", settled)
			}
		})
	}
}

func TestAnalysisRunGeneratesFeaturesWithoutFileStages(t *testing.T) {
	server, calls := analysisResponseServer(t, func(AnalysisStage) string { return `{"suggestions":[]}` })
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	excludeFeatureAnalysisFiles(t, s, []string{"main.go"})
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{1, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunCompletedEmpty || run.Features.Status != AnalysisStageCompletedEmpty || len(run.Files) != 0 || calls.Load() != 1 {
		t.Fatalf("features without file stages: status=%s features=%+v files=%d calls=%d", run.Status, run.Features, len(run.Files), calls.Load())
	}
}

func TestAnalysisRunFeaturesHonorContextAndSuggestedPathExclusions(t *testing.T) {
	for _, target := range []string{"main.go", "helper.go"} {
		t.Run(target, func(t *testing.T) {
			var leaked atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request llm.ChatRequest
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				reply := `{"findings":[]}`
				if request.ResponseFormat.JSONSchema.Name == "feature_suggestions" {
					leaked.Store(strings.Contains(request.Messages[1].Content, "helper.go") || strings.Contains(request.Messages[1].Content, "helper secret"))
					reply = strings.Replace(validFeatureResponse, "main.go", target, 1)
				} else if strings.HasPrefix(request.Messages[0].Content, "You summarize") {
					reply = validSemanticAnalysis
				}
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: reply}}}})
			}))
			t.Cleanup(server.Close)
			s, _ := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
			excludeFeatureAnalysisFiles(t, s, []string{"helper.go"})
			preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
				t.Fatal(err)
			}
			run := completedAnalysisRun(t, s)
			want := AnalysisStageCompleted
			if target == "helper.go" {
				want = AnalysisStageFailed
			}
			if leaked.Load() || run.Features.Status != want {
				t.Fatalf("excluded feature context=%t progress=%+v", leaked.Load(), run.Features)
			}
		})
	}
}

func TestAnalysisRunFeatureRetriesRespectInclusiveAttemptLimit(t *testing.T) {
	for _, limit := range []int{1, 2} {
		var featureCalls atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var request llm.ChatRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if request.ResponseFormat.JSONSchema.Name == "feature_suggestions" {
				featureCalls.Add(1)
				http.Error(w, "temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			reply := `{"findings":[]}`
			if strings.HasPrefix(request.Messages[0].Content, "You summarize") {
				reply = validSemanticAnalysis
			}
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: reply}}}})
		}))
		t.Cleanup(server.Close)
		s, _ := newSemanticAnalysisService(t, server.URL, 0)
		preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, limit}, nil)
		if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
			t.Fatal(err)
		}
		run := completedAnalysisRun(t, s)
		if run.Features.Attempts != limit || int(featureCalls.Load()) != limit || run.Status != AnalysisRunPartial {
			t.Fatalf("limit=%d progress=%+v requests=%d", limit, run.Features, featureCalls.Load())
		}
	}
}

func TestAnalysisRunFeaturesPersistOnceAcrossBatches(t *testing.T) {
	var featureCalls atomic.Int32
	server, calls := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			featureCalls.Add(1)
			return validFeatureResponse
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	if _, err := s.SaveFeatureGoals(context.Background(), featureRequestFor(t, s, "Keep cancellation responsive.")); err != nil {
		t.Fatal(err)
	}
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{1, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	paused := completedAnalysisRun(t, s)
	if paused.Status != AnalysisRunPaused || paused.Features.Status != AnalysisStageCompleted || *paused.Features.SuggestionCount != 1 || featureCalls.Load() != 1 {
		t.Fatalf("paused feature progress=%+v calls=%d", paused, featureCalls.Load())
	}
	resume := featureAnalysisPreviewFor(t, s, preview.Limits, &paused.Identity)
	if resume.Features.MaxModelRequests != 0 || resume.ExpectedModelRequests != 3 {
		t.Fatalf("completed features redispatched=%+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: paused.Identity, Action: AnalysisRunResume,
		PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunCompleted || featureCalls.Load() != 1 || calls.Load() != 7 {
		t.Fatalf("completed=%+v calls=%d", run, calls.Load())
	}
	for _, section := range run.Sections {
		if section.FindingCount == nil || *section.FindingCount != 0 {
			t.Fatalf("advisory idea counted as a finding=%+v", section)
		}
	}
	report, err := s.Features(context.Background())
	if err != nil || len(report.Suggestions) != 1 || report.Hash != run.Features.ReportHash {
		t.Fatalf("feature store=%+v %v", report, err)
	}
	restored, err := loadAnalysisRun(root)
	if err != nil || restored.Features.ReportHash != report.Hash {
		t.Fatalf("restored=%+v %v", restored, err)
	}
	data, err := os.ReadFile(filepath.Join(root, analysisRunRelativePath))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{report.Goals, report.Suggestions[0].Benefit, "package main", "provider_ids", "confirmations"} {
		if strings.Contains(string(data), private) {
			t.Fatalf("run metadata contains %q", private)
		}
	}
}

func TestAnalysisRunFeatureFailureRetainsIdeasAndOtherAnalysis(t *testing.T) {
	var invalid atomic.Bool
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			if invalid.Load() {
				return `{}`
			}
			return validFeatureResponse
		}
		return emptyAnalysisReply(stage)
	})
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	previous, err := s.GenerateFeatures(context.Background(), FeatureGenerateRequest{FeatureRequest: featureRequestFor(t, s, "")})
	if err != nil {
		t.Fatal(err)
	}
	invalid.Store(true)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunPartial || run.Features.Status != AnalysisStageFailed || run.Features.SuggestionCount != nil {
		t.Fatalf("failed feature stage=%+v", run)
	}
	for _, section := range run.Sections {
		if section.Status != AnalysisRunCompletedEmpty {
			t.Fatalf("failed feature step hid completed findings=%+v", section)
		}
	}
	retained, err := s.Features(context.Background())
	if err != nil || retained.Status != "failed" || retained.Suggestions[0].ID != previous.Suggestions[0].ID {
		t.Fatalf("previous ideas lost=%+v %v", retained, err)
	}
}

func TestAnalysisRunFeaturesCancelAndSourceChangesRejectLatePublication(t *testing.T) {
	for _, change := range []string{"cancel", "instructions"} {
		t.Run(change, func(t *testing.T) {
			server, calls, started, release := analysisBlockingFeatureServer(t)
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			run, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview))
			if err != nil {
				t.Fatal(err)
			}
			waitForTestSignal(t, started, "feature request")
			waitForAnalysisFileCount(t, s, len(preview.Files))
			want := AnalysisRunCanceled
			if change == "cancel" {
				if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: run.Identity, Action: AnalysisRunCancel}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("Changed project instructions."), 0644); err != nil {
					t.Fatal(err)
				}
				want = AnalysisRunStale
			}
			release()
			settled := completedAnalysisRun(t, s)
			if settled.Status != want || settled.Features.SuggestionCount != nil || calls.Load() != 4 {
				t.Fatalf("late feature publication=%+v calls=%d", settled, calls.Load())
			}
			if change == "cancel" {
				assertSavedAnalysisCounts(t, s, settled, 0, 0, 0)
			}
			report, err := s.Features(context.Background())
			if err != nil || report.Status != "not_generated" {
				t.Fatalf("late ideas saved=%+v %v", report, err)
			}
		})
	}
}

func TestAnalysisRunFeatureDiscoveryOutlivesFileBudget(t *testing.T) {
	featureStarted, fileCanceled := make(chan struct{}, 1), make(chan struct{}, 1)
	releaseFeature := make(chan struct{})
	var once sync.Once
	release := func() { once.Do(func() { close(releaseFeature) }) }
	defer release()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.ResponseFormat.JSONSchema.Name != "feature_suggestions" {
			<-r.Context().Done()
			fileCanceled <- struct{}{}
			return
		}
		featureStarted <- struct{}{}
		<-releaseFeature
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validFeatureResponse}}}})
	}))
	t.Cleanup(server.Close)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 1, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, featureStarted, "feature discovery during file work")
	waitForTestSignal(t, fileCanceled, "file budget expiry")
	run, err := s.CurrentAnalysisRun(context.Background())
	if err != nil || run.Status != AnalysisRunRunning || run.Features.Status != AnalysisStageRunning {
		t.Fatalf("file deadline interrupted features: %+v, %v", run, err)
	}
	release()
	run = completedAnalysisRun(t, s)
	if run.Status != AnalysisRunPaused || run.Features.Status != AnalysisStageCompleted || run.Files[0].Stages[0].Status != AnalysisStageInterrupted {
		t.Fatalf("independent workers settled incorrectly: %+v", run)
	}
	if restored, err := loadAnalysisRun(root); err != nil || restored.Features.ReportHash != run.Features.ReportHash {
		t.Fatalf("independent progress was not durable: %+v, %v", restored, err)
	}
}

func TestAnalysisRunPauseStopsFeatureDiscoveryWithoutWaitingForResponse(t *testing.T) {
	server, _, started, release := analysisBlockingFeatureServer(t)
	defer release()
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	run, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview))
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "feature request")
	waitForAnalysisFileCount(t, s, len(preview.Files))
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: run.Identity, Action: AnalysisRunPause}); err != nil {
		t.Fatal(err)
	}
	settled := completedAnalysisRun(t, s)
	if settled.Status != AnalysisRunPaused || settled.Features.Status != AnalysisStageInterrupted || settled.Features.SuggestionCount != nil {
		t.Fatalf("paused feature request retained authority: %+v", settled)
	}
	release()
	if report, err := s.Features(context.Background()); err != nil || report.Status != "not_generated" {
		t.Fatalf("paused request published late features: %+v, %v", report, err)
	}
}

func TestAnalysisRunFeaturesAndReviewedApplyCompleteWithoutLockConflict(t *testing.T) {
	server, _, started, releaseProvider := analysisBlockingFeatureServer(t)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := s.ProposeInstructions(context.Background(), InstructionProposalRequest{ChangeCreateRequest: ChangeCreateRequest{
		ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "instructions", Title: "Reviewed project guidance", Paths: []string{"AGENTS.md"}},
		Content: "Preserve cancellation behavior.\n"})
	if err != nil {
		t.Fatal(err)
	}
	proposal = approveChangeFixture(t, s, proposal)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "feature request before Apply")
	writeStarted, continueWrite, applied := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseWrite := func() { once.Do(func() { close(continueWrite) }) }
	defer releaseWrite()
	var receipt *ChangeMutationResult
	var applyErr error
	go func() {
		receipt, applyErr = s.applyChangeWithWriter(context.Background(), proposal.ID, ChangeApplyRequest{ChangeIdentity: changeIdentity(proposal), Confirm: true}, func(path string, data []byte) error {
			close(writeStarted)
			<-continueWrite
			return atomicWrite(path, data)
		})
		close(applied)
	}()
	waitForTestSignal(t, writeStarted, "reviewed Apply write")
	releaseProvider()
	// Let the completed provider response reach publication while Apply owns its write scope.
	select {
	case <-applied:
		t.Fatal("Apply completed before its controlled source write")
	case <-time.After(50 * time.Millisecond):
	}
	releaseWrite()
	waitForTestSignal(t, applied, "concurrent Apply completion")
	if applyErr != nil || receipt.State != "applied" {
		t.Fatalf("concurrent Apply=%+v %v", receipt, applyErr)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunStale || run.Features.SuggestionCount != nil {
		t.Fatalf("changed instructions reused feature output=%+v", run)
	}
	report, err := s.Features(context.Background())
	if err != nil || report.Status != "not_generated" {
		t.Fatalf("late feature report=%+v %v", report, err)
	}
	if content, err := os.ReadFile(filepath.Join(root, "AGENTS.md")); err != nil || string(content) != "Preserve cancellation behavior.\n" {
		t.Fatalf("reviewed instructions=%s %v", content, err)
	}
}

func TestAnalysisRunFeaturesKeepPriorIdeasAndCountOnlyNewOnes(t *testing.T) {
	var featureReply atomic.Value
	featureReply.Store(featureOutput("Idea one", "Idea two", "Idea three", "Idea four", "Idea five"))
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return featureReply.Load().(string)
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	library, err := generateFeaturesFor(t, s, "Improve.")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.UpdateFeatureStatus(context.Background(), library.Suggestions[0].ID, FeatureStatusRequest{FeatureRequest: featureRequestFor(t, s, "Improve."), Status: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	featureReply.Store(featureOutput("idea ONE", "Idea six", "Idea seven"))
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Features.Status != AnalysisStageCompleted || run.Features.SuggestionCount == nil || *run.Features.SuggestionCount != 2 {
		t.Fatalf("run feature count=%+v", run.Features)
	}
	report, err := s.Features(context.Background())
	if err != nil || len(report.Suggestions) != 7 || report.Hash != run.Features.ReportHash || report.LastGeneration.AddedCount != 2 || report.LastGeneration.DuplicateCount != 1 {
		t.Fatalf("run replaced the library=%+v %v", report, err)
	}
	if featureIdeasJSON(t, &FeatureReport{Suggestions: report.Suggestions[:5]}) != featureIdeasJSON(t, saved) {
		t.Fatalf("run changed existing ideas: %+v", report.Suggestions[:5])
	}
	// The cumulative library exceeds five ideas; the stored run still validates and restores.
	if restored, err := loadAnalysisRun(root); err != nil || *restored.Features.SuggestionCount != 2 {
		t.Fatalf("restored run=%+v %v", restored, err)
	}
	restarted, err := New(scopedTestConfig(server.URL), s.manager)
	if err != nil {
		t.Fatal(err)
	}
	restarted.runtimes = s.runtimes
	if current, err := restarted.CurrentAnalysisRun(context.Background()); err != nil || current.Status != run.Status || *current.Features.SuggestionCount != 2 {
		t.Fatalf("restarted run=%+v %v", current, err)
	}
	featureReply.Store(featureOutput("Idea six"))
	again := featureAnalysisPreviewFor(t, restarted, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := restarted.StartAnalysisRun(context.Background(), analysisStartFor(again)); err != nil {
		t.Fatal(err)
	}
	repeated := completedAnalysisRun(t, restarted)
	if repeated.Features.Status != AnalysisStageCompletedEmpty || *repeated.Features.SuggestionCount != 0 {
		t.Fatalf("duplicate-only run=%+v", repeated.Features)
	}
	if kept, err := restarted.Features(context.Background()); err != nil || len(kept.Suggestions) != 7 {
		t.Fatalf("duplicate-only run changed the library=%+v %v", kept, err)
	}
}

func TestAnalysisRunFeatureHistoryFullFailsOnlyTheFeatureStep(t *testing.T) {
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return featureOutput("A genuinely new capability")
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	fillFeatureHistory(t, s, root, 16)
	before := featureHistoryBytes(t, root)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	if run.Status != AnalysisRunPartial || run.Features.Status != AnalysisStageFailed || run.Features.Reason != analysisFeatureHistoryFullReason || run.Features.SuggestionCount != nil {
		t.Fatalf("history-full run=%+v features=%+v", run.Status, run.Features)
	}
	s.analysisRun.mu.Lock()
	fault := s.analysisRun.fault
	s.analysisRun.mu.Unlock()
	if fault != nil {
		t.Fatal("history-full result became a run persistence fault")
	}
	if string(featureHistoryBytes(t, root)) != string(before) {
		t.Fatal("history-full run changed the saved report")
	}
	if restored, err := loadAnalysisRun(root); err != nil || restored.Features.Reason != analysisFeatureHistoryFullReason {
		t.Fatalf("restored history-full run=%+v %v", restored, err)
	}
}

func TestAnalysisRunWithEarlierFeaturePromptIdentityReadsStale(t *testing.T) {
	server, _ := analysisResponseServer(t, func(stage AnalysisStage) string {
		if stage == AnalysisStageFeatures {
			return validFeatureResponse
		}
		return emptyAnalysisReply(stage)
	})
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	completedAnalysisRun(t, s)
	stored, err := loadAnalysisRun(root)
	if err != nil {
		t.Fatal(err)
	}
	// A run admitted under an earlier feature prompt has a provider fingerprint
	// that the current prompt version no longer produces.
	previous, earlier := stored.Plan.Features.ProviderID, strings.Repeat("0", 64)
	for i := range stored.Plan.Providers {
		if stored.Plan.Providers[i].ID == previous {
			stored.Plan.Providers[i].ID = earlier
		}
	}
	for i := range stored.Plan.Files {
		for j := range stored.Plan.Files[i].Stages {
			if stored.Plan.Files[i].Stages[j].ProviderID == previous {
				stored.Plan.Files[i].Stages[j].ProviderID = earlier
			}
		}
	}
	stored.Plan.Features.ProviderID = earlier
	providers, err := analysisFingerprint(stored.Plan.Providers)
	if err != nil {
		t.Fatal(err)
	}
	stored.Identity.ProviderFingerprint, stored.Plan.Identity.ProviderFingerprint = providers, providers
	queue, err := analysisQueueFingerprint(&stored.Plan)
	if err != nil {
		t.Fatal(err)
	}
	stored.Identity.QueueID, stored.Plan.Identity.QueueID = queue, queue
	data, err := json.Marshal(stored)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(filepath.Join(root, analysisRunRelativePath), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAnalysisRun(root); err != nil {
		t.Fatalf("earlier prompt identity read as corrupt: %v", err)
	}
	restarted, err := New(scopedTestConfig(server.URL), s.manager)
	if err != nil {
		t.Fatal(err)
	}
	restarted.runtimes = s.runtimes
	current, err := restarted.CurrentAnalysisRun(context.Background())
	if err != nil || current.Status != AnalysisRunStale || current.Features.Status != AnalysisStageCompleted || *current.Features.SuggestionCount != 1 {
		t.Fatalf("earlier prompt run=%+v %v", current, err)
	}
}
