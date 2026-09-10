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

const validPerformanceReview = `{"findings":[{"category":"cpu","potential_impact":"low","confidence":"low","title":"Avoid repeated formatting","observed_pattern":"The selected function formats output on each call.","workload_conditions":"This matters only when the function is called frequently.","recommendation":"Measure before changing the formatting path.","tradeoff":"Caching may retain stale output.","verification_plan":"Benchmark representative repeated calls.","start_line":5,"end_line":5,"symbol":"Run"}]}`

func TestPerformanceJobPausesResumesAndDerivesCachedReport(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		var request llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		content := validPerformanceReview
		if len(request.Messages) == 1 && strings.Contains(request.Messages[0].Content, "func Second") {
			content = `{"findings":[]}`
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: content}}}})
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })

	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\n\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Files) != 2 || preview.QueueID == "" || preview.PolicyFingerprint == "" {
		t.Fatalf("preview = %+v", preview)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 2, QueueID: preview.QueueID, PolicyFingerprint: preview.PolicyFingerprint}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "performance review request")
	if _, err := service.PausePerformanceJob(); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(release) })
	paused := waitForPerformanceFile(t, service, performanceJobPaused, performanceFileCompleted)
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "sessions", "performance-job.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResumePerformanceJob(context.Background(), paused.ID, false); err != nil {
		t.Fatal(err)
	}
	completed := waitForPerformanceJob(t, service, performanceJobCompleted)
	report, err := service.PerformanceProjectReport()
	if err != nil {
		t.Fatal(err)
	}
	if completed.Elapsed <= 0 || report == nil || report.Counts[performanceFileCompleted] != 2 || len(report.Findings) != 1 || report.Paths[report.Findings[0].ID] != "main.go" {
		t.Fatalf("job = %+v, report = %+v", completed, report)
	}
}

func TestPerformanceJobRejectsChangedPreviewAndConcurrentAnalyzeAll(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, QueueID: "performance:old"}, false); err == nil {
		t.Fatal("start accepted a changed queue identity")
	}
	job, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, QueueID: preview.QueueID, PolicyFingerprint: preview.PolicyFingerprint}, false)
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "performance review request")
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err == nil {
		t.Fatal("analyze-all started while performance job was active")
	}
	if _, err := service.CancelPerformanceJob(); err != nil {
		t.Fatal(err)
	}
	close(release)
	if canceled := waitForPerformanceJob(t, service, performanceJobCanceled); canceled.Status != performanceJobCanceled {
		t.Fatalf("canceled job = %+v", canceled)
	}
	if job.Status != performanceJobRunning {
		t.Fatalf("started job = %+v", job)
	}
}

func TestPerformanceJobReportsFailedFileAndEmptyQueue(t *testing.T) {
	t.Run("failed file", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "deterministic provider failure", http.StatusBadGateway)
		}))
		defer server.Close()
		service, _ := newSemanticAnalysisService(t, server.URL, 0)
		preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
		if err != nil || len(preview.Files) != 1 {
			t.Fatalf("preview = %+v, %v", preview, err)
		}
		if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, QueueID: preview.QueueID, PolicyFingerprint: preview.PolicyFingerprint}, false); err != nil {
			t.Fatal(err)
		}
		job := waitForPerformanceJob(t, service, performanceJobCompleted)
		report, err := service.PerformanceProjectReport()
		if err != nil {
			t.Fatal(err)
		}
		if job.Files[0].Status != performanceFileFailed || job.Files[0].Error == "" || report == nil || report.Counts[performanceFileFailed] != 1 || len(report.Findings) != 0 {
			t.Fatalf("failed performance review = job:%+v report:%+v", job, report)
		}
	})

	t.Run("empty queue", func(t *testing.T) {
		service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
		policyPath := filepath.Join(root, ".mini-orca", "context-policy.json")
		if err := os.MkdirAll(filepath.Dir(policyPath), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(policyPath, []byte(`{"exclude":["main.go"]}`), 0600); err != nil {
			t.Fatal(err)
		}
		preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
		if err != nil || len(preview.Files) != 0 || preview.Excluded != 1 {
			t.Fatalf("empty preview = %+v, %v", preview, err)
		}
		if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1, QueueID: preview.QueueID, PolicyFingerprint: preview.PolicyFingerprint}, false); err != nil {
			t.Fatal(err)
		}
		job := waitForPerformanceJob(t, service, performanceJobCompleted)
		report, err := service.PerformanceProjectReport()
		if err != nil {
			t.Fatal(err)
		}
		if len(job.Files) != 0 || report == nil || report.Status != performanceJobCompleted || len(report.Counts) != 0 || len(report.Findings) != 0 {
			t.Fatalf("empty performance review = job:%+v report:%+v", job, report)
		}
	})
}

func TestPerformanceReportsMarkStaleCachedCoverageWithoutChangingCompletedJob(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(t *testing.T, root string)
	}{
		{name: "source edit without reindex", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() { println(\"changed\") }\n"), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "source deletion without reindex", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.Remove(filepath.Join(root, "main.go")); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "source exceeds review limit without reindex", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(strings.Repeat("x", project.PerformanceMaxSourceBytes+1)), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "policy changes without reindex", change: func(t *testing.T, root string) {
			t.Helper()
			if err := os.WriteFile(filepath.Join(root, ".mini-orca", "context-policy.json"), []byte(`{"include":["main.go"]}`), 0644); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			analysis, file, policy := storeCachedPerformanceReport(t, service, root)
			preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
			if err != nil || len(preview.Files) != 1 || preview.Files[0].Status != performanceFileCached {
				t.Fatalf("initial preview = %+v, %v", preview, err)
			}
			job := &PerformanceJob{
				ID: "performance-test", Generation: "test", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
				Root: root, PolicyFingerprint: policy.Version(), QueueID: "performance:test", Status: performanceJobCompleted,
				MaxFiles: 1, RunBudget: time.Minute, Files: []PerformanceJobFile{{Path: file.Path, ContentHash: file.ContentHash, Status: performanceFileCompleted}},
			}
			service.performance.mu.Lock()
			service.performance.job = job
			service.performance.mu.Unlock()
			if err := service.persistPerformanceJob(job); err != nil {
				t.Fatal(err)
			}
			test.change(t, root)
			preview, err = service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
			if err != nil || len(preview.Files) != 1 || preview.Files[0].Status != performanceFilePending {
				t.Fatalf("stale preview = %+v, %v", preview, err)
			}

			report, err := service.PerformanceProjectReport()
			if err != nil {
				t.Fatal(err)
			}
			if report == nil || report.Status != performanceJobStale || report.Counts[performanceJobStale] != 1 || report.Counts[performanceFileCompleted] != 0 || len(report.Findings) != 0 {
				t.Fatalf("stale report = %+v", report)
			}
			job, err = loadPerformanceJob(root)
			if err != nil || job == nil || job.Status != performanceJobCompleted || job.Files[0].Status != performanceFileCompleted {
				t.Fatalf("durable job = %+v, %v", job, err)
			}
		})
	}
}

