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
