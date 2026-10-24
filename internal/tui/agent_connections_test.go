package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func connectionsTestState() *State {
	s := settingsTestState()
	v := newConnectionsView(s.Project.ProjectIdentity, client.AgentSettingsScope{ProjectIdentity: s.Project.ProjectIdentity}, s.Project.Path)
	s.AgentConnections = v
	expires := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	configs := map[client.ConnectionKind]client.ConnectionConfig{
		client.ConnectionMCP:    {ID: "docs", Transport: "streamable_http", Endpoint: "https://mcp.example", BearerTokenEnv: "MCP_TOKEN", Tools: []client.ConnectionPermission{{Name: "lookup", Effect: "read_external"}}},
		client.ConnectionAPI:    {ID: "catalog", Endpoint: "https://api.example", SpecPath: "api.json", BearerTokenEnv: "API_TOKEN", Operations: []client.ConnectionPermission{{Name: "list", Effect: "read_external"}}},
		client.ConnectionRemote: {ID: "reviewer", Endpoint: "https://agent.example/rpc", CardURL: "https://agent.example/card", Skills: []string{"review"}},
		client.ConnectionAccess: {ID: "partner", ProjectID: s.Project.ProjectID, BearerTokenEnv: "PARTNER_TOKEN", Capabilities: []string{"analysis", "proposal"}},
	}
	for _, kind := range client.ConnectionKinds() {
		entry := client.AgentConnection{Config: configs[kind], Configuration: "config-" + string(kind), State: "ready", Grant: "grant-" + string(kind), ExpiresAt: &expires, Inventory: &client.ConnectionInventory{Fingerprint: "schema-" + string(kind), Name: "reviewed inventory"}}
		if kind == client.ConnectionAccess {
			entry.State, entry.Grant, entry.ExpiresAt, entry.Inventory = "configured", "", nil, nil
		}
		v.Records[kind] = []client.AgentConnection{entry}
	}
	s.Navigate(PageConnections)
	return s
}

func TestConnectionConfirmationBindsCheckoutConfigurationAndProvider(t *testing.T) {
	for index, kind := range client.ConnectionKinds() {
		s := connectionsTestState()
		s.SelectedIdx = index
		s.confirmConnectionControl("discover")
		if s.Confirmation == nil || len(s.PendingActions) != 0 || !strings.Contains(s.Confirmation.Message, "/repo") {
			t.Fatal("connection bypassed scoped confirmation", kind)
		}
		s.AgentConnections.Records[kind][0].Configuration = "changed"
		s.Confirmation.OnConfirm(s)
		if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "changed") {
			t.Fatal("changed connection configuration accepted old confirmation", kind)
		}
		s.Error = ""
		s.confirmConnectionControl("discover")
		s.Confirmation.OnConfirm(s)
		request := s.PendingActions[0].AgentConnection.Control
		if request.ProjectIdentity != s.Project.ProjectIdentity || !request.ConfirmConnect || request.ConfirmRemoteProvider {
			t.Fatal("connection control lost scope or invented provider consent", kind)
		}
		if request.ConfirmSourceSharing != (kind == client.ConnectionAccess) {
			t.Fatal("discovery granted project source sharing", kind)
		}
	}
	s := connectionsTestState()
	s.SelectedIdx = 3
	s.confirmConnectionControl("discover")
	s.Models.Scopes["function"] = Model{Model: "different", ProviderOrigin: "https://different.example", RemoteProvider: true}
	s.Confirmation.OnConfirm(s)
	if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "changed") {
		t.Fatal("incoming grant confirmation authorized another provider")
	}
}

func TestConnectionRefreshPreservesSelectionAndPartialFailures(t *testing.T) {
	s := connectionsTestState()
	view := s.AgentConnections
	docs := view.Records[client.ConnectionMCP][0]
	other := docs
	other.Config.ID = "other"
	view.Records[client.ConnectionMCP] = []client.AgentConnection{other, docs}
	s.SelectedIdx = 1
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("refresh mutated connections: %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/projects/current/integrations":
			_ = json.NewEncoder(w).Encode([]client.AgentConnection{docs, other})
		case "/api/projects/current/apis":
			http.Error(w, "API inventory unavailable", http.StatusServiceUnavailable)
		default:
			_ = json.NewEncoder(w).Encode([]client.AgentConnection{})
		}
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	s.refreshConnections()
	model := NewBubbleModel(t.Context(), s, cl, 80, 24)
	cmd, ok := model.prepareAsyncActionCmd("read", s.PendingActions[0])
	s.PendingActions = nil
	if !ok || cmd == nil {
		t.Fatal("connection inventory did not use the async owner")
	}
	model.Update(cmd())
	if calls.Load() != 4 || s.SelectedIdx != 0 || s.selectedConnectionKey().ID != "docs" || view.Failures[client.ConnectionAPI].State != "unavailable" || len(view.Records[client.ConnectionMCP]) != 2 {
		t.Fatal("partial refresh lost selection or healthy inventory")
	}
	if len(view.Records[client.ConnectionAPI]) != 1 || len(view.Records[client.ConnectionRemote]) != 0 {
		t.Fatal("unavailable cached metadata was confused with an empty configuration")
	}
}

