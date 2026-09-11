package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	if _, err := service.PausePerformanceJob("", ""); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(release) })
	paused := waitForPerformanceFile(t, service, performanceJobPaused, performanceFileCompleted)
	if _, err := os.Stat(filepath.Join(root, analysisRunRelativePath)); err != nil {
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
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer unblock()
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
	if _, err := service.CancelPerformanceJob("", ""); err != nil {
		t.Fatal(err)
	}
	unblock()
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
			service.analysisRun.mu.Lock()
			workerFinished := service.analysisRun.done == nil
			service.analysisRun.mu.Unlock()
			if workerFinished {
				return job
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("performance job did not reach %q", want)
	return nil
}

func TestPerformanceLegacyReportKeepsStaleCoverageAndLifecycle(t *testing.T) {
	for _, state := range []string{"completed", "canceled", "paused"} {
		t.Run(state, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validPerformanceReview}}}})
			}))
			defer server.Close()
			s, root := newSemanticAnalysisService(t, server.URL, 0)
			if _, err := s.StartPerformanceJob(context.Background(), PerformanceJobOptions{MaxFiles: 1}, false); err != nil {
				t.Fatal(err)
			}
			waitAnalysisWindow(t, s)
			job, err := s.PerformanceJob()
			if err != nil {
				t.Fatal(err)
			}
			job.Status = state
			data, _ := json.Marshal(job)
			if err := storage.WriteFile(filepath.Join(root, ".mini-orca/sessions/performance-job.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			// The legacy report fallback reads original metadata after migration.
			if err := os.Remove(filepath.Join(root, analysisRunRelativePath)); err != nil {
				t.Fatal(err)
			}
			s.analysisRun = &analysisRunController{}
			if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0644); err != nil {
				t.Fatal(err)
			}
			report, err := s.PerformanceProjectReport()
			if err != nil {
				t.Fatal(err)
			}
			want := state
			if state == "completed" {
				want = "stale"
			}
			if state == "paused" {
				want = "interrupted"
			}
			if report.Status != want || report.Counts["stale"] != 1 || len(report.Findings) != 0 {
				t.Fatalf("stale report=%+v", report)
			}
			after, _ := os.ReadFile(filepath.Join(root, ".mini-orca/sessions/performance-job.json"))
			if string(after) != string(data) {
				t.Fatal("report rewrote legacy progress")
			}
		})
	}
}

func TestPerformanceLegacyCorruptRootIsPreservedWithoutOtherProjectWrites(t *testing.T) {
	s, root := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	analysis, _ := s.manager.Analysis()
	job := performanceJobFixture(t.TempDir(), analysis, "wrong-root")
	data, _ := json.Marshal(job)
	file := filepath.Join(root, ".mini-orca/sessions/performance-job.json")
	if err := storage.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PerformanceJob(); err == nil {
		t.Fatal("mismatched root accepted")
	}
	after, _ := os.ReadFile(file)
	if string(after) != string(data) {
		t.Fatal("corrupt history moved or replaced")
	}
	if _, err := os.Stat(filepath.Join(job.Root, ".mini-orca")); !os.IsNotExist(err) {
		t.Fatalf("foreign root touched: %v", err)
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
