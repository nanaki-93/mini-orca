package handlers

import (
	"bytes"
	"context"
	"encoding/json"
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
)

func TestProjectRestoreActivatesStoredAnalysisWithoutModelAccess(t *testing.T) {
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	if _, err := project.NewAnalyzerWithProfile(nil, "", "analysis").Analyze(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	service, err := app.New(scopedHandlerConfig("http://127.0.0.1:1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewProjectHandler(manager, service)
	body, err := json.Marshal(map[string]string{"project_path": root})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()

	handler.Restore(response, httptest.NewRequest(http.MethodPost, "/api/projects/restore", bytes.NewReader(body)))

	if response.Code != http.StatusOK {
		t.Fatalf("restore status = %d: %s", response.Code, response.Body.String())
	}
	if _, err := manager.Index(); err != nil {
		t.Fatalf("restored index: %v", err)
	}
}

func TestProjectIndexAndSymbolAPIs(t *testing.T) {
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\n\nfunc Run(name string) string { return name }\n")
	writeProjectHandlerFixture(t, root, ".env", "TOKEN=must-not-expose")
	writeProjectHandlerFixture(t, root, "asset.bin", string([]byte{0, 1, 2}))
	handler := newIndexedProjectHandler(t, root)

	indexResponse := httptest.NewRecorder()
	handler.Index(indexResponse, httptest.NewRequest(http.MethodGet, "/api/projects/current/index", nil))
	if indexResponse.Code != http.StatusOK {
		t.Fatalf("index status = %d: %s", indexResponse.Code, indexResponse.Body.String())
	}
	var index project.ProjectIndex
	if err := json.NewDecoder(indexResponse.Body).Decode(&index); err != nil {
		t.Fatal(err)
	}
	if len(index.Files) != 2 || index.Files[0].Path != "asset.bin" || index.Files[1].Path != "main.go" {
		t.Fatalf("unsafe or unexpected index paths: %+v", index.Files)
	}

	symbolsResponse := httptest.NewRecorder()
	handler.Symbols(symbolsResponse, httptest.NewRequest(http.MethodGet, "/api/projects/current/files/symbols?path=main.go", nil))
	if symbolsResponse.Code != http.StatusOK {
		t.Fatalf("symbols status = %d: %s", symbolsResponse.Code, symbolsResponse.Body.String())
	}
	var symbols struct {
		Path    string               `json:"path"`
		Symbols []project.SymbolInfo `json:"symbols"`
	}
	if err := json.NewDecoder(symbolsResponse.Body).Decode(&symbols); err != nil {
		t.Fatal(err)
	}
	if symbols.Path != "main.go" || len(symbols.Symbols) != 1 || symbols.Symbols[0].Name != "Run" || !symbols.Symbols[0].AtomicTarget || symbols.Symbols[0].Confidence != "exact" {
		t.Fatalf("symbols response = %+v", symbols)
	}

	for _, test := range []struct {
		name string
		path string
		want int
	}{
		{name: "excluded", path: ".env", want: http.StatusForbidden},
		{name: "invalid traversal", path: "../outside.go", want: http.StatusBadRequest},
		{name: "missing", path: "missing.go", want: http.StatusBadRequest},
		{name: "binary", path: "asset.bin", want: http.StatusUnprocessableEntity},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.Symbols(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/files/symbols?path="+test.path, nil))
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.want, response.Body.String())
			}
			assertStructuredError(t, response)
		})
	}

	stale := httptest.NewRecorder()
	handler.Reindex(stale, httptest.NewRequest(http.MethodPost, "/api/projects/current/reindex", bytes.NewBufferString(`{"project_revision":"sha256:stale"}`)))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale reindex status = %d: %s", stale.Code, stale.Body.String())
	}
	assertStructuredError(t, stale)

	reindex := httptest.NewRecorder()
	handler.Reindex(reindex, httptest.NewRequest(http.MethodPost, "/api/projects/current/reindex", bytes.NewBufferString(`{}`)))
	if reindex.Code != http.StatusOK {
		t.Fatalf("reindex status = %d: %s", reindex.Code, reindex.Body.String())
	}
}