func TestPerformanceReportsPreserveLifecycleStatusWithStaleCoverage(t *testing.T) {
	for _, status := range []string{performanceJobRunning, performanceJobPaused, performanceJobCanceled, performanceJobStale} {
		t.Run(status, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			analysis, file, policy := storeCachedPerformanceReport(t, service, root)
			service.performance.mu.Lock()
			service.performance.job = &PerformanceJob{
				ID: "performance-test", Generation: "test", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
				Root: root, PolicyFingerprint: policy.Version(), QueueID: "performance:test", Status: status,
				MaxFiles: 1, RunBudget: time.Minute, Files: []PerformanceJobFile{{Path: file.Path, ContentHash: file.ContentHash, Status: performanceFileCached}},
			}
			service.performance.mu.Unlock()
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() { println(\"changed\") }\n"), 0644); err != nil {
				t.Fatal(err)
			}

			report, err := service.PerformanceProjectReport()
			if err != nil {
				t.Fatal(err)
			}
			if report == nil || report.Status != status || report.Counts[performanceJobStale] != 1 || report.Counts[performanceFileCached] != 0 || len(report.Findings) != 0 {
				t.Fatalf("report = %+v", report)
			}
		})
	}
}

func TestPerformancePreviewDoesNotReuseLegacyPromptVersionReport(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, file, policy := storeCachedPerformanceReport(t, service, root)
	legacy := project.PerformanceFileReport{
		SchemaVersion: "1", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		Path: file.Path, ContentHash: file.ContentHash, Status: "completed",
		Findings: []project.PerformanceFinding{{ID: "performance:legacy", Category: "cpu"}},
		Model:    "model", Profile: "analyze", Scope: "analyze", PromptVersion: "performance-file-v1",
		ContextPolicyVersion: policy.Version(), GeneratedAt: time.Now().UTC(),
	}
	if err := project.StorePerformanceFileReport(root, legacy); err != nil {
		t.Fatal(err)
	}
	loaded, err := project.LoadPerformanceFileReport(root, file.Path, file.ContentHash, policy)
	if err != nil || loaded == nil || loaded.Status != "stale" {
		t.Fatalf("legacy report load = %+v, %v", loaded, err)
	}
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
	if err != nil || len(preview.Files) != 1 || preview.Files[0].Status != performanceFilePending {
		t.Fatalf("legacy prompt preview = %+v, %v", preview, err)
	}
}

func TestRecoverPersistedPerformanceJobNormalizesInterruptedRequests(t *testing.T) {
	analysis := &project.Analysis{ProjectID: "project", ProjectRevision: "revision"}
	now := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name       string
		status     string
		wantStatus string
	}{
		{name: "running job", status: performanceJobRunning, wantStatus: performanceJobPaused},
		{name: "paused job", status: performanceJobPaused, wantStatus: performanceJobPaused},
		{name: "canceled job", status: performanceJobCanceled, wantStatus: performanceJobCanceled},
		{name: "stale job", status: performanceJobStale, wantStatus: performanceJobStale},
		{name: "completed job", status: performanceJobCompleted, wantStatus: performanceJobPaused},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := performanceJobFixture(t.TempDir(), analysis, test.name)
			job.Status = test.status
			job.RunBudget = time.Minute
			job.Elapsed = 45 * time.Second
			job.ActiveStartedAt = now.Add(-30 * time.Second)
			job.Files[0].Status = performanceFileRunning
			job.Files[0].Attempts = 2
			job.Files[0].Error = "interrupted"

			if !recoverPersistedPerformanceJob(job, now) {
				t.Fatal("interrupted job was not recovered")
			}
			file := job.Files[0]
			if job.Status != test.wantStatus || job.Elapsed != job.RunBudget || !job.ActiveStartedAt.IsZero() || !job.UpdatedAt.Equal(now) || file.Status != performanceFilePending || file.Attempts != 2 || file.Error != "" {
				t.Fatalf("recovered job = %+v", job)
			}
		})
	}
}

