package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestRemoteProviderRequiresConfirmation(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(scopedTestConfig("https://example.com/v1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	for _, scope := range []config.ModelScope{config.AnalyzeModelScope, config.BugModelScope, config.FunctionModelScope} {
		if err := service.RequireRemoteConfirmation(scope, false); err == nil {
			t.Fatalf("expected %s confirmation error", scope)
		}
		if err := service.RequireRemoteConfirmation(scope, true); err != nil {
			t.Fatal(err)
		}
	}
	if !service.EffectiveModel().RemoteProvider {
		t.Fatal("effective model must disclose that the configured provider is remote")
	}
}

func TestModelCatalogAndConfirmationAreScopeSpecific(t *testing.T) {
	manager, err := project.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", APIKey: "do-not-return", Model: "analyze-model", ReasoningEffort: "high"},
			Bug:      config.ModelProfileConfig{APIBaseURL: "http://127.0.0.1:11434/v1", Model: "bug-model", ReasoningEffort: "low"},
			Function: config.ModelProfileConfig{APIBaseURL: "https://function.example/v1", APIKey: "do-not-return", Model: "function-model", ReasoningEffort: "max"},
		},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}
	catalog := service.CurrentModelCatalog()
	if len(catalog.Scopes) != 3 {
		t.Fatalf("catalog top-level compatibility = %+v", catalog)
	}
	if catalog.Scopes["analyze"].ProviderOrigin != "https://analyze.example" || catalog.Scopes["analyze"].ReasoningEffort != "high" || catalog.Scopes["bug"].ReasoningEffort != "low" || catalog.Scopes["function"].ReasoningEffort != "max" || catalog.Scopes["bug"].RemoteProvider || !catalog.Scopes["function"].RemoteProvider {
		t.Fatalf("scope catalog = %+v", catalog.Scopes)
	}
	encoded, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "do-not-return") || strings.Contains(string(encoded), "/v1") {
		t.Fatalf("catalog leaked configured endpoint or key: %s", encoded)
	}
	if err := service.RequireRemoteConfirmation(config.BugModelScope, false); err != nil {
		t.Fatalf("local bug provider was gated: %v", err)
	}
	if err := service.RequireRemoteConfirmation(config.AnalyzeModelScope, false); err == nil {
		t.Fatal("remote analyze provider was not gated")
	}
	if err := service.RequireRemoteConfirmation(config.FunctionModelScope, false); err == nil {
		t.Fatal("remote function provider was not gated")
	}
}

