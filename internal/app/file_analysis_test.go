package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestAnalyzeFileCachesStructuredOneFileSummary(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "fixture-model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs the selected command.","responsibilities":["dispatches work"],"dependencies":["fmt"],"side_effects":["writes stdout"],"risks":[{"severity":"High","summary":"No input validation."}],"suggestions":[{"title":"Validate input","summary":"Reject blank names.","target_symbol":"Run","action":"fix"}],"symbol_explanations":{"Run":"Dispatches the command."}}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != project.AnalysisStatusFresh || result.Purpose == "" || len(result.Symbols) != 1 || result.SymbolExplanations["Run"] == "" || result.Model != "fixture-model" || len(result.Risks) != 1 || result.Risks[0].Severity != "high" {
		t.Fatalf("analysis = %+v", result)
	}
	if strings.Contains(prompt, "helper secret") || !strings.Contains(prompt, "func Run") || !strings.Contains(prompt, "TARGET_SOURCE (the only source content supplied)") {
		t.Fatalf("semantic prompt was not one-file scoped: %s", prompt)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "file-analysis")); err != nil {
		t.Fatalf("analysis cache not written: %v", err)
	}
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	if file := findAppIndexFile(index, "main.go"); file == nil || file.AnalysisStatus != project.AnalysisStatusFresh {
		t.Fatalf("index analysis status = %+v, want fresh", file)
	}
	second, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || second.Status != project.AnalysisStatusFresh {
		t.Fatalf("cached analysis = %+v, %v", second, err)
	}
}

func TestClearFileAnalysisResetsIndexStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.AnalyzeFile(context.Background(), "main.go", false, false); err != nil {
		t.Fatal(err)
	}
	if err := service.ClearFileAnalysis("main.go"); err != nil {
		t.Fatal(err)
	}
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	if file := findAppIndexFile(index, "main.go"); file == nil || file.AnalysisStatus != project.AnalysisStatusMissing {
		t.Fatalf("cleared index analysis status = %+v, want missing", file)
	}
	cached, err := service.CachedFileAnalysis("main.go")
	if err != nil || cached.Status != project.AnalysisStatusMissing {
		t.Fatalf("cleared cache = %+v, %v", cached, err)
	}
}

func TestAnalyzeFileAcceptsUnqualifiedMethodExplanations(t *testing.T) {
	output := `{"purpose":"Explains the service.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{"FileSystemService":"The service implementation.","PrintFiles":"Prints files.","ListFiles":"Lists files."}}`
	parsed, err := parseSemanticAnalysis(output, project.IndexFile{Path: "service.go", Language: "Go", Symbols: []project.SymbolInfo{
		{Name: "FileSystemService"},
		{Name: "FileSystemService.PrintFiles"},
		{Name: "FileSystemService.ListFiles"},
	}}, "package service")
	if err != nil {
		t.Fatal(err)
	}
	if parsed.SymbolExplanations["FileSystemService.PrintFiles"] != "Prints files." || parsed.SymbolExplanations["FileSystemService.ListFiles"] != "Lists files." {
		t.Fatalf("normalized symbol explanations = %+v", parsed.SymbolExplanations)
	}
	if _, exists := parsed.SymbolExplanations["PrintFiles"]; exists {
		t.Fatalf("unqualified symbol explanation was not normalized: %+v", parsed.SymbolExplanations)
	}
}

func TestAnalyzeFileValidatesAndCachesOneExactBugTaskWithoutAnotherModelCall(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Explains the file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"Run ignores an error.","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"hallucinated","acceptance_criteria":["Return the formatting error to the caller."],"non_goals":["Do not edit helper files."],"go_test_candidate":{"name":"TestRunReturnsError","content":"package main\n\nimport \"testing\"\n\nfunc TestRunReturnsError(t *testing.T) {}\n"}}}],"suggestions":[],"symbol_explanations":{}}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)

	first, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	task := first.Risks[0].TaskSpec
	if task == nil || task.TargetPath != "main.go" || task.TargetSymbol != "Run" || task.TargetSignature != "func()" || task.GoTestCandidate == nil {
		t.Fatalf("validated task = %+v", task)
	}
	content, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil || string(content) != "package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"run\") }\n" {
		t.Fatalf("analysis changed imported source: %q, %v", content, err)
	}
	second, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || second.Risks[0].TaskSpec == nil || calls != 1 {
		t.Fatalf("cached task = %+v, %v; calls = %d", second, err, calls)
	}
}

