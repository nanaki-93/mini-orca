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
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const validSecurityReview = `{"findings":[{"rule":"go.input.validation","category":"input-validation","title":"Input reaches an operation","source_anchor":{"path":"main.go","start_line":5,"end_line":5,"symbol":"Run"},"severity":"medium","confidence":"low","evidence_kind":"model_suspicion","observed_condition":"The operation uses input without an observed validation boundary.","preconditions_or_unknowns":"Attacker control and reachability are unknown.","remediation":"Validate untrusted input at the boundary.","verification_idea":"Use a safe unit test with a representative invalid value."}]}`

func TestReviewSecurityFilePinsPromptFocusAndReturnsSanitizedReport(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "api_key=provider-secret", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSecurityReview}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	request := securityReviewRequestFor(t, service, "Run")
	report, err := service.ReviewSecurityFile(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != project.SecurityStatusCompleted || len(report.Findings) != 1 || report.Findings[0].Anchor.Symbol != "Run" || strings.Contains(report.Model, "provider-secret") {
		t.Fatalf("report = %+v", report)
	}
	if !strings.Contains(prompt, "untrusted data") || !strings.Contains(prompt, "focused declaration") || !strings.Contains(prompt, "func Run") || strings.Contains(prompt, "helper secret") {
		t.Fatalf("security prompt lost its fixed boundary: %s", prompt)
	}
	data, err := os.ReadFile(filepath.Join(root, ".mini-orca", "security", "files", securityCacheEntry(t, root)))
	if err != nil || strings.Contains(string(data), "provider-secret") || strings.Contains(string(data), "func Run") {
		t.Fatalf("persisted report was not source-free/redacted: %v %s", err, data)
	}
}

func TestReviewSecurityFileRejectsProviderEchoedSource(t *testing.T) {
	for _, echoedSource := range []string{
		`func Run() { fmt.Println(\"run\") }`,
		"func  Run ( ) { fmt . Println ( run ) }",
		"package    main",
	} {
		t.Run(echoedSource, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				output := strings.Replace(validSecurityReview, "The operation uses input without an observed validation boundary.", echoedSource, 1)
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
			}))
			defer server.Close()
			service, root := newSemanticAnalysisService(t, server.URL, 0)
			if _, err := service.ReviewSecurityFile(context.Background(), securityReviewRequestFor(t, service, "Run")); err == nil {
				t.Fatal("source-echoing provider response was accepted")
			}
			assertNoSecurityReport(t, root)
		})
	}
}

func TestReviewSecurityFileRejectsConsentAndStaleResultsWithoutPublishing(t *testing.T) {
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		started <- struct{}{}
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSecurityReview}}}})
	}))
	defer server.Close()
	released := false
	defer func() {
		if !released {
			close(release)
		}
	}()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	service.runtimes.analyze.effective.RemoteProvider = true
	request := securityReviewRequestFor(t, service, "")
	request.ConfirmRemoteProvider = false
	if _, err := service.ReviewSecurityFile(context.Background(), request); err == nil || calls != 0 {
		t.Fatalf("declined review = %v, calls = %d", err, calls)
	}
	request.ConfirmRemoteProvider = true
	done := make(chan error, 1)
	go func() { _, err := service.ReviewSecurityFile(context.Background(), request); done <- err }()
	waitForTestSignal(t, started, "security review request")
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	close(release)
	released = true
	if err := waitForTestError(t, done, "stale security review"); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("review error = %v", err)
	}
	assertNoSecurityReport(t, root)
}

func TestReviewSecurityFileRejectsMalformedOversizedAndFocusedOutOfRangeOutput(t *testing.T) {
	for _, output := range []string{
		`{"findings":[`,
		`{"findings":[{"rule":"go.input.validation","category":"input-validation","title":"Bad","source_anchor":{"path":"main.go","start_line":1,"end_line":1,"symbol":"Run"},"severity":"medium","confidence":"low","evidence_kind":"model_suspicion","observed_condition":"Observed.","preconditions_or_unknowns":"Unknown.","remediation":"Fix.","verification_idea":"Safely test."}]}`,
		strings.Repeat("x", project.SecurityMaxSourceBytes+1),
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: output}}}})
		}))
		service, root := newSemanticAnalysisService(t, server.URL, 0)
		_, err := service.ReviewSecurityFile(context.Background(), securityReviewRequestFor(t, service, "Run"))
		server.Close()
		if err == nil {
			t.Fatal("invalid security model response was accepted")
		}
		assertNoSecurityReport(t, root)
	}
}

