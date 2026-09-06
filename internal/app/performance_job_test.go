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
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("performance job did not reach %q", want)
	return nil
}