func TestPerformanceJobRecoveryResumesInterruptedFile(t *testing.T) {
	for _, initialStatus := range []string{performanceJobRunning, performanceJobPaused} {
		t.Run(initialStatus, func(t *testing.T) {
			started := make(chan struct{}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				started <- struct{}{}
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validPerformanceReview}}}})
			}))
			defer server.Close()

			service, root := newSemanticAnalysisService(t, server.URL, 0)
			analysis, err := service.manager.Analysis()
			if err != nil {
				t.Fatal(err)
			}
			policy, err := project.NewContextPolicy(root)
			if err != nil {
				t.Fatal(err)
			}
			preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
			if err != nil || len(preview.Files) != 1 {
				t.Fatalf("preview = %+v, %v", preview, err)
			}
			elapsed := 5 * time.Second
			job := &PerformanceJob{
				ID:                "performance-recovery-" + initialStatus,
				Generation:        "recovery-" + initialStatus,
				ProjectID:         analysis.ProjectID,
				ProjectRevision:   analysis.ProjectRevision,
				Root:              root,
				PolicyFingerprint: policy.Version(),
				QueueID:           preview.QueueID,
				Status:            initialStatus,
				MaxFiles:          1,
				RunBudget:         time.Minute,
				Elapsed:           elapsed,
				ActiveStartedAt:   time.Now().UTC().Add(-time.Second),
				Files:             append([]PerformanceJobFile(nil), preview.Files...),
				CreatedAt:         time.Now().UTC().Add(-time.Minute),
				UpdatedAt:         time.Now().UTC().Add(-time.Second),
			}
			job.Files[0].Status = performanceFileRunning
			job.Files[0].Attempts = 2
			if err := service.storePerformanceJob(job); err != nil {
				t.Fatal(err)
			}

			recovered, err := service.PerformanceJob()
			if err != nil {
				t.Fatal(err)
			}
			if recovered == nil || recovered.Status != performanceJobPaused || recovered.Elapsed <= elapsed || recovered.Elapsed >= recovered.RunBudget || !recovered.ActiveStartedAt.IsZero() || recovered.Files[0].Status != performanceFilePending || recovered.Files[0].Attempts != 2 {
				t.Fatalf("recovered job = %+v", recovered)
			}
			persisted, err := loadPerformanceJob(root)
			if err != nil {
				t.Fatal(err)
			}
			if persisted == nil || persisted.Status != performanceJobPaused || persisted.Elapsed != recovered.Elapsed || !persisted.ActiveStartedAt.IsZero() || persisted.Files[0].Status != performanceFilePending || persisted.Files[0].Attempts != 2 {
				t.Fatalf("persisted recovered job = %+v", persisted)
			}

			if _, err := service.ResumePerformanceJob(context.Background(), recovered.ID, false); err != nil {
				t.Fatal(err)
			}
			waitForTestSignal(t, started, "recovered performance review request")
			completed := waitForPerformanceJob(t, service, performanceJobCompleted)
			if completed.Files[0].Status != performanceFileCompleted || completed.Files[0].Attempts != 3 {
				t.Fatalf("completed recovered job = %+v", completed)
			}
		})
	}
}

func TestPersistPerformanceJobKeepsNewerStaleState(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	running := performanceJobFixture(root, analysis, "running")
	service.performance.mu.Lock()
	service.performance.job = running
	service.performance.mu.Unlock()

	delayed := clonePerformanceJob(running)
	service.invalidatePerformanceJob()
	if err := service.persistPerformanceJob(delayed); err != nil {
		t.Fatal(err)
	}

	persisted, err := loadPerformanceJob(root)
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.Status != performanceJobStale {
		t.Fatalf("persisted job after delayed running snapshot = %+v", persisted)
	}
}

func TestPerformanceJobRecoversMismatchedPersistedRootWithoutTouchingOtherProject(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	nextRoot, nextAnalysis := newPerformanceProject(t, "next")

	nextJob := performanceJobFixture(nextRoot, nextAnalysis, "next")
	nextJob.Status = performanceJobCompleted
	if err := service.storePerformanceJob(nextJob); err != nil {
		t.Fatal(err)
	}
	nextPath := filepath.Join(nextRoot, ".mini-orca", "sessions", "performance-job.json")
	before, err := os.ReadFile(nextPath)
	if err != nil {
		t.Fatal(err)
	}

	mismatched := clonePerformanceJob(nextJob)
	mismatched.Status = performanceJobRunning
	data, err := json.Marshal(mismatched)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ".mini-orca", "sessions", "performance-job.json")
	if err := storage.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}

	job, err := service.PerformanceJob()
	if err != nil {
		t.Fatal(err)
	}
	if job != nil {
		t.Fatalf("mismatched persisted job was attached: %+v", job)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("mismatched session was not recovered: %v", err)
	}
	corrupt, err := filepath.Glob(path + ".corrupt-*")
	if err != nil || len(corrupt) != 1 {
		t.Fatalf("recovered mismatched session = %v, %v", corrupt, err)
	}
	after, err := os.ReadFile(nextPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("other project session changed:\nwant %s\ngot  %s", before, after)
	}
}

func TestPerformanceJobSerializesProjectTransition(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	nextRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(nextRoot, "main.go"), []byte("package next\n\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	nextManager, err := project.NewManager(nextRoot)
	if err != nil {
		t.Fatal(err)
	}
	nextAnalysis := &project.Analysis{Name: "next", Path: nextRoot}
	if err := nextManager.Set(nextRoot, nextAnalysis); err != nil {
		t.Fatal(err)
	}
	nextJob := performanceJobFixture(nextRoot, nextAnalysis, "next")
	nextJob.Status = performanceJobCompleted
	if err := service.storePerformanceJob(nextJob); err != nil {
		t.Fatal(err)
	}

	readerReady := make(chan struct{})
	releaseReader := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseReader) }) })
	service.performance.mu.Lock()
	service.performance.beforeRead = func() {
		close(readerReady)
		<-releaseReader
	}
	service.performance.mu.Unlock()
	readerDone := make(chan error, 1)
	go func() {
		_, err := service.PerformanceJob()
		readerDone <- err
	}()
	waitForTestSignal(t, readerReady, "performance job read")
	if service.jobLifecycleMu.TryLock() {
		service.jobLifecycleMu.Unlock()
		t.Fatal("performance job read did not hold the project lifecycle lock")
	}

	transitionStarted := make(chan struct{})
	transitionDone := make(chan error, 1)
	go func() {
		close(transitionStarted)
		transitionDone <- service.ActivateProject(nextRoot, nextAnalysis)
	}()
	<-transitionStarted
	releaseOnce.Do(func() { close(releaseReader) })
	if err := <-readerDone; err != nil {
		t.Fatal(err)
	}
	if err := <-transitionDone; err != nil {
		t.Fatal(err)
	}
	service.performance.mu.Lock()
	service.performance.beforeRead = nil
	service.performance.mu.Unlock()

	current, err := service.PerformanceJob()
	if err != nil {
		t.Fatal(err)
	}
	if current == nil || current.ID != nextJob.ID || current.Status != performanceJobCompleted {
		t.Fatalf("current next-project job = %+v", current)
	}
	persisted, err := loadPerformanceJob(nextRoot)
	if err != nil {
		t.Fatal(err)
	}
	if persisted == nil || persisted.ID != nextJob.ID || persisted.Status != performanceJobCompleted {
		t.Fatalf("persisted next-project job = %+v", persisted)
	}
}