func TestSemanticBugTaskRejectsUntrustedTargetsAndDropsInvalidOptionalTests(t *testing.T) {
	target := project.IndexFile{Path: "main.go", Language: "Go", Symbols: []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "exact", AtomicTarget: true}}}
	valid := func(task string) string {
		return `{"purpose":"Explains.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"Risk.","task_spec":` + task + `}],"suggestions":[],"symbol_explanations":{}}`
	}
	spec := func(path, symbol string) string {
		return `{"schema_version":"1","target_path":"` + path + `","target_symbol":"` + symbol + `","acceptance_criteria":["Handle the error."],"non_goals":[]}`
	}
	for _, output := range []string{
		valid(spec("other.go", "Run")),
		valid(spec("main.go", "Unknown")),
		valid(`{"schema_version":"1","target_path":"main.go","target_symbol":"Run","acceptance_criteria":["` + strings.Repeat("x", project.MaxBugTaskItemBytes+1) + `"],"non_goals":[]}`),
	} {
		if _, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}"); err == nil {
			t.Fatalf("untrusted task was accepted: %s", output)
		}
	}
	approximate := target
	approximate.Symbols = []project.SymbolInfo{{Name: "Run", Signature: "func Run()", Confidence: "approximate", AtomicTarget: true}}
	if _, err := parseSemanticAnalysis(valid(spec("main.go", "Run")), approximate, "package main\nfunc Run() {}"); err == nil {
		t.Fatal("approximate target was accepted")
	}
	nonAtomic := target
	nonAtomic.Symbols = append([]project.SymbolInfo(nil), target.Symbols...)
	nonAtomic.Symbols[0].AtomicTarget = false
	if _, err := parseSemanticAnalysis(valid(spec("main.go", "Run")), nonAtomic, "package main\nfunc Run() {}"); err == nil {
		t.Fatal("non-atomic target was accepted")
	}
	duplicate := target
	duplicate.Symbols = append(duplicate.Symbols, duplicate.Symbols[0])
	if _, err := parseSemanticAnalysis(valid(spec("main.go", "Run")), duplicate, "package main\nfunc Run() {}"); err == nil {
		t.Fatal("duplicate target was accepted")
	}

	for _, candidate := range []string{
		`{"name":"TestRun","content":"package other\nfunc TestRun() {}"}`,
		`{"name":"TestRun","content":"package main\nfunc TestRun( {"}`,
		`{"name":"NotATest","content":"package main\nfunc NotATest() {}"}`,
	} {
		output := valid(`{"schema_version":"1","target_path":"main.go","target_symbol":"Run","acceptance_criteria":["Handle the error."],"non_goals":[],"go_test_candidate":` + candidate + `}`)
		parsed, err := parseSemanticAnalysis(output, target, "package main\nfunc Run() {}")
		if err != nil || parsed.Risks[0].TaskSpec == nil || parsed.Risks[0].TaskSpec.GoTestCandidate != nil {
			t.Fatalf("invalid optional test = %+v, %v", parsed.Risks[0].TaskSpec, err)
		}
	}
}

