package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/storage"
)

func TestAnalysisRunStoreRestoresInterruptedProgressWithoutConsentOrRequests(t *testing.T) {
	server, calls, started, release := analysisBlockingServer(t)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	s.runtimes.bug.effective.RemoteProvider = true
	s.runtimes.analyze.effective.RemoteProvider = true
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	startedRun, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview))
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "request before restart fixture")
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: startedRun.Identity, Action: AnalysisRunPause}); err != nil {
		t.Fatal(err)
	}
	release()
	paused := completedAnalysisRun(t, s)
	// Simulate a process loss after reserving the next stage, before its result.
	paused.Status = AnalysisRunRunning
	paused.Files[0].Stages[1].Status = AnalysisStageRunning
	paused.Files[0].Stages[1].Attempts = 1
	refreshAnalysisSections(paused)
	data, err := json.Marshal(paused)
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.WriteFile(filepath.Join(root, analysisRunRelativePath), data, 0600); err != nil {
		t.Fatal(err)
	}
	restored, err := New(scopedTestConfig(server.URL), s.manager)
	if err != nil {
		t.Fatal(err)
	}
	restored.runtimes = s.runtimes
	overview, err := restored.ProjectOverview()
	if err != nil {
		t.Fatal(err)
	}
	run := overview.Run
	if run == nil || run.Status != AnalysisRunInterrupted || run.Files[0].Stages[1].Attempts != 1 || run.Files[0].Stages[1].Status != AnalysisStageInterrupted || run.Files[0].Stages[0].FindingCount == nil || calls.Load() != 1 {
		t.Fatalf("restore=%+v calls=%d", run, calls.Load())
	}
	if restored.analysisRun.done != nil || restored.analysisRun.confirmations.SecurityReview || len(restored.analysisRun.confirmations.ProviderIDs) != 0 {
		t.Fatal("restore retained dispatch authority")
	}
	resume := analysisRunPreviewFor(t, restored, preview.Limits, &run.Identity)
	if resume.ExpectedModelRequests != 2 || resume.MaxModelRequests != 3 {
		t.Fatalf("resume estimate=%+v", resume)
	}
	for _, confirmations := range []AnalysisRunConfirmations{{}, {SecurityReview: true}, {ProviderIDs: analysisStartFor(resume).Confirmations.ProviderIDs}} {
		if _, err := restored.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: run.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err == nil || calls.Load() != 1 {
			t.Fatal("resume accepted missing fresh consent")
		}
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := restored.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: run.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, restored)
	if completed.Status != AnalysisRunCompletedEmpty || completed.Files[0].Stages[1].Attempts != 2 || calls.Load() != 3 {
		t.Fatalf("restored completion=%+v calls=%d", completed, calls.Load())
	}
	info, err := os.Stat(filepath.Join(root, analysisRunRelativePath))
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("progress permissions=%v %v", info, err)
	}
}