func TestProjectChangeDetachesPerformanceJob(t *testing.T) {
	for _, status := range []string{performanceJobRunning, performanceJobPaused} {
		t.Run(status, func(t *testing.T) {
			service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			analysis, err := service.manager.Analysis()
			if err != nil {
				t.Fatal(err)
			}
			old := performanceJobFixture(root, analysis, "old-"+status)
			old.Status = status
			if status == performanceJobRunning {
				old.Files[0].Status = performanceFileRunning
				old.ActiveStartedAt = time.Now().Add(-time.Second)
			}
			delayed := clonePerformanceJob(old)

			nextRoot, nextAnalysis := newPerformanceProject(t, "next")
			next := performanceJobFixture(nextRoot, nextAnalysis, "next-"+status)
			next.Status = performanceJobCompleted
			if err := service.storePerformanceJob(next); err != nil {
				t.Fatal(err)
			}

			canceled := false
			service.performance.mu.Lock()
			service.performance.job = old
			if status == performanceJobRunning {
				service.performance.cancel = func() { canceled = true }
				service.performance.workerGeneration = old.Generation
			}
			service.performance.mu.Unlock()

			if err := service.ActivateProject(nextRoot, nextAnalysis); err != nil {
				t.Fatal(err)
			}
			if err := service.persistPerformanceJob(delayed); err != nil {
				t.Fatal(err)
			}
			if status == performanceJobRunning && !canceled {
				t.Fatal("running performance worker was not canceled")
			}
			service.performance.mu.Lock()
			attached := service.performance.job
			cancel := service.performance.cancel
			generation := service.performance.workerGeneration
			service.performance.mu.Unlock()
			if attached != nil || cancel != nil || generation != "" {
				t.Fatalf("old controller remained attached: job=%+v cancel=%t generation=%q", attached, cancel != nil, generation)
			}

			current, err := service.PerformanceJob()
			if err != nil {
				t.Fatal(err)
			}
			if current == nil || current.ID != next.ID || current.Status != performanceJobCompleted {
				t.Fatalf("active project returned old or missing job: %+v", current)
			}
			oldPersisted, err := loadPerformanceJob(root)
			if err != nil {
				t.Fatal(err)
			}
			if oldPersisted == nil || oldPersisted.ID != old.ID || oldPersisted.Status != performanceJobStale {
				t.Fatalf("old project persisted job = %+v", oldPersisted)
			}
			nextPersisted, err := loadPerformanceJob(nextRoot)
			if err != nil {
				t.Fatal(err)
			}
			if nextPersisted == nil || nextPersisted.ID != next.ID || nextPersisted.Status != performanceJobCompleted {
				t.Fatalf("new project persisted job changed: %+v", nextPersisted)
			}
		})
	}
}

func TestProjectChangeDetachesPerformanceJobWithoutNextPersistedJob(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	old := performanceJobFixture(root, analysis, "old")
	old.Status = performanceJobPaused
	service.performance.mu.Lock()
	service.performance.job = old
	service.performance.mu.Unlock()

	nextRoot, nextAnalysis := newPerformanceProject(t, "empty")
	if err := service.ActivateProject(nextRoot, nextAnalysis); err != nil {
		t.Fatal(err)
	}
	current, err := service.PerformanceJob()
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("active project returned detached old job: %+v", current)
	}
	oldPersisted, err := loadPerformanceJob(root)
	if err != nil {
		t.Fatal(err)
	}
	if oldPersisted == nil || oldPersisted.ID != old.ID || oldPersisted.Status != performanceJobStale {
		t.Fatalf("old project persisted job = %+v", oldPersisted)
	}
}

func newPerformanceProject(t *testing.T, name string) (string, *project.Analysis) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package "+name+"\n\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	analysis := &project.Analysis{Name: name, Path: root}
	if err := manager.Set(root, analysis); err != nil {
		t.Fatal(err)
	}
	return root, analysis
}

func TestPerformanceWorkerCannotAffectReplacementJob(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	old := performanceJobFixture(root, analysis, "old")
	old.Files[0].Status = performanceFileRunning
	old.Files[0].Attempts = 1
	replacement := performanceJobFixture(t.TempDir(), analysis, "replacement")
	replacement.Files[0].Status = performanceFileRunning
	replacement.Files[0].Attempts = 1
	replacement.Elapsed = 5 * time.Second
	replacement.ActiveStartedAt = time.Now().Add(-time.Second)
	replacementCancelCalled := false
	service.performance.mu.Lock()
	service.performance.job = replacement
	service.performance.cancel = func() { replacementCancelCalled = true }
	service.performance.workerGeneration = replacement.Generation
	service.performance.mu.Unlock()

	service.recordPerformanceResult(old, old.Files[0].Path, nil)
	published := false
	err = service.authorizePerformancePublication(old, old.Files[0].Path, old.Files[0].ContentHash, func() error {
		published = true
		return nil
	})
	if !errors.Is(err, context.Canceled) || published {
		t.Fatalf("old worker publication = %v, published = %t", err, published)
	}
	service.clearPerformanceWorker(old.Generation)

	service.performance.mu.Lock()
	current := clonePerformanceJob(service.performance.job)
	cancel := service.performance.cancel
	generation := service.performance.workerGeneration
	service.performance.mu.Unlock()
	if current.Files[0].Status != performanceFileRunning || current.Files[0].Attempts != 1 || current.Elapsed != 5*time.Second || current.ActiveStartedAt != replacement.ActiveStartedAt || cancel == nil || generation != replacement.Generation || replacementCancelCalled {
		t.Fatalf("replacement controller after old worker = %+v, cancel=%t, generation=%q, canceled=%t", current, cancel != nil, generation, replacementCancelCalled)
	}
}