func TestScopedModelLocalMixedAndRemoteConfirmationMatrixUsesNoProvider(t *testing.T) {
	tests := []struct {
		name   string
		scopes config.ModelScopesConfig
		remote map[string]bool
	}{
		{
			name: "all local",
			scopes: config.ModelScopesConfig{
				Analyze:  config.ModelProfileConfig{APIBaseURL: "http://127.0.0.1:11434/v1", Model: "local-analyze"},
				Bug:      config.ModelProfileConfig{APIBaseURL: "http://localhost:1234/v1", Model: "local-bug"},
				Function: config.ModelProfileConfig{APIBaseURL: "http://[::1]:8080/v1", Model: "local-function"},
			},
			remote: map[string]bool{"analyze": false, "bug": false, "function": false},
		},
		{
			name: "mixed cloud cloud local",
			scopes: config.ModelScopesConfig{
				Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", Model: "cloud-analyze"},
				Bug:      config.ModelProfileConfig{APIBaseURL: "https://bug.example/v1", Model: "cloud-bug"},
				Function: config.ModelProfileConfig{APIBaseURL: "http://127.0.0.1:11434/v1", Model: "local-function"},
			},
			remote: map[string]bool{"analyze": true, "bug": true, "function": false},
		},
		{
			name: "all remote",
			scopes: config.ModelScopesConfig{
				Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", Model: "cloud-analyze"},
				Bug:      config.ModelProfileConfig{APIBaseURL: "https://bug.example/v1", Model: "cloud-bug"},
				Function: config.ModelProfileConfig{APIBaseURL: "https://function.example/v1", Model: "cloud-function"},
			},
			remote: map[string]bool{"analyze": true, "bug": true, "function": true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manager, err := project.NewManager(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			service, err := New(&config.Config{ModelScopes: test.scopes}, manager)
			if err != nil {
				t.Fatal(err)
			}
			catalog := service.CurrentModelCatalog()
			for _, scope := range []config.ModelScope{config.AnalyzeModelScope, config.BugModelScope, config.FunctionModelScope} {
				profile := catalog.Scopes[string(scope)]
				if profile.RemoteProvider != test.remote[string(scope)] {
					t.Fatalf("%s remote metadata = %+v", scope, profile)
				}
				err := service.RequireRemoteConfirmation(scope, false)
				if test.remote[string(scope)] && err == nil || !test.remote[string(scope)] && err != nil {
					t.Fatalf("%s confirmation error = %v", scope, err)
				}
			}
		})
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
	cfg := scopedTestConfig(server.URL)
	cfg.ModelScopes.Analyze.Model = "analysis-model"
	service, err := New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := service.AnalyzeProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Report.Status != project.ProjectAnalysisStatusFresh || analysis.Report.Model != "fixture-model" || analysis.Report.ConfiguredModel != "analysis-model" || analysis.Report.Scope != "analyze" || analysis.Report.Profile != "analyze" || analysis.Report.Purpose != "Runs a small daemon." {
		t.Fatalf("project report = %+v", analysis.Report)
	}
	if len(received.Messages) != 2 || !strings.Contains(received.Messages[0].Content, "exactly one JSON object") {
		t.Fatalf("project analysis prompt = %+v", received.Messages)
	}
}

func TestAnalyzeProjectKeepsInventoryWhenProjectSummaryTimesOut(t *testing.T) {
	providerCanceled := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		providerCanceled <- struct{}{}
	}))
	defer server.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(scopedTestConfig(server.URL), manager)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	analysis, err := service.AnalyzeProject(ctx, root)
	if err != nil {
		t.Fatalf("AnalyzeProject() error = %v", err)
	}
	if analysis.FileCount != 1 || analysis.Report.Status != project.ProjectAnalysisStatusFailed || analysis.Report.Failure == "" {
		t.Fatalf("timed-out analysis = %+v", analysis)
	}
	select {
	case <-providerCanceled:
	case <-time.After(time.Second):
		t.Fatal("project summary request was not canceled")
	}
}