func TestAnalyzeAllRetriesFailedFiles(t *testing.T) {
	var mu sync.Mutex
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		callCount++
		call := callCount
		mu.Unlock()
		output := validSemanticAnalysis
		if call == 1 {
			output = "not-json"
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	failed, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || failed.Status != project.AnalysisStatusFailed {
		t.Fatalf("initial analysis = %+v, %v", failed, err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1, MaxRetries: 0}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 1 || job.Files[0].Path != "main.go" || job.Files[0].Status != analysisAllFileCompleted {
		t.Fatalf("retry job = %+v", job)
	}
	result, err := service.CachedFileAnalysis("main.go")
	if err != nil || result.Status != project.AnalysisStatusFresh {
		t.Fatalf("retried analysis = %+v, %v", result, err)
	}
	mu.Lock()
	gotCalls := callCount
	mu.Unlock()
	if gotCalls != 2 {
		t.Fatalf("model call count = %d, want initial failure plus retry", gotCalls)
	}
}

func TestAnalyzeFileStoresActionableMalformedAndEmptyFailures(t *testing.T) {
	for _, output := range []string{"not-json", ""} {
		t.Run("output", func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
			}))
			defer server.Close()
			service, _ := newSemanticAnalysisService(t, server.URL, 0)
			result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
			if err != nil || result.Status != project.AnalysisStatusFailed || result.Failure == "" {
				t.Fatalf("malformed result = %+v, %v", result, err)
			}
			index, err := service.manager.Index()
			if err != nil {
				t.Fatal(err)
			}
			if file := findAppIndexFile(index, "main.go"); file == nil || file.AnalysisStatus != project.AnalysisStatusFailed {
				t.Fatalf("index analysis status = %+v, want failed", file)
			}
		})
	}
}

func TestAnalyzeFileHonorsTimeout(t *testing.T) {
	handlerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		handlerCanceled <- struct{}{}
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 1)
	_, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("analysis error = %v, want deadline exceeded", err)
	}
	waitForTestSignal(t, handlerCanceled, "analysis provider cancellation")
}

func TestAnalyzeAllProcessesEligibleFilesSequentially(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	handlerErrors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			handlerErrors <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		paths = append(paths, path)
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCompleted)
	if len(job.Files) != 2 || job.Files[0].Status != analysisAllFileCompleted || job.Files[1].Status != analysisAllFileCompleted {
		t.Fatalf("completed job = %+v", job)
	}
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"main.go", "second.go"} {
		if file := findAppIndexFile(index, path); file == nil || file.AnalysisStatus != project.AnalysisStatusFresh {
			t.Fatalf("index analysis status for %s = %+v, want fresh", path, file)
		}
	}
	mu.Lock()
	got := append([]string(nil), paths...)
	mu.Unlock()
	if want := []string{"main.go", "second.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("request order = %v, want %v", got, want)
	}
	assertNoTestHandlerError(t, handlerErrors)
}

func TestAnalyzeAllCancelRetainsCompletedEntries(t *testing.T) {
	firstDone := make(chan struct{}, 1)
	secondStarted := make(chan struct{}, 1)
	secondCanceled := make(chan struct{}, 1)
	handlerErrors := make(chan error, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			handlerErrors <- err
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if path == "main.go" {
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
			firstDone <- struct{}{}
			return
		}
		if path != "second.go" {
			handlerErrors <- fmt.Errorf("analysis target path = %q, want second.go", path)
			http.Error(w, "unexpected analysis target", http.StatusBadRequest)
			return
		}
		secondStarted <- struct{}{}
		<-r.Context().Done()
		secondCanceled <- struct{}{}
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, firstDone, "first analysis completion")
	waitForTestSignal(t, secondStarted, "second analysis request")
	if _, err := service.CancelAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateCanceled)
	waitForTestSignal(t, secondCanceled, "second analysis cancellation")
	if job.Files[0].Status != analysisAllFileCompleted || job.Files[1].Status == analysisAllFileCompleted {
		t.Fatalf("canceled job should preserve only the completed cache entry: %+v", job)
	}
	analysis, err := service.CachedFileAnalysis("main.go")
	if err != nil || analysis.Status != project.AnalysisStatusFresh {
		t.Fatalf("completed cache entry = %+v, %v", analysis, err)
	}
	assertNoTestHandlerError(t, handlerErrors)
}

