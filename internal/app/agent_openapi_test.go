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
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app/agentruntime"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

const applicationAPISpec = `{"openapi":"3.0.3","info":{"title":"Fixture API","version":"1"},"paths":{"/items":{"post":{"operationId":"update","requestBody":{"required":true,"content":{"application/json":{"schema":{"type":"object","properties":{"id":{"type":"integer"}},"required":["id"],"additionalProperties":false}}}},"responses":{"204":{"description":"Updated"}}}}}}`

func installAPIFixture(t *testing.T, service *Service, endpoint string) (AgentRunQuery, *AgentAPI) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(service.manager.Root(), "api.json"), []byte(applicationAPISpec), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Reindex(); err != nil {
		t.Fatal(err)
	}
	registry, err := newIntegrationRegistry(config.IntegrationsConfig{OpenAPI: []config.OpenAPIService{{ID: "items", SpecPath: "api.json", Endpoint: endpoint, Operations: []config.IntegrationTool{{Name: "update", Effect: "mutate_external"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	service.integrations = registry
	index, err := service.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	views, err := service.ListAPIs(t.Context(), query)
	if err != nil || len(views) != 1 {
		t.Fatalf("list APIs: %+v %v", views, err)
	}
	view, err := service.ControlAPI(t.Context(), "items", IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "discover", Configuration: views[0].Configuration, ConfirmConnect: true})
	if err != nil {
		t.Fatal(err)
	}
	return query, view
}

func TestOpenAPIPlanToolsProposeWithoutSendingAndRequireExactApproval(t *testing.T) {
	var sends, turns atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sends.Add(1)
		body, _ := io.ReadAll(r.Body)
		if r.Method != "POST" || r.URL.Path != "/items" || string(body) != `{"id":7}` {
			t.Errorf("changed write: %s %s %s", r.Method, r.URL.Path, body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer remote.Close()
	name, hash, grant := "", "", ""
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), grant) {
			t.Error("API grant reached provider")
		}
		if turns.Add(1) == 1 {
			repositoryToolCall(w, name, `{"body":{"id":7}}`)
			return
		}
		if !strings.Contains(string(body), "approval_required") {
			t.Error("proposal state missing")
		}
		content, _ := json.Marshal(planInvestigationFixture(hash))
		json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{Message: llm.ChatMessage{Content: string(content)}}}})
	}))
	defer model.Close()
	service, _ := newSemanticAnalysisService(t, model.URL, 0)
	query, view := installAPIFixture(t, service, remote.URL)
	if sends.Load() != 0 {
		t.Fatal("discovery connected to API")
	}
	grant = view.Grant
	name = agentruntime.APITools(agentruntime.APIConnection{}, "items", *view.Inventory, nil, nil)[0].Name
	request := planInvestigationRequest(t, service)
	hash = request.Sources[0].Hash
	request.Integrations = []IntegrationUse{{ID: "items", Grant: view.Grant, SchemaFingerprint: view.Inventory.Fingerprint}}
	if _, err := service.InvestigatePlan(t.Context(), request); err == nil || turns.Load() != 0 {
		t.Fatal("source grant omitted")
	}
	request.Integrations[0].ConfirmSourceSharing = true
	if _, err := service.InvestigatePlan(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	actions, err := service.ListAPIActions(t.Context(), query)
	if err != nil || len(actions) != 1 || !actions[0].CanApprove || sends.Load() != 0 {
		t.Fatalf("proposal: %+v %v sends=%d", actions, err, sends.Load())
	}
	action := actions[0]
	control := APIActionControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "approve", Version: action.Version, RequestHash: action.RequestHash}
	if _, err := service.ControlAPIAction(t.Context(), action.ID, control); err == nil || sends.Load() != 0 {
		t.Fatal("unconfirmed write dispatched")
	}
	control.Confirm = true
	control.RequestHash = strings.Repeat("0", 64)
	if _, err := service.ControlAPIAction(t.Context(), action.ID, control); err == nil || sends.Load() != 0 {
		t.Fatal("different request approved")
	}
	control.RequestHash = action.RequestHash
	completed, err := service.ControlAPIAction(t.Context(), action.ID, control)
	if err != nil || completed.State != "completed" || completed.HTTPStatus != 204 || sends.Load() != 1 {
		t.Fatalf("write receipt: %+v %v sends=%d", completed, err, sends.Load())
	}
	if _, err := service.ControlAPIAction(t.Context(), action.ID, control); err == nil || sends.Load() != 1 {
		t.Fatal("approval replayed")
	}
}

func TestOpenAPIInventoryIsIsolatedAndInvalidatesChangedContracts(t *testing.T) {
	service, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	query, view := installAPIFixture(t, service, "http://127.0.0.1:1")
	use := IntegrationUse{ID: "items", Grant: view.Grant, SchemaFingerprint: view.Inventory.Fingerprint, ConfirmSourceSharing: true}
	view.Config.Operations[0].Effect = "read_external"
	view.Inventory.Operations[0].InputSchema[0] = 'x'
	views, err := service.ListAPIs(t.Context(), query)
	if err != nil || !json.Valid(views[0].Inventory.Operations[0].InputSchema) || views[0].Config.Operations[0].Effect != "mutate_external" {
		t.Fatalf("inventory alias: %+v %v", views, err)
	}
	check := func(ctx context.Context) error { return context.Cause(ctx) }
	if _, _, err := service.admitAPI(t.Context(), service.manager.Root(), strings.Repeat("a", 64), use, check); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(service.manager.Root(), "api.json"), []byte(applicationAPISpec+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.admitAPI(t.Context(), service.manager.Root(), strings.Repeat("a", 64), use, check); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatalf("changed contract admitted: %v", err)
	}
	if _, err := service.ControlAPI(t.Context(), "items", IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "revoke", Configuration: view.Configuration, Grant: view.Grant}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.admitAPI(t.Context(), service.manager.Root(), strings.Repeat("a", 64), use, check); !errors.Is(err, ErrIntegrationConflict) {
		t.Fatal("revoked API admitted")
	}
}