func TestScopedModelRuntimesRouteOnlyAssignedOperations(t *testing.T) {
	var analyzeCalls, bugCalls, functionCalls int
	var analyzeRequest, bugRequest, functionRequest llm.ChatRequest
	analyzeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		analyzeCalls++
		if err := json.NewDecoder(r.Body).Decode(&analyzeRequest); err != nil {
			t.Fatal(err)
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "analyze-returned", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Understands the fixture.","architecture":"One package.","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[]}`}}}})
	}))
	defer analyzeServer.Close()
	bugServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bugCalls++
		if err := json.NewDecoder(r.Body).Decode(&bugRequest); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(bugRequest.Messages[0].Content, "## Immutable session scope") {
			t.Fatal("semantic analysis received a function request")
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "bug-returned", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: validSemanticAnalysis}}}})
	}))
	defer bugServer.Close()
	functionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		functionCalls++
		if err := json.NewDecoder(r.Body).Decode(&functionRequest); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(functionRequest.Messages[0].Content, "## Immutable session scope") || strings.Contains(functionRequest.Messages[0].Content, "Use the function_skill skill.") {
			t.Fatal("function proposal did not receive the direct scoped request")
		}
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "function-returned", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"version":"v1","declaration":"func Run() {}","explanation":"Keeps the declaration focused."}`}}}})
	}))
	defer functionServer.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: analyzeServer.URL + "/v1", Model: "analyze-model", ReasoningEffort: "high"},
			Bug:      config.ModelProfileConfig{APIBaseURL: bugServer.URL + "/v1", Model: "bug-model", ReasoningEffort: "medium"},
			Function: config.ModelProfileConfig{APIBaseURL: functionServer.URL + "/v1", Model: "function-model", ReasoningEffort: "low"},
		},
		Retry: config.RetryConfig{MaxRetries: 0, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}

	analysis, err := service.AnalyzeProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if analysis.Report.Scope != "analyze" || analysis.Report.Model != "analyze-returned" || analysis.Report.ConfiguredModel != "analyze-model" || analysis.Report.ProviderOrigin != analyzeServer.URL || analysis.Report.ReasoningEffort != "high" {
		t.Fatalf("analysis provenance = %+v", analysis.Report)
	}
	if err := manager.Set(root, analysis); err != nil {
		t.Fatal(err)
	}
	fileAnalysis, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if fileAnalysis.Scope != "bug" || fileAnalysis.ConfiguredModel != "bug-model" || fileAnalysis.Model != "bug-returned" || fileAnalysis.ProviderOrigin != bugServer.URL || fileAnalysis.ReasoningEffort != "medium" {
		t.Fatalf("file analysis provenance = %+v", fileAnalysis)
	}
	session := openFixtureChatSession(t, service, project.DeclarationEditReplaceSymbol, "Run")
	proposal, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "Clarify it."})
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Draft.EffectiveModel.Scope != "function" || proposal.Draft.EffectiveModel.Model != "function-model" || proposal.Draft.EffectiveModel.ProviderOrigin != functionServer.URL || proposal.Draft.EffectiveModel.ReasoningEffort != "low" {
		t.Fatalf("draft provenance = %+v", proposal.Draft.EffectiveModel)
	}
	if analyzeCalls != 1 || bugCalls != 1 || functionCalls != 1 {
		t.Fatalf("scope calls = analyze:%d bug:%d function:%d", analyzeCalls, bugCalls, functionCalls)
	}
	if analyzeRequest.ReasoningEffort != "high" || bugRequest.ReasoningEffort != "medium" || functionRequest.ReasoningEffort != "low" {
		t.Fatalf("scope reasoning efforts = analyze:%q bug:%q function:%q", analyzeRequest.ReasoningEffort, bugRequest.ReasoningEffort, functionRequest.ReasoningEffort)
	}
}

