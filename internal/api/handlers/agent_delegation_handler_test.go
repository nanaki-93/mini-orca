package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

func TestDelegationHandlerStrictConsentAndPrivateReads(t *testing.T) {
	var cards, calls atomic.Int32
	var endpoint string
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/card" {
			cards.Add(1)
			fmt.Fprintf(w, `{"name":"Review","description":"Fixture","version":"1","supportedInterfaces":[{"url":%q,"protocolBinding":"JSONRPC","protocolVersion":"1.0"}],"capabilities":{},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"review","name":"Review","description":"Review","tags":[]}]}`, endpoint)
			return
		}
		calls.Add(1)
		var request struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%s,"result":{"task":{"id":"task","contextId":"context","status":{"state":"TASK_STATE_COMPLETED","message":{"messageId":"reply","role":"ROLE_AGENT","parts":[{"text":"Private review"}]}}}}}`, request.ID)
	}))
	defer remote.Close()
	endpoint = remote.URL + "/rpc"
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.Integrations.A2A = []config.RemoteAgent{{ID: "review", CardURL: remote.URL + "/card", Endpoint: endpoint, Skills: []string{"review"}}}
	service, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAgentDelegationHandler(service)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	get := func(fn http.HandlerFunc, id string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
		r.SetPathValue("delegationID", id)
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	post := func(fn http.HandlerFunc, key, id string, value any) *httptest.ResponseRecorder {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		r.SetPathValue(key, id)
		w := httptest.NewRecorder()
		fn(w, r)
		return w
	}
	w := get(handler.Remotes, "")
	var configured []app.AgentRemote
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &configured) != nil || len(configured) != 1 || cards.Load() != 0 {
		t.Fatalf("passive list: %d %s", w.Code, w.Body.String())
	}
	control := app.IntegrationControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "discover", Configuration: configured[0].Configuration}
	if w := post(handler.ControlRemote, "integrationID", "review", control); w.Code != 400 || cards.Load() != 0 {
		t.Fatalf("consent: %d %s", w.Code, w.Body.String())
	}
	control.ConfirmConnect = true
	w = post(handler.ControlRemote, "integrationID", "review", control)
	var ready app.AgentRemote
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &ready) != nil || ready.Inventory == nil {
		t.Fatalf("discover: %d %s", w.Code, w.Body.String())
	}
	request := app.DelegationRequest{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Kind: "review", Skill: "review", Question: "Review this question", Remote: app.IntegrationUse{ID: "review", Grant: ready.Grant, SchemaFingerprint: ready.Inventory.Fingerprint, ConfirmSourceSharing: true}, ConfirmDelegate: true}
	w = post(handler.Start, "", "", request)
	var receipt app.AgentDelegation
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &receipt) != nil || receipt.State != "completed" || !receipt.Advisory {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	w = get(handler.List, "")
	if w.Code != 200 || strings.Contains(w.Body.String(), "Private review") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("private list", w.Body.String())
	}
	w = get(handler.Detail, receipt.ID)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Private review") || calls.Load() != 1 || cards.Load() != 2 {
		t.Fatal("detail dispatched", w.Body.String())
	}
	bad := app.DelegationControl{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Version: receipt.Version - 1, Action: "cancel", Confirm: true}
	if w := post(handler.Control, "delegationID", receipt.ID, bad); w.Code != 409 {
		t.Fatalf("stale control: %d %s", w.Code, w.Body.String())
	}
	for _, fn := range []http.HandlerFunc{handler.Remotes, handler.List, handler.Detail} {
		w := httptest.NewRecorder()
		fn(w, httptest.NewRequest("GET", "/?"+query.Encode()+"&unknown=1", nil))
		if w.Code != 400 {
			t.Fatalf("unknown query: %d", w.Code)
		}
	}
	for _, fn := range []http.HandlerFunc{handler.ControlRemote, handler.Start, handler.Control} {
		for _, body := range []string{`{"unknown":true}`, `{} {}`} {
			w := httptest.NewRecorder()
			fn(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
			if w.Code != 400 {
				t.Fatalf("strict body: %d %s", w.Code, w.Body.String())
			}
		}
	}
}
