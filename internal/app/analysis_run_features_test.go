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
	if _, err := s.SaveFeatureGoals(featureRequestFor(t, s, "Changed goals after preview.")); err != nil {
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
	if _, err := s.SaveFeatureGoals(featureRequestFor(t, s, "Keep cancellation responsive.")); err != nil {
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
	previous, err := s.GenerateFeatures(context.Background(), featureRequestFor(t, s, ""))
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
			server, calls, started, release := analysisBlockingServer(t)
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			preview := featureAnalysisPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
			run, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview))
			if err != nil {
				t.Fatal(err)
			}
			waitForTestSignal(t, started, "feature request")
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
			if settled.Status != want || settled.Features.SuggestionCount != nil || calls.Load() != 1 {
				t.Fatalf("late feature publication=%+v calls=%d", settled, calls.Load())
			}
			report, err := s.Features(context.Background())
			if err != nil || report.Status != "not_generated" {
				t.Fatalf("late ideas saved=%+v %v", report, err)
			}
		})
	}
}

func TestAnalysisRunFeaturesAndReviewedApplyCompleteWithoutLockConflict(t *testing.T) {
	server, _, started, releaseProvider := analysisBlockingServer(t)
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