func TestScopedModelEndToEndTaskFlowKeepsNonPromptOperationsModelFree(t *testing.T) {
	var analyzeCalls, bugCalls, functionCalls int
	analyzeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		analyzeCalls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "analyze-response", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Fixture.","architecture":"One package.","components":[],"entry_points":[],"flows":[],"risks":[],"next_steps":[]}`}}}})
	}))
	defer analyzeServer.Close()
	bugServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bugCalls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "bug-response", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs one boolean operation.","responsibilities":["Return its current state."],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"The operation never succeeds.","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","target_signature":"untrusted","acceptance_criteria":["Run returns true."],"non_goals":["Do not edit another declaration."],"go_test_candidate":{"name":"TestRun","content":"package fixture\n\nimport \"testing\"\n\nfunc TestRun(t *testing.T) { if !Run() { t.Fatal(\"expected true\") } }\n"}}}],"suggestions":[],"symbol_explanations":{"Run":"Returns the current boolean."}}`}}}})
	}))
	defer bugServer.Close()
	functionServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		functionCalls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Model: "function-response", Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"version":"v1","declaration":"func Run() bool { return true }","explanation":"Satisfies the reviewed task."}`}}}})
	}))
	defer functionServer.Close()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.22\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package fixture\n\nfunc Run() bool { return false }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := New(&config.Config{
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: analyzeServer.URL + "/v1", Model: "analyze-model"},
			Bug:      config.ModelProfileConfig{APIBaseURL: bugServer.URL + "/v1", Model: "bug-model"},
			Function: config.ModelProfileConfig{APIBaseURL: functionServer.URL + "/v1", Model: "function-model", ContextMaxTokens: intPointer(400)},
		},
		Retry: config.RetryConfig{MaxRetries: 0, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := service.AnalyzeProject(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, analysis); err != nil {
		t.Fatal(err)
	}
	fileAnalysis, err := service.AnalyzeFile(context.Background(), "main.go", false, false)
	if err != nil || len(fileAnalysis.Risks) != 1 || fileAnalysis.Risks[0].TaskSpec == nil {
		t.Fatalf("bug task analysis = %+v, err = %v", fileAnalysis, err)
	}
	task := fileAnalysis.Risks[0].TaskSpec
	bugManifest, err := service.AnalysisContextManifest("main.go")
	if err != nil || bugManifest.Scope != "bug" || bugManifest.Model != "bug-model" || bugManifest.ProviderOrigin != bugServer.URL {
		t.Fatalf("bug context manifest = %+v, err = %v", bugManifest, err)
	}
	functionManifest, err := service.ContextManifest("main.go")
	if err != nil || functionManifest.Scope != "function" || functionManifest.Model != "function-model" || functionManifest.ProviderOrigin != functionServer.URL {
		t.Fatalf("function context manifest = %+v, err = %v", functionManifest, err)
	}
	indexedFile, err := manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.OpenChatSession(ChatSessionCreateRequest{ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: indexedFile.ContentHash, OpenPath: "main.go", Mode: project.DeclarationEditReplaceSymbol, TargetSymbol: "Run", TaskSpec: task})
	if err != nil {
		t.Fatal(err)
	}
	proposal, err := service.SendChatSessionMessage(context.Background(), ChatSessionMessageRequest{SessionID: session.ID, Message: "Implement the reviewed task."})
	if err != nil || proposal.Draft.TaskSpec == nil || proposal.Draft.EffectiveModel.Scope != string(config.FunctionModelScope) {
		t.Fatalf("function proposal = %+v, err = %v", proposal, err)
	}
	validated, err := service.ValidateDraft(proposal.Draft.ID, proposal.Draft.Revision)
	if err != nil || validated.State != DraftValid {
		t.Fatalf("validated draft = %+v, err = %v", validated, err)
	}
	checks, err := service.CheckDraft(context.Background(), DraftCheckRequest{ID: validated.ID, ExpectedRevision: validated.Revision, ExpectedHash: validated.Hash})
	if err != nil || !checks.Applicable || len(taskChecks(checks.Checks)) != 2 {
		t.Fatalf("task checks = %+v, err = %v", checks, err)
	}
	applied, err := service.ApplyDraft(context.Background(), ApplyRequest{DraftID: validated.ID, DraftRevision: validated.Revision, DraftHash: validated.Hash, ProjectID: analysis.ProjectID, ProjectRevision: analysis.ProjectRevision, BaseFileHash: indexedFile.ContentHash, Confirm: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UndoDraft(context.Background(), UndoRequest{ProjectID: analysis.ProjectID, ProjectRevision: applied.ProjectRevision, PostApplyHash: applied.PostApplyHash, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	current, err := manager.Analysis()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ScanGoProject(context.Background(), current.ProjectRevision); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	if analyzeCalls != 1 || bugCalls != 1 || functionCalls != 1 {
		t.Fatalf("all model calls must stay on their single prompt operation: analyze:%d bug:%d function:%d", analyzeCalls, bugCalls, functionCalls)
	}
}

func intPointer(value int) *int { return &value }

func scopedTestConfig(apiBaseURL string) *config.Config {
	cfg := config.Default()
	cfg.ModelScopes.Analyze.APIBaseURL = apiBaseURL
	cfg.ModelScopes.Bug.APIBaseURL = apiBaseURL
	cfg.ModelScopes.Function.APIBaseURL = apiBaseURL
	return cfg
}
