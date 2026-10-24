package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const integrationFixtureSchema = `{"type":"object","properties":{"query":{"type":"string","maxLength":128}},"required":["query"],"additionalProperties":false}`

func integrationFixtureResult(method string) any {
	switch method {
	case "initialize":
		return map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
	case "tools/list":
		return map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": "Read external docs", "inputSchema": json.RawMessage(integrationFixtureSchema)}}}
	default:
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": "EXTERNAL_ADVISORY_RESULT"}}}
	}
}

func installIntegration(t *testing.T, service *Service, server config.MCPServer) (AgentRunQuery, AgentIntegration) {
	t.Helper()
	registry, err := newIntegrationRegistry(config.IntegrationsConfig{MCP: []config.MCPServer{server}})
	if err != nil {
		t.Fatal(err)
	}
	service.integrations = registry
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	list, err := service.ListIntegrations(t.Context(), query)
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %+v %v", list, err)
	}
	return query, list[0]
}

func discoverIntegrationFixture(t *testing.T, service *Service, query AgentRunQuery, view AgentIntegration) *AgentIntegration {
	t.Helper()
	result, err := service.ControlIntegration(t.Context(), view.Config.ID, IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "discover", Configuration: view.Configuration, ConfirmConnect: true})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestMCPIntegrationsReachPlanAndDeclarationOnlyWithSourceGrant(t *testing.T) {
	for _, mode := range []string{"plan", "declaration"} {
		t.Run(mode, func(t *testing.T) {
			var remoteCalls, protocolCalls, modelCalls atomic.Int32
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				protocolCalls.Add(1)
				var request struct {
					ID     json.RawMessage `json:"id"`
					Method string          `json:"method"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if len(request.ID) == 0 {
					w.WriteHeader(202)
					return
				}
				if request.Method == "tools/call" {
					remoteCalls.Add(1)
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": integrationFixtureResult(request.Method)})
			}))
			defer remote.Close()
			var view *AgentIntegration
			hash := ""
			model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if strings.Contains(string(body), view.Grant) {
					t.Error("grant token entered model context")
				}
				if modelCalls.Add(1) == 1 {
					repositoryToolCall(w, view.Inventory.Tools[0].Name, `{"query":"admitted source question"}`)
					return
				}
				if !strings.Contains(string(body), "EXTERNAL_ADVISORY_RESULT") {
					t.Error("native result missing")
				}
				content := validDeclarationExplanation
				if mode == "plan" {
					data, _ := json.Marshal(planInvestigationFixture(hash))
					content = string(data)
				}
				json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: content}}}})
			}))
			defer model.Close()
			service, _ := newSemanticAnalysisService(t, model.URL, 0)
			query, configured := installIntegration(t, service, config.MCPServer{ID: "docs", Transport: "streamable_http", Endpoint: remote.URL, Tools: []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}})
			if protocolCalls.Load() != 0 {
				t.Fatal("passive listing connected")
			}
			view = discoverIntegrationFixture(t, service, query, configured)
			use := IntegrationUse{ID: "docs", Grant: view.Grant, SchemaFingerprint: view.Inventory.Fingerprint}
			hash = declarationRequest(t, service).BaseFileHash
			run := func() error {
				if mode == "plan" {
					request := planInvestigationRequest(t, service)
					request.Integrations = []IntegrationUse{use}
					_, err := service.InvestigatePlan(t.Context(), request)
					return err
				}
				request := declarationRequest(t, service)
				request.Integrations = []IntegrationUse{use}
				_, err := service.ExplainDeclaration(t.Context(), request)
				return err
			}
			if err := run(); err == nil || modelCalls.Load() != 0 || remoteCalls.Load() != 0 {
				t.Fatalf("unconfirmed dispatch: %v", err)
			}
			use.ConfirmSourceSharing = true
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if modelCalls.Load() != 2 || remoteCalls.Load() != 1 {
				t.Fatalf("model=%d remote=%d", modelCalls.Load(), remoteCalls.Load())
			}
			if _, err := service.revokeIntegration("docs", view.Grant); err != nil {
				t.Fatal(err)
			}
			if err := run(); !errors.Is(err, ErrIntegrationConflict) || modelCalls.Load() != 2 {
				t.Fatalf("revoked grant: %v", err)
			}
		})
	}
}

func TestMCPRevocationAndReindexCancelDiscoveryBeforePublication(t *testing.T) {
	for _, action := range []string{"revoke", "reindex", "shutdown"} {
		t.Run(action, func(t *testing.T) {
			started := make(chan struct{})
			closed := make(chan struct{})
			release := make(chan struct{})
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.Copy(io.Discard, r.Body)
				close(started)
				select {
				case <-r.Context().Done():
					close(closed)
				case <-release:
				}
			}))
			defer remote.Close()
			defer close(release)
			service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
			query, view := installIntegration(t, service, config.MCPServer{ID: "docs", Transport: "streamable_http", Endpoint: remote.URL, Tools: []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}})
			done := make(chan error, 1)
			go func() {
				_, err := service.ControlIntegration(t.Context(), "docs", IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "discover", Configuration: view.Configuration, ConfirmConnect: true})
				done <- err
			}()
			<-started
			switch action {
			case "revoke":
				if _, err := service.ControlIntegration(t.Context(), "docs", IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: view.Configuration, Action: "revoke"}); err != nil {
					t.Fatal(err)
				}
			case "reindex":
				if _, err := service.Reindex(); err != nil {
					t.Fatal(err)
				}
			case "shutdown":
				if err := service.ShutdownChangeWorkflows(t.Context()); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("canceled discovery published")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("discovery did not join")
			}
			select {
			case <-closed:
			case <-time.After(time.Second):
				t.Fatal("connection still active")
			}
			service.integrations.mu.Lock()
			defer service.integrations.mu.Unlock()
			if len(service.integrations.records) != 0 || len(service.integrations.active) != 0 {
				t.Fatal("revoked discovery retained state")
			}
		})
	}
}

func TestMCPStdioUsesPrivateDirectoryAndJoinsOwnedProcess(t *testing.T) {
	t.Setenv("MINI_ORCA_TEST_MCP_HELPER", "1")
	t.Setenv("UNADMITTED_TEST_SECRET", "must-not-inherit")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	query, view := installIntegration(t, service, config.MCPServer{ID: "docs", Transport: "stdio", Command: executable, Args: []string{"-test.run=^TestMCPHelperProcess$"}, Environment: map[string]string{"MINI_ORCA_TEST_MCP_HELPER": "MINI_ORCA_TEST_MCP_HELPER"}, Tools: []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}})
	result := discoverIntegrationFixture(t, service, query, view)
	if result.State != "ready" {
		t.Fatalf("discovery: %+v", result)
	}
	if len(service.integrations.active) != 0 {
		t.Fatal("process operation leaked")
	}
	connection, _, err := service.integrationConnection(view.Config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	reader, writer, cleanup, err := connection.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(writer, `{"jsonrpc":"2.0","id":1,"method":"test/directory"}`)
	var reply struct {
		Result string `json:"result"`
	}
	if err := json.NewDecoder(reader).Decode(&reply); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reply.Result, "mini-orca-mcp-") {
		t.Fatalf("cwd=%q", reply.Result)
	}
	cancel()
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(reply.Result); !os.IsNotExist(err) {
		t.Fatalf("private directory retained: %v", err)
	}
}

func TestMCPHelperProcess(t *testing.T) {
	if os.Getenv("MINI_ORCA_TEST_MCP_HELPER") != "1" {
		return
	}
	if os.Getenv("UNADMITTED_TEST_SECRET") != "" {
		os.Exit(3)
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(scanner.Bytes(), &request) != nil {
			os.Exit(4)
		}
		if len(request.ID) == 0 {
			continue
		}
		result := integrationFixtureResult(request.Method)
		if request.Method == "test/directory" {
			directory, err := os.Getwd()
			if err != nil {
				os.Exit(5)
			}
			result = directory
		}
		if err := json.NewEncoder(os.Stdout).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			os.Exit(6)
		}
	}
	os.Exit(0)
}

func TestMCPGrantFreshnessAndReturnedInventoryIsolation(t *testing.T) {
	t.Setenv("MCP_TEST_CREDENTIAL", "original")
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer original" {
			t.Error("credential mismatch")
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(202)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": integrationFixtureResult(request.Method)})
	}))
	defer remote.Close()
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	root := service.manager.Root()
	query, configured := installIntegration(t, service, config.MCPServer{ID: "docs", Transport: "streamable_http", Endpoint: remote.URL, BearerTokenEnv: "MCP_TEST_CREDENTIAL", Tools: []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}})
	ready := discoverIntegrationFixture(t, service, query, configured)
	use := IntegrationUse{ID: "docs", Grant: ready.Grant, SchemaFingerprint: ready.Inventory.Fingerprint, ConfirmSourceSharing: true}
	ready.Config.Tools[0].Effect = "execute"
	ready.Inventory.Tools[0].InputSchema[0] = 'x'
	ready.ExpiresAt = nil
	views, err := service.ListIntegrations(t.Context(), query)
	if err != nil || views[0].Config.Tools[0].Effect != "read_external" || !json.Valid(views[0].Inventory.Tools[0].InputSchema) || views[0].ExpiresAt == nil {
		t.Fatalf("inventory alias: %+v %v", views, err)
	}
	before := calls.Load()
	noSourceChange := func(ctx context.Context) error { return context.Cause(ctx) }
	if _, _, err := service.admitIntegration(t.Context(), root, use, noSourceChange); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_TEST_CREDENTIAL", "changed")
	if _, _, err := service.admitIntegration(t.Context(), root, use, noSourceChange); !errors.Is(err, ErrIntegrationUnavailable) {
		t.Fatalf("changed credentials: %v", err)
	}
	t.Setenv("MCP_TEST_CREDENTIAL", "original")
	service.integrations.mu.Lock()
	record := service.integrations.records["docs"]
	expired := time.Now().Add(-time.Second)
	record.view.ExpiresAt = &expired
	service.integrations.records["docs"] = record
	service.integrations.mu.Unlock()
	if _, _, err := service.admitIntegration(t.Context(), root, use, noSourceChange); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatalf("expired grant: %v", err)
	}
	views, err = service.ListIntegrations(t.Context(), query)
	if err != nil || views[0].State != "expired" || views[0].Grant != "" || calls.Load() != before {
		t.Fatalf("expiry connected or misreported: %+v %v", views, err)
	}
	revoked, err := service.ControlIntegration(t.Context(), "docs", IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "revoke", Configuration: views[0].Configuration})
	if err != nil || revoked.State != "configured" || calls.Load() != before {
		t.Fatalf("expired inventory could not be revoked passively: %+v %v", revoked, err)
	}
}
