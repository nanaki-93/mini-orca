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
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

const validDeclarationExplanation = `{"version":"v1","summary":"Run writes a fixed status line.","behavior":["formats and writes the status"],"inputs":[],"outputs":[],"side_effects":["writes standard output"],"error_behavior":["does not expose the write error"]}`

func TestExplainDeclarationReturnsBoundedTransientResultWithoutMutation(t *testing.T) {
	var prompt string
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request llm.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		prompt = request.Messages[0].Content
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisServiceWithHelper(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	before, _ := os.ReadFile(filepath.Join(root, "main.go"))

	result, err := service.ExplainDeclaration(context.Background(), DeclarationExplanationRequest{
		ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash,
		TargetPath: "main.go", TargetSymbol: "Run",
	})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if result.Anchor.Path != "main.go" || result.Anchor.Symbol != "Run" || result.Anchor.StartLine <= 0 || result.Summary == "" {
		t.Fatalf("explanation = %+v", result)
	}
	if result.ContextManifest.Scope != "function" || len(result.ContextManifest.Included) != 1 || result.ContextManifest.Included[0].Path != "main.go" {
		t.Fatalf("manifest = %+v", result.ContextManifest)
	}
	if calls.Load() != 1 || !strings.Contains(prompt, "func Run") || strings.Contains(prompt, "helper secret") {
		t.Fatalf("calls = %d, prompt = %q", calls.Load(), prompt)
	}
	if string(after) != string(before) || len(service.drafts.records) != 0 || len(service.chatSessions.records) != 0 {
		t.Fatalf("read-only explanation mutated workflow or source")
	}
}

func TestExplainDeclarationRequiresConsentAndRejectsIneligibleOrStaleTargetsWithoutProviderCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	service.runtimes.function.effective.RemoteProvider = true
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	base := DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"}

	if _, err := service.ExplainDeclaration(context.Background(), base); err == nil || !strings.Contains(err.Error(), "explicit confirmation") {
		t.Fatalf("consent error = %v", err)
	}
	missing := base
	missing.ConfirmRemoteProvider = true
	missing.TargetSymbol = "Missing"
	if _, err := service.ExplainDeclaration(context.Background(), missing); err == nil {
		t.Fatal("missing target accepted")
	}
	stale := base
	stale.ConfirmRemoteProvider = true
	stale.ProjectRevision = "sha256:stale"
	if _, err := service.ExplainDeclaration(context.Background(), stale); !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("stale error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() {}\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	duplicateIndex, err := service.Reindex()
	if err != nil {
		t.Fatal(err)
	}
	duplicateFile, _ := service.manager.IndexedFile("main.go")
	ambiguous := DeclarationExplanationRequest{ProjectID: duplicateIndex.ProjectID, ProjectRevision: duplicateIndex.ProjectRevision, BaseFileHash: duplicateFile.ContentHash, TargetPath: "main.go", TargetSymbol: "Run", ConfirmRemoteProvider: true}
	if _, err := service.ExplainDeclaration(context.Background(), ambiguous); err == nil {
		t.Fatal("ambiguous declaration target accepted")
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid requests made %d provider calls", calls.Load())
	}
}

func TestExplainDeclarationRejectsLatePublicationAfterSourceChanges(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	result := make(chan error, 1)
	go func() {
		_, err := service.ExplainDeclaration(context.Background(), DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"})
		result <- err
	}()
	<-started
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, project.ErrRevisionConflict) {
		t.Fatalf("late publication error = %v", err)
	}
}

func TestExplainDeclarationRejectsLatePublicationAfterPolicyChange(t *testing.T) {
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		close(started)
		<-release
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	before, _ := os.ReadFile(filepath.Join(root, "main.go"))
	result := make(chan error, 1)
	go func() {
		_, err := service.ExplainDeclaration(context.Background(), DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"})
		result <- err
	}()
	<-started
	if err := os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("main.go\n"), 0644); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-result; !errors.Is(err, project.ErrExcludedFile) {
		t.Fatalf("late policy error = %v", err)
	}
	after, _ := os.ReadFile(filepath.Join(root, "main.go"))
	if calls.Load() != 1 || string(before) != string(after) || len(service.drafts.records) != 0 || len(service.chatSessions.records) != 0 {
		t.Fatal("policy-invalidated explanation published or mutated workflow state")
	}
}

func TestExplainDeclarationRevalidatesPolicyAfterContextPreparationBeforeProviderCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, root := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	originalBuilder := service.buildDeclarationContext
	service.buildDeclarationContext = func(rootPath string, options project.FunctionContextOptions) (string, project.ContextManifest, error) {
		contextValue, manifest, err := originalBuilder(rootPath, options)
		if err == nil {
			err = os.WriteFile(filepath.Join(root, ".mini-orcaignore"), []byte("main.go\n"), 0644)
		}
		return contextValue, manifest, err
	}

	_, err := service.ExplainDeclaration(context.Background(), DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"})
	if !errors.Is(err, project.ErrExcludedFile) {
		t.Fatalf("post-context policy error = %v", err)
	}
	if calls.Load() != 0 || len(service.drafts.records) != 0 || len(service.chatSessions.records) != 0 {
		t.Fatal("policy change during context preparation reached provider or workflow state")
	}
}

func TestExplainDeclarationPreservesCancellationWithoutWorkflowMutation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validDeclarationExplanation}}}})
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.ExplainDeclaration(ctx, DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	if calls.Load() != 0 || len(service.drafts.records) != 0 || len(service.chatSessions.records) != 0 {
		t.Fatal("canceled explanation created workflow state")
	}
}