func TestAnalyzeAllPersistsPausedJobAndResumesAfterServiceRestart(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	var releaseOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, err := analyzeAllRequestPath(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if path == "main.go" {
			started <- struct{}{}
			select {
			case <-release:
			case <-r.Context().Done():
			}
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer server.Close()
	defer releaseOnce.Do(func() { close(release) })
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "second.go"), []byte("package main\nfunc Second() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 2}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "first analysis request")
	if _, err := service.PauseAnalyzeAll(); err != nil {
		t.Fatal(err)
	}
	releaseOnce.Do(func() { close(release) })
	waitForAnalyzeAll(t, service, analysisAllStatePaused)

	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig(server.URL)
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	restarted, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.ResumeAnalyzeAll(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, restarted, analysisAllStateCompleted)
	if len(job.Files) != 2 || job.Files[1].Status != analysisAllFileCompleted {
		t.Fatalf("resumed job = %+v", job)
	}
}

func TestAnalyzeAllBecomesStaleWhenReindexChangesRevision(t *testing.T) {
	started := make(chan struct{}, 1)
	handlerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-r.Context().Done()
		handlerCanceled <- struct{}{}
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if _, err := service.StartAnalyzeAll(context.Background(), AnalyzeAllOptions{MaxFiles: 1}, false); err != nil {
		t.Fatal(err)
	}
	waitForTestSignal(t, started, "analysis request")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() { println(\"changed\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	job := waitForAnalyzeAll(t, service, analysisAllStateStale)
	waitForTestSignal(t, handlerCanceled, "stale analysis cancellation")
	if job.Status != analysisAllStateStale {
		t.Fatalf("reindexed job = %+v", job)
	}
}

const validSemanticAnalysis = `{"purpose":"Explains the file.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`

func waitForAnalyzeAll(t *testing.T, service *Service, want string) *AnalyzeAllJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job, err := service.AnalyzeAllJob()
		if err != nil {
			t.Fatal(err)
		}
		if job != nil && job.Status == want {
			return job
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("analyze-all job did not reach %q", want)
	return nil
}

func waitForTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForTestError(t *testing.T, done <-chan error, description string) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func assertNoTestHandlerError(t *testing.T, handlerErrors <-chan error) {
	t.Helper()
	select {
	case err := <-handlerErrors:
		t.Fatal(err)
	default:
	}
}

func analyzeAllRequestPath(r *http.Request) (string, error) {
	var request llm.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		return "", fmt.Errorf("decode LLM request: %w", err)
	}
	if len(request.Messages) != 1 {
		return "", fmt.Errorf("LLM messages = %d, want 1", len(request.Messages))
	}
	const marker = "TARGET_FACTS:\n{\"path\":\""
	prompt := request.Messages[0].Content
	start := strings.Index(prompt, marker)
	if start < 0 {
		return "", fmt.Errorf("target facts missing from semantic prompt")
	}
	path := prompt[start+len(marker):]
	var found bool
	path, _, found = strings.Cut(path, "\"")
	if !found || path == "" {
		return "", fmt.Errorf("target path missing from semantic prompt")
	}
	return path, nil
}

func findAppIndexFile(index *project.ProjectIndex, path string) *project.IndexFile {
	for i := range index.Files {
		if index.Files[i].Path == path {
			return &index.Files[i]
		}
	}
	return nil
}

func newSemanticAnalysisService(t *testing.T, baseURL string, analysisSeconds int) (*Service, string) {
	return newSemanticAnalysisServiceFixture(t, baseURL, analysisSeconds, false)
}

func newSemanticAnalysisServiceWithHelper(t *testing.T, baseURL string, analysisSeconds int) (*Service, string) {
	return newSemanticAnalysisServiceFixture(t, baseURL, analysisSeconds, true)
}

func newSemanticAnalysisServiceFixture(t *testing.T, baseURL string, analysisSeconds int, includeHelper bool) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"run\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if includeHelper {
		if err := os.WriteFile(filepath.Join(root, "helper.go"), []byte("package main\n\nfunc Helper() { println(\"helper secret\") }\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root, Summary: "A compact fixture project."}); err != nil {
		t.Fatal(err)
	}
	cfg := scopedTestConfig(baseURL)
	cfg.ModelScopes.Bug.Model = "analysis-model"
	cfg.Timeouts = config.TimeoutConfig{AnalysisSeconds: analysisSeconds}
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	service, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	return service, root
}
