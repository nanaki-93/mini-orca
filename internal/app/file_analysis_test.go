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
)

func TestAnalyzeFileCachesStructuredOneFileSummary(t *testing.T) {
	var prompt string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "fixture-model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs the selected command.","responsibilities":["dispatches work"],"dependencies":["fmt"],"side_effects":["writes stdout"],"risks":[{"severity":"low","summary":"No input validation."}],"suggestions":[{"title":"Validate input","summary":"Reject blank names.","target_symbol":"Run","action":"fix"}],"symbol_explanations":{"Run":"Dispatches the command."}}`}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	result, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != project.AnalysisStatusFresh || result.Purpose == "" || len(result.Symbols) != 1 || result.SymbolExplanations["Run"] == "" || result.Model != "fixture-model" {
		t.Fatalf("analysis = %+v", result)
	}
	if strings.Contains(prompt, "helper secret") || !strings.Contains(prompt, "func Run") || !strings.Contains(prompt, "TARGET_SOURCE (the only source content supplied)") {
		t.Fatalf("semantic prompt was not one-file scoped: %s", prompt)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "file-analysis")); err != nil {
		t.Fatalf("analysis cache not written: %v", err)
	}
	second, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || second.Status != project.AnalysisStatusFresh {
		t.Fatalf("cached analysis = %+v, %v", second, err)
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
		})
	}
}

func TestAnalyzeFileHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 1)
	_, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("analysis error = %v, want deadline exceeded", err)
	}
}

func newSemanticAnalysisService(t *testing.T, baseURL string, analysisSeconds int) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nimport \"fmt\"\n\nfunc Run() { fmt.Println(\"run\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "helper.go"), []byte("package main\n\nfunc Helper() { println(\"helper secret\") }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root, Summary: "A compact fixture project."}); err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{LLM: config.LLMConfig{BaseURL: baseURL, Model: "analysis-model"}, Timeouts: config.TimeoutConfig{AnalysisSeconds: analysisSeconds}, Retry: config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}}, manager)
	if err != nil {
		t.Fatal(err)
	}
	return service, root
}