func TestExplainDeclarationReturnsProviderFailureWithoutWorkflowMutation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "provider detail that must not become workflow state", http.StatusBadGateway)
	}))
	defer server.Close()
	service, _ := newSemanticAnalysisService(t, server.URL, 0)
	index, _ := service.manager.Index()
	file, _ := service.manager.IndexedFile("main.go")
	_, err := service.ExplainDeclaration(context.Background(), DeclarationExplanationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, BaseFileHash: file.ContentHash, TargetPath: "main.go", TargetSymbol: "Run"})
	if err == nil {
		t.Fatal("provider failure was accepted")
	}
	if len(service.drafts.records) != 0 || len(service.chatSessions.records) != 0 {
		t.Fatal("provider failure created workflow state")
	}
}

func TestParseDeclarationExplanationResponseIsStrictAndBounded(t *testing.T) {
	if _, err := ParseDeclarationExplanationResponse(validDeclarationExplanation); err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{
		strings.Repeat("x", maxDeclarationExplanationBytes+1),
		validDeclarationExplanation + strings.Repeat(" ", maxDeclarationExplanationBytes),
		`{"version":"v1","summary":"x","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[],"unknown":true}`,
		`{"version":"v1","summary":"x","behavior":[],"inputs":[],"outputs":[],"side_effects":[]}`,
		`{"version":"v1","summary":"x","behavior":["","b"],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[]}`,
	} {
		if _, err := ParseDeclarationExplanationResponse(output); err == nil {
			t.Fatalf("accepted malformed output %s", output)
		}
	}
}

func TestParseDeclarationExplanationResponseOmitsInvalidOptionalInsight(t *testing.T) {
	base := `{"version":"v1","summary":"x","behavior":[],"inputs":[],"outputs":[],"side_effects":[],"error_behavior":[]`
	for _, insight := range []string{
		`"not an object"`,
		`{"mechanism":"m","why_it_matters_here":"w","unknown":true}`,
		`{"mechanism":"m"}`,
		`{"mechanism":"` + strings.Repeat("界", 1000) + `","why_it_matters_here":"w"}`,
		`null`,
		``,
	} {
		output := base + `}`
		if insight != "" {
			output = base + `,"engineering_insight":` + insight + `}`
		}
		parsed, err := ParseDeclarationExplanationResponse(output)
		if err != nil || parsed.EngineeringInsight != nil || parsed.Summary != "x" {
			t.Fatalf("optional insight %q parsed = %+v, %v", insight, parsed, err)
		}
	}
}