func TestAnalysisRunStoreRejectsCorruptionAndKeepsTheOriginalBytes(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	valid := newAnalysisRun(*preview)
	for _, test := range []struct {
		name   string
		mutate func(*AnalysisRun)
	}{
		{"queue", func(run *AnalysisRun) { run.Identity.QueueID = "changed" }},
		{"attempts", func(run *AnalysisRun) { run.Files[0].Stages[0].Attempts = 3 }},
		{"stage", func(run *AnalysisRun) { run.Files[0].Stages[0].Stage = AnalysisStageSecurityAI }},
		{"status", func(run *AnalysisRun) { run.Files[0].Stages[0].Status = "invented" }},
		{"coverage", func(run *AnalysisRun) { run.Sections[0].Coverage.Succeeded++ }},
		{"source reason", func(run *AnalysisRun) { run.Files[0].Stages[0].Reason = "package main; secret input" }},
		{"traversal", func(run *AnalysisRun) { run.Files[0].Path = "../outside.go" }},
		{"empty claims", func(run *AnalysisRun) { run.Status = AnalysisRunCompletedEmpty }},
		{"unknown schema", func(run *AnalysisRun) { run.SchemaVersion = "future" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			run := cloneAnalysisRun(valid)
			test.mutate(run)
			data, err := json.Marshal(run)
			if err != nil {
				t.Fatal(err)
			}
			name := filepath.Join(root, analysisRunRelativePath)
			if err := storage.WriteFile(name, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadAnalysisRun(root); !errors.Is(err, errAnalysisRunCorrupt) {
				t.Fatalf("corruption=%v", err)
			}
			after, err := os.ReadFile(name)
			if err != nil || string(after) != string(data) {
				t.Fatal("corrupt progress was discarded or rewritten")
			}
		})
	}
	data, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"schema_version":`, string(data) + "\n{}", strings.Replace(string(data), `"schema_version":"1"`, `"confirmations":{"security_review":true},"schema_version":"1"`, 1)} {
		if err := storage.WriteFile(filepath.Join(root, analysisRunRelativePath), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadAnalysisRun(root); !errors.Is(err, errAnalysisRunCorrupt) {
			t.Fatal("invalid or consent-bearing metadata accepted")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("metadata validation contacted a provider")
	}
}

func TestAnalysisRunStoreCloneDoesNotExposeMutableControllerState(t *testing.T) {
	server, _ := analysisResponseServer(t, emptyAnalysisReply)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	run := completedAnalysisRun(t, s)
	run.Files[0].Stages[0].Attempts = 99
	*run.Files[0].Stages[0].FindingCount = 99
	run.Plan.Files[0].Stages[0].MaxModelRequests = 99
	run.Plan.Providers[0].Stages[0] = AnalysisStageSecurityRules
	*run.Sections[0].FindingCount = 99
	current, err := s.CurrentAnalysisRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if current.Files[0].Stages[0].Attempts != 1 || *current.Files[0].Stages[0].FindingCount != 0 || current.Plan.Files[0].Stages[0].MaxModelRequests != 2 || current.Plan.Providers[0].Stages[0] != AnalysisStageSemantic || *current.Sections[0].FindingCount != 0 {
		t.Fatal("response mutation reached the run owner")
	}
}

func TestAnalysisRunStoreResumeReusesPublishedEvidenceWithNoAttemptsRemaining(t *testing.T) {
	server, calls := analysisResponseServer(t, emptyAnalysisReply)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 1}, nil)
	// Preserve a successfully published semantic report while losing only its
	// progress update, as a crash or disk failure at that boundary can do.
	s.writeAnalysisRun = func(name string, data []byte, mode os.FileMode) error {
		var run AnalysisRun
		if err := json.Unmarshal(data, &run); err != nil {
			return err
		}
		if run.Files[0].Stages[0].FindingCount != nil {
			return errors.New("progress update failed after report publication")
		}
		return storage.WriteFile(name, data, mode)
	}
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitAnalysisWindow(t, s)
	restored, err := New(scopedTestConfig(server.URL), s.manager)
	if err != nil {
		t.Fatal(err)
	}
	restored.runtimes = s.runtimes
	interrupted, err := restored.CurrentAnalysisRun(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if interrupted.Files[0].Stages[0].Attempts != 1 || calls.Load() != 1 {
		t.Fatal("fixture did not retain the spent attempt")
	}
	resume := analysisRunPreviewFor(t, restored, preview.Limits, &interrupted.Identity)
	if !resume.Files[0].Stages[0].Cached || resume.Files[0].Stages[0].MaxModelRequests != 0 || resume.ExpectedModelRequests != 2 {
		t.Fatalf("resume cache estimate=%+v", resume)
	}
	confirmations := analysisStartFor(resume).Confirmations
	if _, err := restored.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: interrupted.Identity, Action: AnalysisRunResume, PreviewID: resume.PreviewID, Confirmations: &confirmations}); err != nil {
		t.Fatal(err)
	}
	completed := completedAnalysisRun(t, restored)
	stage := completed.Files[0].Stages[0]
	if completed.Status != AnalysisRunCompletedEmpty || stage.Status != AnalysisStageCompletedEmpty || !stage.Cached || stage.Attempts != 1 || calls.Load() != 3 {
		t.Fatalf("published evidence with exhausted transport allowance: run=%s stage=%s cached=%t attempts=%d calls=%d; want completed_empty, cached, one retained attempt and three total calls", completed.Status, stage.Status, stage.Cached, stage.Attempts, calls.Load())
	}
	if _, err := os.Stat(filepath.Join(root, analysisRunRelativePath)); err != nil {
		t.Fatal(err)
	}
}

func TestAnalysisRunDetachmentRetainsFailureOnlyForCapturedProject(t *testing.T) {
	server, _, started, release := analysisBlockingServer(t)
	s, _ := newSemanticAnalysisService(t, server.URL, 0)
	root := s.manager.Root()
	original, _ := s.manager.Analysis()
	preview := analysisRunPreviewFor(t, s, AnalysisRunLimits{100, 30, 2}, nil)
	if _, err := s.StartAnalysisRun(context.Background(), analysisStartFor(preview)); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "request before detachment")
	s.analysisRun.mu.Lock()
	s.writeAnalysisRun = func(path string, data []byte, mode os.FileMode) error {
		var run AnalysisRun
		_ = json.Unmarshal(data, &run)
		if path == filepath.Join(root, analysisRunRelativePath) && run.Status == AnalysisRunStale {
			return errors.New("private stale write failure")
		}
		return storage.WriteFile(path, data, mode)
	}
	s.analysisRun.mu.Unlock()
	next := t.TempDir()
	if err := os.WriteFile(filepath.Join(next, "next.go"), []byte("package next\nfunc Next() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := s.ActivateProject(next, &project.Analysis{Name: "next", Path: next}); err != nil {
		t.Fatal(err)
	}
	release()
	waitAnalysisWindow(t, s)
	if run, err := s.CurrentAnalysisRun(context.Background()); err != nil || run != nil {
		t.Fatalf("new project inherited fault: %+v %v", run, err)
	}
	if err := s.ActivateProject(root, original); err != nil {
		t.Fatal(err)
	}
	retained, err := s.CurrentAnalysisRun(context.Background())
	if err != errAnalysisRunPersistence || retained == nil || retained.Status != AnalysisRunStale {
		t.Fatalf("lost detached fault: %+v %v", retained, err)
	}
	s.writeAnalysisRun = nil
	if _, err := s.ControlAnalysisRun(context.Background(), AnalysisRunControlRequest{Identity: retained.Identity, Action: AnalysisRunCancel}); err != nil {
		t.Fatal(err)
	}
}

func TestAnalysisRunStorePreservesPerformanceBudgetAcrossProcessLoss(t *testing.T) {
	server, calls, started, release := analysisBlockingServer(t)
	s, root := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := s.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, RunBudget: 30 * time.Second}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "performance request before restart")
	if _, err := s.PausePerformanceJob("", ""); err != nil {
		t.Fatal(err)
	}
	release()
	paused := completedAnalysisRun(t, s)
	for _, state := range []AnalysisRunStatus{AnalysisRunRunning, AnalysisRunPausing, AnalysisRunCanceling, AnalysisRunPaused} {
		t.Run(string(state), func(t *testing.T) {
			saved := cloneAnalysisRun(paused)
			saved.Status = state
			saved.CreatedAt = time.Now().Add(-2 * time.Minute).UTC()
			saved.UpdatedAt = saved.CreatedAt.Add(time.Minute)
			saved.CompatibilityElapsed = 100 * time.Millisecond
			saved.ElapsedSeconds, saved.WindowElapsedSeconds = 0, 0
			refreshAnalysisSections(saved)
			data, err := json.Marshal(saved)
			if err != nil {
				t.Fatal(err)
			}
			if err := storage.WriteFile(filepath.Join(root, analysisRunRelativePath), data, 0600); err != nil {
				t.Fatal(err)
			}
			restored, err := New(scopedTestConfig(server.URL), s.manager)
			if err != nil {
				t.Fatal(err)
			}
			restored.runtimes = s.runtimes
			current, err := restored.CurrentAnalysisRun(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			want := 30 * time.Second
			if state == AnalysisRunPaused {
				want = 100 * time.Millisecond
			}
			if current.CompatibilityElapsed != want || current.Files[0].Stages[1].Attempts != paused.Files[0].Stages[1].Attempts || restored.analysisRun.done != nil || calls.Load() != 1 {
				t.Fatalf("recovery=%+v calls=%d", current, calls.Load())
			}
			if state == AnalysisRunRunning || state == AnalysisRunPausing {
				if _, err := restored.ResumePerformanceJob(context.Background(), current.Identity.ID, false); !errors.Is(err, project.ErrRevisionConflict) {
					t.Fatalf("spent budget resumed: %v", err)
				}
			}
			persisted, err := loadAnalysisRun(root)
			if err != nil || persisted.CompatibilityElapsed != want {
				t.Fatalf("budget not durable: %+v %v", persisted, err)
			}
		})
	}
}
