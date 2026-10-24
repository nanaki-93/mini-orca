package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/config"
)

type delegationFixture struct {
	server       *httptest.Server
	cards, calls atomic.Int32
	status       atomic.Value
	fail         atomic.Bool
	method       atomic.Value
	payload      atomic.Value
}

func newDelegationFixture(t *testing.T) *delegationFixture {
	t.Helper()
	f := &delegationFixture{}
	f.status.Store("TASK_STATE_INPUT_REQUIRED")
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/card" {
			f.cards.Add(1)
			fmt.Fprintf(w, `{"name":"Reviewer","description":"Advisory","version":"1","supportedInterfaces":[{"url":%q,"protocolBinding":"JSONRPC","protocolVersion":"1.0"}],"capabilities":{"streaming":true},"defaultInputModes":["text/plain"],"defaultOutputModes":["text/plain"],"skills":[{"id":"review","name":"Review","description":"Review source","tags":[]}]}`, f.server.URL+"/rpc")
			return
		}
		f.calls.Add(1)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		f.payload.Store(string(body))
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			return
		}
		f.method.Store(request.Method)
		if f.fail.Load() {
			w.WriteHeader(500)
			return
		}
		state := f.status.Load().(string)
		if request.Method == "CancelTask" {
			state = "TASK_STATE_CANCELED"
		}
		task := fmt.Sprintf(`{"id":"task","contextId":"context","status":{"state":%q,"message":{"messageId":"reply","role":"ROLE_AGENT","parts":[{"text":"Private advisory"}]}}}`, state)
		result := task
		if strings.Contains(request.Method, "Message") || request.Method == "SubscribeToTask" {
			result = `{"task":` + task + `}`
		}
		response := fmt.Sprintf(`{"jsonrpc":"2.0","id":%s,"result":%s}`, request.ID, result)
		if r.Header.Get("Accept") == "text/event-stream" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\n", response)
			return
		}
		io.WriteString(w, response)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func delegationService(t *testing.T, f *delegationFixture) (*Service, AgentRunQuery, DelegationRequest) {
	t.Helper()
	t.Setenv("MINI_ORCA_FIXTURE_REMOTE_TOKEN", "credential-one")
	s, _ := newSemanticAnalysisService(t, "http://127.0.0.1:1", 0)
	registry, err := newIntegrationRegistry(config.IntegrationsConfig{A2A: []config.RemoteAgent{{ID: "reviewer", CardURL: f.server.URL + "/card", Endpoint: f.server.URL + "/rpc", BearerTokenEnv: "MINI_ORCA_FIXTURE_REMOTE_TOKEN", Skills: []string{"review"}}}})
	if err != nil {
		t.Fatal(err)
	}
	s.integrations = registry
	index, err := s.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := s.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	query := AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	views, err := s.ListRemoteAgents(t.Context(), query)
	if err != nil || len(views) != 1 || views[0].State != "configured" || f.cards.Load() != 0 {
		t.Fatalf("passive connections=%+v %v", views, err)
	}
	control := IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Action: "discover", Configuration: views[0].Configuration, ConfirmConnect: true}
	ready, err := s.ControlRemoteAgent(t.Context(), "reviewer", control)
	if err != nil {
		t.Fatal(err)
	}
	request := DelegationRequest{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Remote: IntegrationUse{ID: "reviewer", Grant: ready.Grant, SchemaFingerprint: ready.Inventory.Fingerprint, ConfirmSourceSharing: true}, Kind: "review", Skill: "review", Question: "Review only this file", Sources: []ExternalSource{{Path: "main.go", Hash: file.ContentHash}}, ConfirmDelegate: true}
	return s, query, request
}

func TestRemoteConnectionRevocationAndCredentialChange(t *testing.T) {
	f := newDelegationFixture(t)
	s, query, request := delegationService(t, f)
	views, err := s.ListRemoteAgents(t.Context(), query)
	if err != nil {
		t.Fatal(err)
	}
	view := views[0]
	t.Setenv("MINI_ORCA_FIXTURE_REMOTE_TOKEN", "credential-two")
	if _, err := s.StartDelegation(t.Context(), request); err == nil || f.calls.Load() != 0 || f.cards.Load() != 1 {
		t.Fatal("changed credential dispatched", err)
	}
	t.Setenv("MINI_ORCA_FIXTURE_REMOTE_TOKEN", "credential-one")
	request.Remote.Grant = "wrong"
	if _, err := s.StartDelegation(t.Context(), request); err == nil || f.calls.Load() != 0 {
		t.Fatal("stale grant dispatched", err)
	}
	control := IntegrationControl{ProjectID: query.ProjectID, ProjectRevision: query.ProjectRevision, Configuration: view.Configuration, Action: "revoke", Grant: view.Grant}
	if _, err := s.ControlRemoteAgent(t.Context(), "reviewer", control); err != nil {
		t.Fatal(err)
	}
	request.Remote.Grant = view.Grant
	if _, err := s.StartDelegation(t.Context(), request); err == nil || f.calls.Load() != 0 || f.cards.Load() != 1 {
		t.Fatal("revoked grant dispatched", err)
	}
}
