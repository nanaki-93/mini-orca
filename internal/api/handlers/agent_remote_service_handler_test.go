package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestRemoteServiceHandlerRequiresExplicitCurrentGrant(t *testing.T) {
	t.Setenv("INBOUND_FIXTURE_TOKEN", strings.Repeat("p", 32))
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.Integrations.A2AServer = config.AgentServer{Enabled: true, Principals: []config.AgentPrincipal{{ID: "reviewer", BearerTokenEnv: "INBOUND_FIXTURE_TOKEN", ProjectID: index.ProjectID, Capabilities: []string{"analysis"}}}}
	service, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := service.NewRemoteService("http://127.0.0.1:9091/a2a")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close(context.Background())
	handler := NewAgentRemoteServiceHandler(service)
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	response := httptest.NewRecorder()
	handler.List(response, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	var views []app.RemoteServicePrincipal
	if err := json.Unmarshal(response.Body.Bytes(), &views); err != nil || response.Code != 200 || len(views) != 1 {
		t.Fatal("inventory", response.Body.String(), err)
	}
	if strings.Contains(response.Body.String(), strings.Repeat("p", 32)) || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private credential or cacheable grant")
	}
	post := func(data []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		r.SetPathValue("principalID", "reviewer")
		w := httptest.NewRecorder()
		handler.Control(w, r)
		return w
	}
	control := app.RemoteServiceControl{IntegrationControl: app.IntegrationControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Configuration: views[0].Configuration, Action: "grant", ConfirmConnect: true}}
	data, _ := json.Marshal(control)
	if w := post(data); w.Code != http.StatusBadRequest {
		t.Fatal("missing sharing consent", w.Code, w.Body.String())
	}
	control.ConfirmSourceSharing = true
	data, _ = json.Marshal(control)
	for _, invalid := range [][]byte{append(append([]byte{}, data...), []byte(` {}`)...), []byte(strings.TrimSuffix(string(data), "}") + `,"apply":true}`)} {
		if w := post(invalid); w.Code != http.StatusBadRequest {
			t.Fatal("ambiguous request accepted", w.Code)
		}
	}
	w := post(data)
	var ready app.RemoteServicePrincipal
	if err := json.Unmarshal(w.Body.Bytes(), &ready); err != nil || w.Code != 200 || ready.Grant == "" || ready.State != "authorized" {
		t.Fatal("grant", w.Code, w.Body.String(), err)
	}
	control.Action, control.Grant = "revoke", "stale"
	data, _ = json.Marshal(control)
	if w := post(data); w.Code != http.StatusConflict {
		t.Fatal("stale revoke", w.Code, w.Body.String())
	}
	control.Grant = ready.Grant
	data, _ = json.Marshal(control)
	if w := post(data); w.Code != 200 || strings.Contains(w.Body.String(), ready.Grant) {
		t.Fatal("revoke", w.Code, w.Body.String())
	}
}