func TestValidPerformanceJobRejectsInvalidPersistedState(t *testing.T) {
	analysis := &project.Analysis{ProjectID: "project", ProjectRevision: "revision"}
	valid := performanceJobFixture(t.TempDir(), analysis, "valid")
	if validPerformanceJob(nil) {
		t.Fatal("nil persisted job was accepted")
	}
	for _, test := range []struct {
		name   string
		mutate func(*PerformanceJob)
	}{
		{name: "missing identity", mutate: func(job *PerformanceJob) { job.ID = "" }},
		{name: "too many files", mutate: func(job *PerformanceJob) { job.Files = append(job.Files, job.Files[0]) }},
		{name: "negative attempts", mutate: func(job *PerformanceJob) { job.Files[0].Attempts = -1 }},
		{name: "invalid job state", mutate: func(job *PerformanceJob) { job.Status = "unknown" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := clonePerformanceJob(valid)
			test.mutate(job)
			if validPerformanceJob(job) {
				t.Fatalf("invalid persisted job was accepted: %+v", job)
			}
		})
	}
}

func performanceJobFixture(root string, analysis *project.Analysis, generation string) *PerformanceJob {
	now := time.Now().UTC()
	return &PerformanceJob{
		ID:                "performance-" + generation,
		Generation:        generation,
		ProjectID:         analysis.ProjectID,
		ProjectRevision:   analysis.ProjectRevision,
		Root:              root,
		PolicyFingerprint: "policy",
		QueueID:           "performance:" + generation,
		Status:            performanceJobRunning,
		MaxFiles:          1,
		RunBudget:         time.Minute,
		Files: []PerformanceJobFile{{
			Path: "main.go", ContentHash: "hash", Status: performanceFilePending,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func storeCachedPerformanceReport(t *testing.T, service *Service, root string) (*project.Analysis, *project.IndexFile, *project.ContextPolicy) {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := project.NewContextPolicy(root)
	if err != nil {
		t.Fatal(err)
	}
	cachedReport := project.PerformanceFileReport{
		SchemaVersion: "1", ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision,
		Path: file.Path, ContentHash: file.ContentHash, Status: "completed",
		Findings: []project.PerformanceFinding{{ID: "performance:cached", Category: "cpu"}},
		Model:    "model", Profile: "analyze", Scope: "analyze", PromptVersion: project.PerformancePromptVersion,
		ContextPolicyVersion: policy.Version(), GeneratedAt: time.Now().UTC(),
	}
	if err := project.StorePerformanceFileReport(root, cachedReport); err != nil {
		t.Fatal(err)
	}
	return analysis, file, policy
}

func waitForPerformanceFile(t *testing.T, service *Service, jobStatus, fileStatus string) *PerformanceJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.PerformanceJob()
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == jobStatus && len(job.Files) > 0 && job.Files[0].Status == fileStatus {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("performance job did not reach %q with first file %q", jobStatus, fileStatus)
	return nil
}

func waitForPerformanceJob(t *testing.T, service *Service, want string) *PerformanceJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.PerformanceJob()
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == want {
			if want != performanceJobCompleted {
				return job
			}
			service.performance.mu.Lock()
			workerFinished := service.performance.workerGeneration == ""
			service.performance.mu.Unlock()
			if workerFinished {
				return job
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("performance job did not reach %q", want)
	return nil
}

func TestPerformanceJobPersistenceFailuresRequireDurableRecovery(t *testing.T) {
	for _, test := range []struct {
		name         string
		failedWrite  int64
		wantRequests int64
	}{
		{"admission", 2, 0},
		{"result", 3, 1},
		{"between files", 4, 1},
		{"completion", 6, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requests, writes atomic.Int64
			var fail atomic.Bool
			fail.Store(true)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests.Add(1)
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"findings":[]}`}}}})
			}))
			defer server.Close()
			service, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
			cause := errors.New("private path and provider response must stay internal")
			service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
				if writes.Add(1) >= test.failedWrite && fail.Load() {
					return cause
				}
				return storage.WriteFile(path, data, mode)
			}
			started, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 2}, false)
			if err != nil {
				t.Fatal(err)
			}
			waitForPerformanceWorkerExit(t, service)
			assertPerformancePersistenceFault(t, service)
			if got := requests.Load(); got != test.wantRequests {
				t.Fatalf("provider requests = %d, want %d", got, test.wantRequests)
			}
			service.performance.mu.Lock()
			faulted := clonePerformanceJob(service.performance.job)
			internalCause := service.performance.persistenceFault
			service.performance.mu.Unlock()
			if !errors.Is(internalCause, cause) || faulted.Status != performanceJobPaused || !faulted.ActiveStartedAt.IsZero() {
				t.Fatalf("faulted state = %+v, cause = %v", faulted, internalCause)
			}
			attempts := 0
			for _, file := range faulted.Files {
				attempts += file.Attempts
			}
			if int64(attempts) != test.wantRequests || (test.wantRequests == 0 && faulted.Elapsed != 0) || (test.wantRequests > 0 && faulted.Elapsed <= 0) {
				t.Fatalf("attempts/elapsed after fault = %+v", faulted)
			}
			policy, err := project.NewContextPolicy(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, file := range faulted.Files {
				if file.Status != performanceFileCompleted {
					continue
				}
				report, err := project.LoadPerformanceFileReport(root, file.Path, file.ContentHash, policy)
				if err != nil || report == nil || report.Status != performanceJobCompleted {
					t.Fatalf("completed report lost: %+v, %v", report, err)
				}
			}
			// A separate controller reconstructs only durable progress. Interrupted
			// admissions stay charged and become pending without automatic dispatch.
			restarted := &Service{manager: service.manager, performance: newPerformanceController()}
			recovered, err := restarted.PerformanceJob()
			if err != nil || recovered == nil || recovered.Status != performanceJobPaused || !recovered.ActiveStartedAt.IsZero() {
				t.Fatalf("restart recovery = %+v, %v", recovered, err)
			}
			for _, file := range recovered.Files {
				if file.Status == performanceFileRunning {
					t.Fatalf("restart retained running file: %+v", recovered)
				}
			}
			if _, err := service.ResumePerformanceJob(context.Background(), started.ID, false); !errors.Is(err, errPerformancePersistence) {
				t.Fatalf("failed recovery = %v", err)
			}
			if requests.Load() != test.wantRequests {
				t.Fatal("failed recovery dispatched another request")
			}
			fail.Store(false)
			assertPerformancePersistenceFault(t, service)
			if _, err := service.ResumePerformanceJob(context.Background(), started.ID, false); err != nil {
				t.Fatal(err)
			}
			waitForPerformanceWorkerExit(t, service)
			completed, err := service.PerformanceJob()
			if err != nil || completed.Status != performanceJobCompleted || requests.Load() != 2 || completed.Elapsed < faulted.Elapsed {
				t.Fatalf("completed recovery = %+v, requests = %d, error = %v", completed, requests.Load(), err)
			}
			for _, file := range completed.Files {
				if file.Status != performanceFileCompleted || file.Attempts != 1 {
					t.Fatalf("recovery repeated or lost work: %+v", completed)
				}
			}
			durable, err := loadPerformanceJob(root)
			if err != nil || durable.Status != performanceJobCompleted || durable.Elapsed != completed.Elapsed {
				t.Fatalf("durable recovery = %+v, %v", durable, err)
			}
		})
	}
}

func TestPerformanceJobDetachmentFailureIsScopedToCapturedRoot(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	old := performanceJobFixture(root, analysis, "detached-fault")
	old.Files[0].Status = performanceFileRunning
	old.Files[0].Attempts = 1
	old.ActiveStartedAt = time.Now().Add(-time.Second)
	service.performance.job = old
	service.performance.requestStarted = old.ActiveStartedAt
	if err := service.storePerformanceJob(old); err != nil {
		t.Fatal(err)
	}
	nextRoot, nextAnalysis := newPerformanceProject(t, "next")
	next := performanceJobFixture(nextRoot, nextAnalysis, "replacement")
	next.Status = performanceJobCompleted
	if err := service.storePerformanceJob(next); err != nil {
		t.Fatal(err)
	}
	canonicalRoot, err := canonicalPerformanceJobRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	var fail atomic.Bool
	fail.Store(true)
	var writes atomic.Int64
	service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
		writes.Add(1)
		if filepath.Dir(filepath.Dir(filepath.Dir(path))) == canonicalRoot && fail.Load() {
			return errors.New("private detachment failure")
		}
		return storage.WriteFile(path, data, mode)
	}
	if err := service.ActivateProject(nextRoot, nextAnalysis); err != nil {
		t.Fatal(err)
	}
	current, err := service.PerformanceJob()
	if err != nil || current.ID != next.ID {
		t.Fatalf("replacement inherited old fault: %+v, %v", current, err)
	}
	before := writes.Load()
	service.recordPerformanceResult(old, old.Files[0].Path, nil)
	if err := service.persistPerformanceJob(old); err != nil {
		t.Fatal(err)
	}
	service.clearPerformanceWorker(old.Generation)
	if writes.Load() != before {
		t.Fatal("detached worker wrote progress")
	}
	current, err = service.PerformanceJob()
	if err != nil || current.Elapsed != next.Elapsed || current.Files[0].Attempts != 0 {
		t.Fatalf("replacement accounting changed: %+v, %v", current, err)
	}
	if err := service.ActivateProject(root, analysis); err != nil {
		t.Fatal(err)
	}
	assertPerformancePersistenceFault(t, service)
	if _, err := service.ResumePerformanceJob(context.Background(), old.ID, false); !errors.Is(err, errPerformancePersistence) {
		t.Fatalf("failed detached recovery = %v", err)
	}
	fail.Store(false)
	recovered, err := service.ResumePerformanceJob(context.Background(), old.ID, false)
	if err != nil || recovered.Status != performanceJobStale || recovered.Elapsed <= 0 || !recovered.ActiveStartedAt.IsZero() || recovered.Files[0].Attempts != 1 {
		t.Fatalf("detached recovery = %+v, %v", recovered, err)
	}
	service.performance.mu.Lock()
	active := service.performance.workerGeneration
	retained := len(service.performance.detachedFaults)
	service.performance.mu.Unlock()
	if active != "" || retained != 0 {
		t.Fatalf("detached recovery dispatched or retained fault: worker = %q, faults = %d", active, retained)
	}
	durable, err := loadPerformanceJob(root)
	if err != nil || durable.Status != performanceJobStale || durable.Elapsed != recovered.Elapsed {
		t.Fatalf("detached durable recovery = %+v, %v", durable, err)
	}
}

func TestPerformanceJobObsoleteSaveCannotFaultSameRootReplacement(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	old := performanceJobFixture(root, analysis, "old")
	next := performanceJobFixture(root, analysis, "next")
	service.performance.job = next
	service.writePerformanceJob = func(string, []byte, os.FileMode) error {
		t.Fatal("obsolete worker attempted to save replacement")
		return errors.New("unexpected write")
	}
	if err := service.persistPerformanceJob(old); err != nil {
		t.Fatal(err)
	}
	current, err := service.PerformanceJob()
	if err != nil || current.ID != next.ID || current.Status != performanceJobRunning {
		t.Fatalf("replacement after obsolete save = %+v, %v", current, err)
	}
}

func assertPerformancePersistenceFault(t *testing.T, service *Service) {
	t.Helper()
	if job, err := service.PerformanceJob(); job != nil || !errors.Is(err, errPerformancePersistence) || strings.Contains(err.Error(), "private") {
		t.Fatalf("progress fault = %+v, %v", job, err)
	}
	if report, err := service.PerformanceProjectReport(); report != nil || !errors.Is(err, errPerformancePersistence) {
		t.Fatalf("report fault = %+v, %v", report, err)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{}, false); !errors.Is(err, errPerformancePersistence) {
		t.Fatalf("start bypassed fault: %v", err)
	}
}

func waitForPerformanceWorkerExit(t *testing.T, service *Service) {
	t.Helper()
	service.performance.mu.Lock()
	done := service.performance.workerDone
	service.performance.mu.Unlock()
	if done != nil {
		waitForTestSignal(t, done, "performance worker exit")
	}
}

func TestPerformanceJobDoesNotPublishCompletionBeforeSave(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"findings":[]}`}}}})
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
		var job PerformanceJob
		if err := json.Unmarshal(data, &job); err != nil {
			return err
		}
		if job.Status == performanceJobCompleted {
			close(entered)
			<-release
			return errors.New("private completion error")
		}
		return storage.WriteFile(path, data, mode)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, entered, "blocked completion save")
	job, err := service.PerformanceJob()
	if err != nil || job.Status != performanceJobRunning || job.Files[0].Status != performanceFileCompleted {
		t.Fatalf("progress claimed unpersisted completion: %+v, %v", job, err)
	}
	report, err := service.PerformanceProjectReport()
	if err != nil || report.Status == performanceJobCompleted {
		t.Fatalf("report claimed unpersisted completion: %+v, %v", report, err)
	}
	releaseOnce.Do(func() { close(release) })
	waitForPerformanceWorkerExit(t, service)
	assertPerformancePersistenceFault(t, service)
}

