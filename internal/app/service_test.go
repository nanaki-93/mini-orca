package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

func TestGenerateUsesConfiguredCoderProfile(t *testing.T) {
	var received llm.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{
			Model:   "coder-override",
			Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Role: "assistant", Content: `{"version":"v1","target_path":"sample.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package sample\n\nfunc Run() {}"}`}}},
		})
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{
		LLM:    config.LLMConfig{BaseURL: server.URL, Model: "default-model", Temperature: 0.25, MaxTokens: 321},
		Agents: config.AgentsConfig{Coder: config.AgentConfig{Model: "coder-override", Skills: []string{"custom_skill"}, TimeoutSeconds: 30}},
		Retry:  config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}

	preview, err := service.Generate(context.Background(), "improve Run", "sample.go", "Run", workflow.ScopeStrictSymbol, false)
	if err != nil {
		t.Fatal(err)
	}
	if preview.GenerationID == "" || preview.BaseFileHash == "" || preview.CandidateHash == "" || preview.CandidateContent != "package sample\n\nfunc Run() {}" || preview.ScopeMode != workflow.ScopeStrictSymbol || preview.EffectiveModel.Model != "coder-override" || len(preview.ContextManifest.Included) != 1 {
		t.Fatalf("generation preview = %+v", preview)
	}
	if received.Model != "coder-override" || received.Temperature != 0.25 || received.MaxTokens != 321 {
		t.Fatalf("generation used %+v, want configured coder model and LLM settings", received)
	}
	if len(received.Messages) != 1 || !strings.Contains(received.Messages[0].Content, "Use the custom_skill skill.") {
		t.Fatalf("coder skills missing from prompt: %+v", received.Messages)
	}
	manifest, err := service.ContextManifest("sample.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Included) != 1 || manifest.Included[0].Path != "sample.go" || !strings.Contains(received.Messages[0].Content, "func Run()") {
		t.Fatalf("manifest does not describe generated prompt context: %+v", manifest)
	}
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifestJSON), "func Run()") {
		t.Fatalf("manifest leaked source text: %s", manifestJSON)
	}
	profile := service.EffectiveModel()
	if profile.Model != "coder-override" || profile.Profile != "coder" || profile.Timeout != "30s" {
		t.Fatalf("unexpected effective profile: %+v", profile)
	}
}

func TestRemoteProviderRequiresConfirmation(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{LLM: config.LLMConfig{BaseURL: "https://example.com"}}, manager)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RequireRemoteConfirmation(false); err == nil {
		t.Fatal("expected remote provider confirmation error")
	}
	if err := service.RequireRemoteConfirmation(true); err != nil {
		t.Fatal(err)
	}
	if !service.EffectiveModel().RemoteProvider {
		t.Fatal("effective model must disclose that the configured provider is remote")
	}
}

func TestAnalyzeProjectStoresStructuredReport(t *testing.T) {
	var received llm.ChatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Error(err)
			return
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "fixture-model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs a small daemon.","architecture":"One Go package.","components":["daemon"],"entry_points":["main.main"],"flows":["request to service"],"risks":[],"next_steps":["Add coverage"]}`}}}})
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{LLM: config.LLMConfig{BaseURL: server.URL, Model: "analysis-model"}}, manager)
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := service.AnalyzeProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Report.Status != project.ProjectAnalysisStatusFresh || analysis.Report.Model != "analysis-model" || analysis.Report.Profile != "analysis" || analysis.Report.Purpose != "Runs a small daemon." {
		t.Fatalf("project report = %+v", analysis.Report)
	}
	if len(received.Messages) != 2 || !strings.Contains(received.Messages[0].Content, "exactly one JSON object") {
		t.Fatalf("project analysis prompt = %+v", received.Messages)
	}
}

func TestGenerateCancellationStopsLLMRequest(t *testing.T) {
	started := make(chan struct{}, 1)
	providerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		started <- struct{}{}
		<-r.Context().Done()
		providerCanceled <- struct{}{}
	}))
	defer server.Close()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{LLM: config.LLMConfig{BaseURL: server.URL}, Retry: config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}}, manager)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := service.Generate(ctx, "improve Run", "sample.go", "Run", workflow.ScopeStrictSymbol, false)
		done <- err
	}()
	waitForTestSignal(t, started, "generation provider request")
	cancel()
	if err := waitForTestError(t, done, "canceled generation"); err == nil {
		t.Fatal("expected canceled generation error")
	} else if !errors.Is(err, context.Canceled) {
		t.Fatalf("generation error = %v, want context cancellation", err)
	}
	waitForTestSignal(t, providerCanceled, "generation provider cancellation")
}

func TestGenerateReturnsDeadlineExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{
		LLM:      config.LLMConfig{BaseURL: server.URL},
		Timeouts: config.TimeoutConfig{GenerationSeconds: 1},
		Retry:    config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Generate(context.Background(), "improve Run", "sample.go", "Run", workflow.ScopeStrictSymbol, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("generation error = %v, want deadline exceeded", err)
	}
}
