package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
	"github.com/nanaki-93/mini-orca/v2/internal/workflow"
)

func TestSendMessageReportsCanceledGeneration(t *testing.T) {
	started := make(chan struct{})
	providerCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(providerCanceled)
	}))
	defer server.Close()

	handler, body := newCancellationTestHandler(t, server.URL, 0)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/chat/message", bytes.NewReader(body)).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		handler.SendMessage(response, req)
		close(done)
	}()
	<-started
	cancel()
	<-done
	<-providerCanceled

	if response.Code != http.StatusRequestTimeout {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusRequestTimeout, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "generation canceled") {
		t.Fatalf("response does not identify cancellation: %s", response.Body.String())
	}
}

func TestSendMessageReportsGenerationTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	defer server.Close()

	handler, body := newCancellationTestHandler(t, server.URL, 1)
	req := httptest.NewRequest(http.MethodPost, "/api/chat/message", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.SendMessage(response, req)

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusGatewayTimeout, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "generation timed out") {
		t.Fatalf("response does not identify timeout: %s", response.Body.String())
	}
}

func TestSendMessageRejectsStaleProjectState(t *testing.T) {
	handler, body := newCancellationTestHandler(t, "http://127.0.0.1:1", 1)
	var request ChatRequest
	if err := json.Unmarshal(body, &request); err != nil {
		t.Fatal(err)
	}
	request.BaseFileHash = "sha256:stale"
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.SendMessage(response, httptest.NewRequest(http.MethodPost, "/api/chat/message", bytes.NewReader(body)))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
	}
}

func TestSendMessageReturnsStructuredGenerationPreview(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "fixture-model", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"version":"v1","target_path":"sample.go","target_symbol":"Run","scope_mode":"strict_symbol","candidate_content":"package sample\nfunc Run() {}"}`}}}})
	}))
	defer server.Close()

	handler, body := newCancellationTestHandler(t, server.URL, 0)
	response := httptest.NewRecorder()
	handler.SendMessage(response, httptest.NewRequest(http.MethodPost, "/api/chat/message", bytes.NewReader(body)))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var preview app.GenerationPreview
	if err := json.NewDecoder(response.Body).Decode(&preview); err != nil {
		t.Fatal(err)
	}
	if preview.GenerationID == "" || preview.TargetPath != "sample.go" || preview.TargetSymbol != "Run" || preview.ScopeMode != workflow.ScopeStrictSymbol || preview.CandidateHash == "" || preview.BaseFileHash == "" {
		t.Fatalf("preview = %+v", preview)
	}
}

func newCancellationTestHandler(t *testing.T, baseURL string, generationSeconds int) (*ChatHandler, []byte) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{}); err != nil {
		t.Fatal(err)
	}
	analysis, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	file, err := project.GetFileInfo(root, "sample.go")
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(&config.Config{
		LLM:      config.LLMConfig{BaseURL: baseURL},
		Timeouts: config.TimeoutConfig{GenerationSeconds: generationSeconds},
		Retry:    config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(ChatRequest{
		Message: "improve Run", FilePath: "sample.go", TargetSymbol: "Run",
		ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: file.ContentHash,
	})
	if err != nil {
		t.Fatal(err)
	}
	return NewChatHandler(service), body
}

func TestContextErrorResponseIsStructuredJSON(t *testing.T) {
	response := httptest.NewRecorder()
	writeContextError(response, "project import", context.DeadlineExceeded)
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Result().Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Message != "project import timed out" {
		t.Fatalf("message = %q", body.Message)
	}
}

func TestContextPreviewUsesOneFileManifestForAnalysis(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte("package sample\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(&config.Config{LLM: config.LLMConfig{BaseURL: "http://127.0.0.1:1"}}, manager)
	if err != nil {
		t.Fatal(err)
	}

	response := httptest.NewRecorder()
	NewContextHandler(service).Preview(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/context?path=sample.go&action=analyze_file", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	var manifest project.ContextManifest
	if err := json.NewDecoder(response.Body).Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Included) != 1 || manifest.Included[0].Path != "sample.go" || manifest.ByteLimit != 64*1024 {
		t.Fatalf("analysis context manifest = %+v", manifest)
	}
}