func TestReviewSecurityFileRejectsOver64KiBSourceBeforeProviderDelivery(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"findings":[]}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	source := "package main\n" + strings.Repeat("// bounded source\n", 5000)
	if len(source) <= project.SecurityMaxSourceBytes {
		t.Fatal("fixture did not exceed the source limit")
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReviewSecurityFile(context.Background(), securityReviewRequestFor(t, service, "")); err == nil || calls != 0 {
		t.Fatalf("oversized source error = %v, provider calls = %d", err, calls)
	}
}

func TestReviewSecurityFileRollsBackCanceledOrStalePublication(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Service, string, context.CancelFunc)
		want   error
	}{
		{name: "canceled", change: func(_ *Service, _ string, cancel context.CancelFunc) { cancel() }, want: context.Canceled},
		{name: "policy", change: func(_ *Service, root string, _ context.CancelFunc) {
			_ = os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("main.go\n"), 0644)
		}, want: project.ErrExcludedFile},
		{name: "provider", change: func(service *Service, _ string, _ context.CancelFunc) {
			service.runtimes.analyze.effective.ProviderOrigin = "https://changed.example"
		}, want: project.ErrRevisionConflict},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSecurityReview}}}})
			}))
			defer server.Close()
			service, root := newSemanticAnalysisService(t, server.URL, 0)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			service.beforeSecurityReviewPublication = func() { test.change(service, root, cancel) }
			_, err := service.ReviewSecurityFile(ctx, securityReviewRequestFor(t, service, "Run"))
			if !errors.Is(err, test.want) {
				t.Fatalf("review error = %v, want %v", err, test.want)
			}
			assertNoSecurityReport(t, root)
		})
	}
}

func TestReviewSecurityFileCompletesEmptyIndexedFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"findings":[]}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	report, err := service.ReviewSecurityFile(context.Background(), securityReviewRequestFor(t, service, ""))
	if err != nil || report.Status != project.SecurityStatusCompletedEmpty || len(report.Findings) != 0 {
		t.Fatalf("empty-file report = %+v, %v", report, err)
	}
}

func TestReviewSecurityFilePreservesSourceLinesAndRejectsRuntimeTransition(t *testing.T) {
	var prompt string
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var request llm.ChatRequest
		_ = json.NewDecoder(r.Body).Decode(&request)
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSecurityReview}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	source := "\n\npackage main\n\nfunc Run() { fmt.Println(\"run\") }\n"
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	request := securityReviewRequestFor(t, service, "Run")
	if _, err := service.ReviewSecurityFile(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "SOURCE:\n```\n"+source+"```") {
		t.Fatalf("prompt changed source line positions: %q", prompt)
	}
	snapshot, err := service.prepareSecurityReview(request)
	if err != nil {
		t.Fatal(err)
	}
	service.runtimes.analyze.effective.RemoteProvider = true
	if _, _, err := service.executeSecurityReview(context.Background(), snapshot); !errors.Is(err, project.ErrRevisionConflict) || calls != 1 {
		t.Fatalf("runtime transition error = %v, provider calls = %d", err, calls)
	}
}

func securityReviewRequestFor(t *testing.T, service *Service, symbol string) SecurityReviewRequest {
	t.Helper()
	analysis, err := service.manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := service.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	return SecurityReviewRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash, Path: file.Path, Symbol: symbol}
}

func securityCacheEntry(t *testing.T, root string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".mini-orca", "security", "files"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("security cache entries = %v, %v", entries, err)
	}
	return entries[0].Name()
}

func assertNoSecurityReport(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(root, ".mini-orca", "security", "files"))
	if os.IsNotExist(err) {
		return
	}
	if err != nil || len(entries) != 0 {
		t.Fatalf("security reports = %v, %v", entries, err)
	}
}