func TestPerformanceJobExhaustedCompletionFaultCanBeFinalized(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	job := performanceJobFixture(service.manager.Root(), analysis, "exhausted")
	job.PolicyFingerprint, job.QueueID, job.Files = preview.PolicyFingerprint, preview.QueueID, preview.Files
	job.Elapsed = job.RunBudget
	service.performance.job = job
	if err := service.persistPerformanceJob(job); err != nil {
		t.Fatal(err)
	}
	fail := true
	service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
		if fail {
			return errors.New("completion failure")
		}
		return storage.WriteFile(path, data, mode)
	}
	service.runPerformanceJob(context.Background(), job.Generation)
	assertPerformancePersistenceFault(t, service)
	if _, err := service.ResumePerformanceJob(context.Background(), job.ID, false); !errors.Is(err, errPerformancePersistence) {
		t.Fatalf("exhausted recovery failure = %v", err)
	}
	fail = false
	final, err := service.ResumePerformanceJob(context.Background(), job.ID, false)
	if err != nil || final.Status != performanceJobCompleted || final.Elapsed != final.RunBudget || final.Files[0].Attempts != 0 || final.Files[0].Status != performanceFilePending {
		t.Fatalf("exhausted recovery = %+v, %v", final, err)
	}
	if service.performance.workerGeneration != "" {
		t.Fatal("exhausted recovery dispatched work")
	}
	durable, err := loadPerformanceJob(root)
	if err != nil || durable.Status != performanceJobCompleted || durable.Elapsed != job.RunBudget {
		t.Fatalf("exhausted durable recovery = %+v, %v", durable, err)
	}
}