func TestFileAnalysisAPIsRequireRevisionAndExposeStates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[{"severity":"high","summary":"Run ignores errors.","task_spec":{"schema_version":"1","target_path":"main.go","target_symbol":"Run","acceptance_criteria":["Return errors."],"non_goals":[]}}],"suggestions":[],"symbol_explanations":{}}`}}}})
	}))
	defer server.Close()
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	cfg := scopedHandlerConfig(server.URL)
	cfg.Retry = config.RetryConfig{MaxRetries: 1, BackoffBase: 1, BackoffMax: 1}
	service, err := app.New(cfg, manager)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewProjectHandler(manager, service)
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}

	missing := httptest.NewRecorder()
	handler.FileAnalysis(missing, httptest.NewRequest(http.MethodGet, "/api/projects/current/files/analysis?path=main.go&project_revision="+index.ProjectRevision, nil))
	if missing.Code != http.StatusOK || !strings.Contains(missing.Body.String(), `"status":"missing"`) {
		t.Fatalf("missing analysis = %d %s", missing.Code, missing.Body.String())
	}
	stale := httptest.NewRecorder()
	handler.FileAnalysis(stale, httptest.NewRequest(http.MethodGet, "/api/projects/current/files/analysis?path=main.go&project_revision=sha256:stale", nil))
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale revision = %d %s", stale.Code, stale.Body.String())
	}

	body := bytes.NewBufferString(`{"path":"main.go","project_revision":"` + index.ProjectRevision + `"}`)
	analyzed := httptest.NewRecorder()
	handler.AnalyzeFile(analyzed, httptest.NewRequest(http.MethodPost, "/api/projects/current/files/analysis", body))
	if analyzed.Code != http.StatusOK || !strings.Contains(analyzed.Body.String(), `"status":"fresh"`) || !strings.Contains(analyzed.Body.String(), `"task_spec"`) || !strings.Contains(analyzed.Body.String(), `"target_symbol":"Run"`) {
		t.Fatalf("analyzed response = %d %s", analyzed.Code, analyzed.Body.String())
	}
	cleared := httptest.NewRecorder()
	handler.DeleteFileAnalysis(cleared, httptest.NewRequest(http.MethodDelete, "/api/projects/current/files/analysis?path=main.go&project_revision="+index.ProjectRevision, nil))
	if cleared.Code != http.StatusNoContent {
		t.Fatalf("delete response = %d %s", cleared.Code, cleared.Body.String())
	}
}

func TestPromptHandlersConfirmOnlyTheirOwnRemoteScope(t *testing.T) {
	bugCalls := 0
	bugProvider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		bugCalls++
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: `{"purpose":"Runs.","responsibilities":[],"dependencies":[],"side_effects":[],"risks":[],"suggestions":[],"symbol_explanations":{}}`}}}})
	}))
	defer bugProvider.Close()

	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "main.go", "package main\nfunc Run() {}\n")
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(&config.Config{
		ModelScopes: config.ModelScopesConfig{
			Analyze:  config.ModelProfileConfig{APIBaseURL: "https://analyze.example/v1", APIKey: "hidden", Model: "analyze"},
			Bug:      config.ModelProfileConfig{APIBaseURL: bugProvider.URL + "/v1", Model: "bug"},
			Function: config.ModelProfileConfig{APIBaseURL: "https://function.example/v1", APIKey: "hidden", Model: "function"},
		},
		Retry: config.RetryConfig{MaxRetries: 0, BackoffBase: 1, BackoffMax: 1},
	}, manager)
	if err != nil {
		t.Fatal(err)
	}
	projectHandler := NewProjectHandler(manager, service)
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}

	importResponse := httptest.NewRecorder()
	projectHandler.Import(importResponse, httptest.NewRequest(http.MethodPost, "/api/projects/import", bytes.NewBufferString(`{"project_path":"`+root+`"}`)))
	if importResponse.Code != http.StatusBadRequest || bugCalls != 0 || strings.Contains(importResponse.Body.String(), "hidden") {
		t.Fatalf("unconfirmed remote import = %d %s, bug calls %d", importResponse.Code, importResponse.Body.String(), bugCalls)
	}

	analysisResponse := httptest.NewRecorder()
	projectHandler.AnalyzeFile(analysisResponse, httptest.NewRequest(http.MethodPost, "/api/projects/current/files/analysis", bytes.NewBufferString(`{"path":"main.go","project_revision":"`+index.ProjectRevision+`"}`)))
	if analysisResponse.Code != http.StatusOK || bugCalls != 1 {
		t.Fatalf("local bug analysis = %d %s, bug calls %d", analysisResponse.Code, analysisResponse.Body.String(), bugCalls)
	}

	chatHandler := NewChatHandler(service)
	file, err := manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	open := httptest.NewRecorder()
	chatHandler.OpenSession(open, httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions", bytes.NewBufferString(`{"project_id":"`+index.ProjectID+`","project_revision":"`+index.ProjectRevision+`","base_file_hash":"`+file.ContentHash+`","open_path":"main.go","mode":"replace_symbol","target_symbol":"Run"}`)))
	if open.Code != http.StatusCreated {
		t.Fatalf("open session = %d: %s", open.Code, open.Body.String())
	}
	var session app.ChatSession
	if err := json.NewDecoder(open.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	message := httptest.NewRequest(http.MethodPost, "/api/projects/current/chat/sessions/"+session.ID+"/messages", bytes.NewBufferString(`{"message":"Change Run."}`))
	message.SetPathValue("sessionID", session.ID)
	functionResponse := httptest.NewRecorder()
	chatHandler.SendSessionMessage(functionResponse, message)
	if functionResponse.Code != http.StatusBadRequest || !strings.Contains(functionResponse.Body.String(), "remote function provider") || strings.Contains(functionResponse.Body.String(), "hidden") || bugCalls != 1 {
		t.Fatalf("unconfirmed remote function = %d %s, bug calls %d", functionResponse.Code, functionResponse.Body.String(), bugCalls)
	}

	reindexResponse := httptest.NewRecorder()
	projectHandler.Reindex(reindexResponse, httptest.NewRequest(http.MethodPost, "/api/projects/current/reindex", bytes.NewBufferString(`{"project_revision":"`+index.ProjectRevision+`"}`)))
	if reindexResponse.Code != http.StatusOK || bugCalls != 1 {
		t.Fatalf("model-free reindex = %d %s, bug calls %d", reindexResponse.Code, reindexResponse.Body.String(), bugCalls)
	}
}

func TestProjectWorkspaceAPIsAreRevisionGuardedAndSourceFree(t *testing.T) {
	root := t.TempDir()
	writeProjectHandlerFixture(t, root, "go.mod", "module fixture\n\ngo 1.22\n")
	writeProjectHandlerFixture(t, root, "main.go", "package fixture\nfunc Run() {}\n")
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(scopedHandlerConfig("http://127.0.0.1:1"), manager)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewProjectHandler(manager, service)
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}

	overview := httptest.NewRecorder()
	handler.Overview(overview, httptest.NewRequest(http.MethodGet, "/api/projects/current/overview?project_revision="+index.ProjectRevision, nil))
	if overview.Code != http.StatusOK || strings.Contains(overview.Body.String(), "package fixture") {
		t.Fatalf("overview = %d %s", overview.Code, overview.Body.String())
	}

	findings := httptest.NewRecorder()
	handler.Findings(findings, httptest.NewRequest(http.MethodGet, "/api/projects/current/findings?project_revision="+index.ProjectRevision+"&confidence=suggested", nil))
	if findings.Code != http.StatusOK || !strings.Contains(findings.Body.String(), `"findings":[]`) {
		t.Fatalf("findings = %d %s", findings.Code, findings.Body.String())
	}

	noScan := httptest.NewRecorder()
	handler.GoScanProgress(noScan, httptest.NewRequest(http.MethodGet, "/api/projects/current/scan?project_revision="+index.ProjectRevision, nil))
	if noScan.Code != http.StatusNoContent {
		t.Fatalf("scan without job = %d %s", noScan.Code, noScan.Body.String())
	}

	for _, call := range []struct {
		name string
		call func(*httptest.ResponseRecorder)
	}{
		{name: "overview", call: func(response *httptest.ResponseRecorder) {
			handler.Overview(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/overview?project_revision=stale", nil))
		}},
		{name: "findings", call: func(response *httptest.ResponseRecorder) {
			handler.Findings(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/findings?project_revision=stale", nil))
		}},
		{name: "scan start", call: func(response *httptest.ResponseRecorder) {
			handler.StartGoScan(response, httptest.NewRequest(http.MethodPost, "/api/projects/current/scan", bytes.NewBufferString(`{"project_revision":"stale"}`)))
		}},
		{name: "scan progress", call: func(response *httptest.ResponseRecorder) {
			handler.GoScanProgress(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/scan?project_revision=stale", nil))
		}},
		{name: "scan cancel", call: func(response *httptest.ResponseRecorder) {
			handler.CancelGoScan(response, httptest.NewRequest(http.MethodDelete, "/api/projects/current/scan?project_revision=stale", nil))
		}},
		{name: "triage", call: func(response *httptest.ResponseRecorder) {
			handler.UpdateFindingStatus(response, httptest.NewRequest(http.MethodPatch, "/api/projects/current/findings/finding:test", bytes.NewBufferString(`{"project_revision":"stale","status":"dismissed"}`)))
		}},
	} {
		t.Run(call.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			call.call(response)
			if response.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusConflict, response.Body.String())
			}
			assertStructuredError(t, response)
		})
	}
}

func TestProjectIndexAPIsRequireAnActiveProject(t *testing.T) {
	root := t.TempDir()
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewProjectHandler(manager, nil)
	for _, test := range []struct {
		name string
		call func(*httptest.ResponseRecorder)
	}{
		{name: "index", call: func(response *httptest.ResponseRecorder) {
			handler.Index(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/index", nil))
		}},
		{name: "symbols", call: func(response *httptest.ResponseRecorder) {
			handler.Symbols(response, httptest.NewRequest(http.MethodGet, "/api/projects/current/files/symbols?path=main.go", nil))
		}},
		{name: "reindex", call: func(response *httptest.ResponseRecorder) {
			handler.Reindex(response, httptest.NewRequest(http.MethodPost, "/api/projects/current/reindex", nil))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			test.call(response)
			if response.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
			}
			assertStructuredError(t, response)
		})
	}
}

func newIndexedProjectHandler(t *testing.T, root string) *ProjectHandler {
	t.Helper()
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	return NewProjectHandler(manager, nil)
}

func writeProjectHandlerFixture(t *testing.T, root, relative, content string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func assertStructuredError(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		Type        string `json:"type"`
		Message     string `json:"message"`
		UserMessage string `json:"user_message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Type == "" || body.Message == "" || body.UserMessage == "" {
		t.Fatalf("unstructured error = %+v", body)
	}
}
