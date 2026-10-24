package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestIntegrationHandlerExplicitDiscoveryAndRevocation(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
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
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fixture", "version": "1"}}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{"name": "lookup", "description": "Read external docs", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}}}}
		default:
			t.Errorf("unexpected protocol method %s", request.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
	}))
	defer remote.Close()
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.Integrations.MCP = []config.MCPServer{{ID: "docs", Transport: "streamable_http", Endpoint: remote.URL, Tools: []config.IntegrationTool{{Name: "lookup", Effect: "read_external"}}}}
	service, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAgentIntegrationsHandler(service)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	var views []app.AgentIntegration
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &views) != nil || len(views) != 1 || calls.Load() != 0 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("passive list: %d %s", w.Code, w.Body.String())
	}
	control := app.IntegrationControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "discover", Configuration: views[0].Configuration}
	mutate := func(value any) *httptest.ResponseRecorder {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		r.SetPathValue("integrationID", "docs")
		w := httptest.NewRecorder()
		h.Control(w, r)
		return w
	}
	if w := mutate(control); w.Code != 400 || calls.Load() != 0 {
		t.Fatalf("unconfirmed: %d %s", w.Code, w.Body.String())
	}
	control.ConfirmConnect = true
	w = mutate(control)
	var ready app.AgentIntegration
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ready) != nil || ready.State != "ready" || ready.Inventory == nil {
		t.Fatalf("discover: %d %s", w.Code, w.Body.String())
	}
	control.Action, control.Grant = "revoke", "stale"
	if w := mutate(control); w.Code != 409 {
		t.Fatalf("stale grant: %d %s", w.Code, w.Body.String())
	}
	control.Grant = ready.Grant
	if w := mutate(control); w.Code != 200 || strings.Contains(w.Body.String(), ready.Grant) {
		t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
	}
	for _, body := range []string{`{"unknown":true}`, `{} {}`, `{}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.SetPathValue("integrationID", "docs")
		w := httptest.NewRecorder()
		h.Control(w, r)
		if w.Code < 400 {
			t.Fatalf("invalid body accepted: %s", body)
		}
	}
	for _, extra := range []string{"&unknown=1", "&project_id=duplicate"} {
		w := httptest.NewRecorder()
		h.List(w, httptest.NewRequest("GET", "/?"+query.Encode()+extra, nil))
		if w.Code != 400 {
			t.Fatalf("invalid query: %d", w.Code)
		}
	}
}