func TestPerformanceJobCanceledSaveFailureRetainsAccounting(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	var fail atomic.Bool
	service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
		if fail.Load() {
			return errors.New("canceled save failure")
		}
		return storage.WriteFile(path, data, mode)
	}
	job, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1}, false)
	if err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "cancelable performance request")
	fail.Store(true)
	if _, err := service.CancelPerformanceJob(); !errors.Is(err, errPerformancePersistence) {
		t.Fatalf("cancel failure = %v", err)
	}
	waitForPerformanceWorkerExit(t, service)
	assertPerformancePersistenceFault(t, service)
	fail.Store(false)
	canceled, err := service.ResumePerformanceJob(context.Background(), job.ID, false)
	if err != nil || canceled.Status != performanceJobCanceled || canceled.Elapsed <= 0 || !canceled.ActiveStartedAt.IsZero() || canceled.Files[0].Attempts != 1 {
		t.Fatalf("canceled recovery = %+v, %v", canceled, err)
	}
	durable, err := loadPerformanceJob(root)
	if err != nil || durable.Elapsed != canceled.Elapsed || durable.Status != performanceJobCanceled {
		t.Fatalf("canceled durable recovery = %+v, %v", durable, err)
	}
}

func TestPerformanceJobFailedAdmissionDetachesWithoutChargingRequest(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	nextRoot, nextAnalysis := newPerformanceProject(t, "next")
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
		var job PerformanceJob
		if err := json.Unmarshal(data, &job); err != nil {
			return err
		}
		if job.Status == performanceJobRunning && job.Files[0].Status == performanceFileRunning {
			close(entered)
			<-release
			return errors.New("admission failure")
		}
		return storage.WriteFile(path, data, mode)
	}
	if _, err := service.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, entered, "blocked admission save")
	if service.performance.persistMu.TryLock() {
		service.performance.persistMu.Unlock()
		t.Fatal("admission did not hold persistence serialization")
	}
	transition := make(chan error, 1)
	go func() { transition <- service.ActivateProject(nextRoot, nextAnalysis) }()
	releaseOnce.Do(func() { close(release) })
	select {
	case err := <-transition:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("detachment did not complete")
	}
	old, err := loadPerformanceJob(root)
	if err != nil || old.Status != performanceJobStale || old.Files[0].Attempts != 0 || old.Files[0].Status != performanceFilePending || old.Elapsed != 0 || !old.ActiveStartedAt.IsZero() {
		t.Fatalf("detached failed admission = %+v, %v", old, err)
	}
	if current, err := service.PerformanceJob(); err != nil || current != nil {
		t.Fatalf("replacement inherited failed admission: %+v, %v", current, err)
	}
}

