package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestConnectionRoutesPreserveScopeAndExactControlFields(t *testing.T) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, WorktreePath: "/tmp/task space"}
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("worktree_path") != scope.WorktreePath || r.URL.Query().Get("project_id") != "p" || r.URL.Query().Get("project_revision") != "r" {
			t.Errorf("lost checkout query: %s", r.URL)
		}
		entry := AgentConnection{Config: ConnectionConfig{ID: "docs", ProjectID: "p", BearerTokenEnv: "DOCS_TOKEN"}, Configuration: "config", State: "configured"}
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]AgentConnection{entry})
			return
		}
		var request map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		_, sharing := request["confirm_source_sharing"]
		_, provider := request["confirm_remote_provider"]
		incoming := strings.Contains(r.URL.Path, "/agent-access/")
		if sharing != incoming || provider != incoming || string(request["project_id"]) != `"p"` || string(request["configuration"]) != `"config"` || string(request["grant"]) != `"grant"` {
			t.Errorf("incompatible control fields for %s: %+v", r.URL.Path, request)
		}
		_ = json.NewEncoder(w).Encode(entry)
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	for _, kind := range ConnectionKinds() {
		items, err := cl.ReadConnections(t.Context(), "read-"+string(kind), scope, kind)
		if err != nil || len(items) != 1 || items[0].Config.BearerTokenEnv != "DOCS_TOKEN" {
			t.Fatalf("%s inventory: %+v %v", kind, items, err)
		}
		request := ConnectionControl{ProjectIdentity: scope.ProjectIdentity, Action: "revoke", Configuration: "config", Grant: "grant", ConfirmConnect: true}
		if kind == ConnectionAccess {
			request.ConfirmSourceSharing, request.ConfirmRemoteProvider = true, true
		}
		if _, err := cl.ControlConnection(t.Context(), "control-"+string(kind), scope, kind, "docs", request); err != nil {
			t.Fatal(err)
		}
	}
	for _, target := range []string{"/integrations/docs/discover", "/apis/docs/execute", "/agent-access/docs/grant", "/remote-agents/docs/send"} {
		if allowedRequest(http.MethodPost, "/api/projects/current"+target, `{}`) {
			t.Fatal("unadmitted direct external action", target)
		}
	}
	if calls.Load() != 8 {
		t.Fatalf("unexpected connection requests: %d", calls.Load())
	}
}

func TestConnectionClientRejectsInvalidIdentityAndCanceledDiscovery(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == http.MethodPost {
			_ = json.NewEncoder(w).Encode(AgentConnection{Config: ConnectionConfig{ID: "foreign"}, Configuration: "changed"})
		} else {
			_ = json.NewEncoder(w).Encode([]AgentConnection{{Config: ConnectionConfig{ID: "docs", ProjectID: "foreign"}, Configuration: "config"}, {Config: ConnectionConfig{ID: "docs"}, Configuration: "config"}})
		}
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	if _, err := cl.ReadConnections(t.Context(), "invalid-list", scope, ConnectionMCP); err == nil {
		t.Fatal("accepted duplicate connection identities")
	}
	if _, err := cl.ReadConnections(t.Context(), "foreign-access", scope, ConnectionAccess); err == nil {
		t.Fatal("accepted another project's incoming principal")
	}
	request := ConnectionControl{ProjectIdentity: scope.ProjectIdentity, Action: "discover", Configuration: "config", ConfirmConnect: true}
	if _, err := cl.ControlConnection(t.Context(), "foreign", scope, ConnectionMCP, "docs", request); err == nil {
		t.Fatal("accepted a foreign connection receipt")
	}
	cl.Cancel("canceled")
	if _, err := cl.ControlConnection(t.Context(), "canceled", scope, ConnectionMCP, "docs", request); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-dispatch cancellation lost: %v", err)
	}
	for _, id := range []string{"", "../docs", "other/control", "docs?target=external"} {
		if _, err := cl.ControlConnection(t.Context(), "invalid", scope, ConnectionMCP, id, request); err == nil {
			t.Fatal("invalid ID dispatched", id)
		}
	}
	if calls.Load() != 3 {
		t.Fatalf("invalid requests reached server: %d", calls.Load())
	}
}