func TestConnectionCancellationStopsCurrentAndFutureInventoryReads(t *testing.T) {
	s := connectionsTestState()
	started, stopped := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path == "/api/projects/current/apis" {
			close(started)
			select {
			case <-r.Context().Done():
			case <-ctx.Done():
			}
			close(stopped)
			return
		}
		_ = json.NewEncoder(w).Encode([]client.AgentConnection{})
	}))
	defer server.Close()
	defer cancel()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	s.refreshConnections()
	model := NewBubbleModel(ctx, s, cl, 80, 24)
	cmd, _ := model.prepareAsyncActionCmd("read", s.PendingActions[0])
	s.PendingActions = nil
	result := make(chan agentConnectionResultMsg, 1)
	go func() { result <- cmd().(agentConnectionResultMsg) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("inventory request did not start")
	}
	s.cancelConnectionRequest()
	s.FlushActions(ctx, cl)
	select {
	case <-stopped:
	case <-ctx.Done():
		t.Fatal("inventory HTTP request was not canceled")
	}
	model.Update(<-result)
	if ctx.Err() != nil || calls.Load() != 2 || s.AgentConnections.Pending != "" || s.AgentConnections.Failures[client.ConnectionMCP].State != "unavailable" || len(s.AgentConnections.Records[client.ConnectionMCP]) != 1 {
		t.Fatal("canceled refresh published partial data or dispatched subsequent reads")
	}
}

func TestConnectionReceiptsRejectLateCheckoutAndPreserveDisconnectedState(t *testing.T) {
	s := connectionsTestState()
	s.confirmConnectionControl("revoke")
	s.Confirmation.OnConfirm(s)
	action := *s.PendingActions[0].AgentConnection
	data := s.AgentConnections.Records[client.ConnectionMCP][0]
	data.State, data.Grant, data.ExpiresAt, data.Inventory = "configured", "", nil, nil
	s.applyConnectionResult(agentConnectionResult{Action: action, Data: &data})
	if s.connectionRows()[0].State != "disconnected" {
		t.Fatal("confirmed revocation was not represented as disconnected")
	}
	s.PendingActions = nil
	s.Confirmation = nil
	s.refreshConnections()
	read := *s.PendingActions[0].AgentConnection
	s.Project.ProjectRevision = "other"
	s.applyConnectionResult(agentConnectionResult{Action: read, Lists: map[client.ConnectionKind]connectionListResult{client.ConnectionMCP: {Data: []client.AgentConnection{}}}})
	if len(s.AgentConnections.Records[client.ConnectionMCP]) != 1 || s.connectionsCurrent() {
		t.Fatal("late response replaced the new checkout")
	}
}

func TestConnectionResultCannotReplaceAnotherSelectedConnection(t *testing.T) {
	s := connectionsTestState()
	s.confirmConnectionControl("discover")
	s.Confirmation.OnConfirm(s)
	action := *s.PendingActions[0].AgentConnection
	s.SelectedIdx = 1
	data := s.AgentConnections.Records[client.ConnectionMCP][0]
	data.Grant = "new-mcp-grant"
	s.applyConnectionResult(agentConnectionResult{Action: action, Data: &data})
	if s.selectedConnectionKey().ID != "catalog" || s.AgentConnections.Records[client.ConnectionAPI][0].Grant != "grant-api" || s.AgentConnections.Records[client.ConnectionMCP][0].Grant != "new-mcp-grant" {
		t.Fatal("late connection result changed another selection or grant")
	}
}

func TestConnectionDetailsArePassiveSanitizedAndReachable(t *testing.T) {
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		for _, size := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
			for index, kind := range client.ConnectionKinds() {
				s := connectionsTestState()
				s.Palette, s.SelectedIdx = palette, index
				entry := &s.AgentConnections.Records[kind][0]
				if entry.Inventory != nil {
					entry.Inventory.Description = "remote metadata\x1b[2J\nmore"
				}
				var rendered strings.Builder
				for range 24 {
					frame := s.Render(size[0], size[1])
					rendered.WriteString(frame)
					for _, row := range strings.Split(frame, "\n") {
						if theme.VisibleRuneCount(row) != size[0] {
							t.Fatalf("connection overflow at %v: %q", size, row)
						}
					}
					s.HandleKey(KeyEvent{Key: "PageDown"})
				}
				for _, token := range []string{"CONNECTION DETAILS", "Checkout:", "/repo", "Destination:"} {
					if !strings.Contains(rendered.String(), token) {
						t.Fatalf("%s inaccessible for %s at %v", token, kind, size)
					}
				}
				if strings.Contains(rendered.String(), "\x1b[2J") || strings.Contains(rendered.String(), entry.Grant) && entry.Grant != "" || len(s.PendingActions) != 0 || s.Confirmation != nil || s.SelectedIdx != index {
					t.Fatal("connection browsing exposed a grant, dispatched or changed selection")
				}
			}
		}
	}
}