func TestPerformanceJobResumeRejectsFaultedAnalyzeAll(t *testing.T) {
	service, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	job := performanceJobFixture(root, analysis, "paused")
	job.Status = performanceJobPaused
	service.performance.job = job
	service.analysisAll.job = &AnalyzeAllJob{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, Status: analysisAllStatePaused, root: service.manager.Root()}
	service.analysisAll.persistenceFault = errors.New("analyze-all save failure")
	service.writePerformanceJob = func(string, []byte, os.FileMode) error {
		t.Fatal("performance recovery wrote while analyze-all was faulted")
		return nil
	}
	if _, err := service.ResumePerformanceJob(context.Background(), job.ID, false); !errors.Is(err, errAnalyzeAllPersistence) {
		t.Fatalf("resume with faulted analyze-all = %v", err)
	}
}

func TestPerformanceJobRecoveryWaitsForConcurrentResultFailure(t *testing.T) {
	for _, test := range []struct {
		name, initial, returned, final string
		action                         func(*Service, string) (*PerformanceJob, error)
	}{
		{"resume", performanceJobPaused, performanceJobRunning, performanceJobCompleted, func(s *Service, id string) (*PerformanceJob, error) {
			return s.ResumePerformanceJob(context.Background(), id, false)
		}},
		{"pause", performanceJobRunning, performanceJobPaused, performanceJobPaused, func(s *Service, _ string) (*PerformanceJob, error) { return s.PausePerformanceJob() }},
		{"cancel", performanceJobRunning, performanceJobCanceled, performanceJobCanceled, func(s *Service, _ string) (*PerformanceJob, error) { return s.CancelPerformanceJob() }},
	} {
		t.Run(test.name, func(t *testing.T) {
			service, job := performanceJobWithInFlightFile(t, test.initial)
			entered, release, workerDone := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			defer releaseOnce.Do(func() { close(release) })
			var writes atomic.Int64
			service.writePerformanceJob = func(path string, data []byte, mode os.FileMode) error {
				if writes.Add(1) == 1 {
					close(entered)
					<-release
					return errors.New("result save failed during recovery")
				}
				return storage.WriteFile(path, data, mode)
			}
			service.performance.cancel = func() {}
			service.performance.workerGeneration = job.Generation
			service.performance.workerDone = workerDone
			go func() {
				defer close(workerDone)
				defer service.clearPerformanceWorker(job.Generation)
				service.recordPerformanceResult(job, job.Files[0].Path, nil)
			}()
			waitForTestSignal(t, entered, "blocked worker result save")
			actionRead := make(chan struct{})
			var readOnce sync.Once
			service.performance.mu.Lock()
			service.performance.beforeRead = func() { readOnce.Do(func() { close(actionRead) }) }
			service.performance.mu.Unlock()
			type actionResult struct {
				job *PerformanceJob
				err error
			}
			result := make(chan actionResult, 1)
			go func() {
				published, err := test.action(service, job.ID)
				result <- actionResult{published, err}
			}()
			waitForTestSignal(t, actionRead, "recovery reading pending progress")
			releaseOnce.Do(func() { close(release) })
			select {
			case result := <-result:
				if result.err != nil || result.job.Status != test.returned {
					t.Fatalf("recovery result = %+v, %v", result.job, result.err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("recovery did not finish after result persistence failed")
			}
			waitForTestSignal(t, workerDone, "faulted worker exit")
			waitForPerformanceWorkerExit(t, service)
			current, err := service.PerformanceJob()
			if err != nil || current.Status != test.final || current.Files[0].Status != performanceFileCompleted || current.Files[0].Attempts != 1 || current.Elapsed <= 0 || !current.ActiveStartedAt.IsZero() {
				t.Fatalf("recovered progress = %+v, %v", current, err)
			}
			durable, err := loadPerformanceJob(job.Root)
			if err != nil || durable.Status != current.Status || durable.Elapsed != current.Elapsed {
				t.Fatalf("durable recovery = %+v, %v", durable, err)
			}
		})
	}
}

func TestPerformanceJobRecoveryRetainsPersistenceAuthority(t *testing.T) {
	service, _ := performanceJobWithInFlightFile(t, performanceJobPaused)
	service.jobLifecycleMu.Lock()
	defer service.jobLifecycleMu.Unlock()
	job, fault, finish, err := service.beginPerformanceRecoveryLocked()
	if err != nil {
		t.Fatal(err)
	}
	defer finish()
	if service.performance.persistMu.TryLock() {
		service.performance.persistMu.Unlock()
		t.Fatal("recovery released persistence authority before preparing and saving state")
	}
	published, _, err := service.preparePerformanceResume(job, fault)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.persistPerformanceJobSerialized(published, true); err != nil {
		t.Fatal(err)
	}
	durable, err := loadPerformanceJob(job.Root)
	if err != nil || durable.Status != published.Status {
		t.Fatalf("recovery publication differs from durable state: %+v, %v", durable, err)
	}
}

func performanceJobWithInFlightFile(t *testing.T, status string) (*Service, *PerformanceJob) {
	t.Helper()
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewPerformanceQueue(PerformanceJobOptions{MaxFiles: 1})
	if err != nil {
		t.Fatal(err)
	}
	job := performanceJobFixture(service.manager.Root(), analysis, "recovery-result-race")
	job.PolicyFingerprint, job.QueueID, job.Files = preview.PolicyFingerprint, preview.QueueID, preview.Files
	job.Status = status
	job.Files[0].Status, job.Files[0].Attempts = performanceFileRunning, 1
	job.ActiveStartedAt = time.Now().Add(-time.Second)
	service.performance.job = job
	service.performance.requestStarted = job.ActiveStartedAt
	if err := service.persistPerformanceJob(job); err != nil {
		t.Fatal(err)
	}
	return service, clonePerformanceJob(job)
}
