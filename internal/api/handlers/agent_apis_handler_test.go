package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestAPIInventoryHandlerLocalDiscoveryAndStrictControls(t *testing.T) {
	var calls atomic.Int32
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); w.WriteHeader(204) }))
	defer remote.Close()
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	spec := `{"openapi":"3.0.3","info":{"title":"Local fixture","version":"1"},"paths":{"/items":{"post":{"operationId":"update","responses":{"204":{"description":"Done"}}}}}}`
	if err := os.WriteFile(filepath.Join(fixture.manager.Root(), "api.json"), []byte(spec), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Reindex(); err != nil {
		t.Fatal(err)
	}
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.Integrations.OpenAPI = []config.OpenAPIService{{ID: "items", SpecPath: "api.json", Endpoint: remote.URL, Operations: []config.IntegrationTool{{Name: "update", Effect: "mutate_external"}}}}
	s, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAgentAPIsHandler(s)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	var views []app.AgentAPI
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &views) != nil || len(views) != 1 || views[0].State != "configured" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("API list: %d %s", w.Code, w.Body.String())
	}
	control := app.IntegrationControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "discover", Configuration: views[0].Configuration}
	send := func(value any) *httptest.ResponseRecorder {
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.SetPathValue("integrationID", "items")
		w := httptest.NewRecorder()
		h.Control(w, r)
		return w
	}
	if w := send(control); w.Code != 400 {
		t.Fatalf("missing discovery consent: %d %s", w.Code, w.Body.String())
	}
	control.ConfirmConnect = true
	w = send(control)
	var ready app.AgentAPI
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ready) != nil || ready.Inventory == nil || ready.State != "ready" || calls.Load() != 0 {
		t.Fatalf("local discovery: %d %s calls=%d", w.Code, w.Body.String(), calls.Load())
	}
	control.Action, control.Grant = "revoke", "changed"
	if w := send(control); w.Code != 409 {
		t.Fatalf("stale revoke: %d %s", w.Code, w.Body.String())
	}
	control.Grant = ready.Grant
	if w := send(control); w.Code != 200 {
		t.Fatalf("revoke: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.Actions(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	if w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatalf("actions: %d %s", w.Code, w.Body.String())
	}
	for _, handler := range []http.HandlerFunc{h.List, h.Actions} {
		for _, extra := range []string{"&unknown=1", "&project_id=duplicate"} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest("GET", "/?"+query.Encode()+extra, nil))
			if w.Code != 400 {
				t.Fatalf("invalid query accepted: %d", w.Code)
			}
		}
	}
	for _, handler := range []http.HandlerFunc{h.Control, h.ControlAction} {
		for _, body := range []string{`{"unknown":true}`, `{} {}`, `{}`} {
			w := httptest.NewRecorder()
			handler(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
			if w.Code < 400 {
				t.Fatalf("invalid body accepted: %s", body)
			}
		}
	}
	body, _ := json.Marshal(app.APIActionControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "approve", Confirm: true, Version: 1, RequestHash: strings.Repeat("a", 64)})
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.SetPathValue("actionID", "absent")
	w = httptest.NewRecorder()
	h.ControlAction(w, r)
	if w.Code != 404 || calls.Load() != 0 {
		t.Fatalf("unknown action dispatched: %d %s", w.Code, w.Body.String())
	}
}
