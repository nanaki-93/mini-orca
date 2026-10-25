package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func triggerClientFixture() (AgentSettingsScope, AgentTriggers) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}}
	b := TriggerBinding{ProjectIdentity: scope.ProjectIdentity, Configuration: strings.Repeat("b", 64), SettingsVersion: 2, Policy: "3:policy", Model: "analysis-model", ProviderOrigin: "https://provider.example"}
	now := time.Now().UTC()
	v := AgentTriggers{TriggerBinding: b, State: "watch_only", Owned: true, ScopeHash: strings.Repeat("c", 64), Destination: Model{Model: b.Model, ProviderOrigin: b.ProviderOrigin, RemoteProvider: true},
		Ceilings:  TriggerCeilings{PollSeconds: 2, DebounceSeconds: 3, QuietSeconds: 60, WindowSeconds: 3600, MaxRuns: 4, MaxModelCalls: 16, MaxInputTokens: 4194304, MaxOutputTokens: 262144, RunSeconds: 120, GrantSeconds: 3600},
		RunLimits: AgentLimits{ModelCalls: 4, ToolCalls: 1, InputTokens: 1048576, OutputTokens: 65536, ElapsedSeconds: 120, ConcurrentCalls: 1},
		Record:    TriggerRecord{Version: 1, ControlRevision: 1, ProjectID: scope.ProjectID, Options: TriggerOptions{Mode: "changes", Paths: []string{"main.go"}}, State: "watching", Observed: map[string]string{"main.go": strings.Repeat("a", 64)}, Handled: map[string]string{}, UpdatedAt: now}}
	return scope, v
}

func triggerClientGrant(v *AgentTriggers, seconds int) {
	now := time.Now().UTC()
	v.State, v.Owned = "granted", true
	v.Record.Grant = &TriggerGrant{ID: "abcdefghijklmnopqrstuvwx12", ProjectID: v.ProjectID, Configuration: v.Configuration, SettingsVersion: v.SettingsVersion, Policy: v.Policy, ScopeHash: v.ScopeHash, Model: v.Model, ProviderOrigin: v.ProviderOrigin, CreatedAt: now, ExpiresAt: now.Add(time.Duration(seconds) * time.Second), RunLimits: v.RunLimits}
}

func TestTriggerClientControlsPreserveExactScopeAndSeparatePermissions(t *testing.T) {
	scope, initial := triggerClientFixture()
	var calls atomic.Int32
	var current atomic.Pointer[AgentTriggers]
	current.Store(&initial)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("project_id") != scope.ProjectID || r.URL.Query().Get("project_revision") != scope.ProjectRevision || r.URL.Query().Has("worktree_path") {
			t.Error("lost main checkout binding")
		}
		v := *current.Load()
		if r.Method == http.MethodPost {
			var request TriggerControl
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if request.ControlRevision != v.Record.ControlRevision || !request.Confirm || request.ProjectIdentity != scope.ProjectIdentity {
				t.Error("lost control admission")
			}
			v.Record.Version++
			v.Record.ControlRevision++
			switch request.Action {
			case "watch":
				v.Record.Options = *request.Options
				v.Record.Options.Paths = slices.Clone(request.Options.Paths)
				slices.Sort(v.Record.Options.Paths)
			case "grant":
				if !request.ConfirmSourceSharing || !request.ConfirmRemoteProvider || request.ScopeHash != v.ScopeHash {
					t.Error("lost grant consent")
				}
				triggerClientGrant(&v, request.GrantSeconds)
			case "revoke":
				v.State = "watch_only"
				v.Record.Grant = nil
			case "disable":
				v.State, v.Owned = "disabled", false
				v.Record.Grant = nil
				v.Record.Options = TriggerOptions{Mode: "disabled", Paths: []string{}}
				v.Record.Observed = map[string]string{}
			}
			current.Store(&v)
		}
		_ = json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	v, err := cl.ReadAgentTriggers(t.Context(), "read", scope)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"watch", "grant", "revoke", "disable"} {
		r := TriggerControl{TriggerBinding: v.TriggerBinding, ControlRevision: v.Record.ControlRevision, Action: action, Confirm: true}
		if action == "watch" {
			r.Options = &TriggerOptions{Mode: "interval", IntervalSeconds: 60, Paths: []string{"main.go"}}
		}
		if action == "grant" {
			r.ScopeHash = v.ScopeHash
			r.GrantSeconds = 60
			r.ConfirmSourceSharing = true
			r.ConfirmRemoteProvider = true
		}
		v, err = cl.ControlAgentTriggers(t.Context(), action, scope, r)
		if err != nil {
			t.Fatalf("%s: %v", action, err)
		}
	}
	if v.Owned || v.Record.Grant != nil || calls.Load() != 5 {
		t.Fatal("unexpected final receipt or dispatch count")
	}
	for _, route := range [][2]string{{"POST", "agent-triggers"}, {"GET", "agent-triggers/control"}, {"DELETE", "agent-triggers"}} {
		if allowedRequest(route[0], "/api/projects/current/"+route[1], "") {
			t.Fatal("unexpected ambient route admitted", route)
		}
	}
}

func TestTriggerClientRejectsCorruptReceiptsAndForeignScopes(t *testing.T) {
	scope, base := triggerClientFixture()
	var current atomic.Pointer[AgentTriggers]
	current.Store(&base)
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(current.Load())
	}))
	t.Cleanup(server.Close)
	cl, _ := NewClient(server.URL)
	t.Cleanup(func() { _ = cl.Close() })
	for _, mutate := range []func(*AgentTriggers){
		func(v *AgentTriggers) { v.ProjectID = "other" }, func(v *AgentTriggers) { v.Record.ControlRevision = 3 },
		func(v *AgentTriggers) { v.State = "invented" }, func(v *AgentTriggers) { v.Destination.Model = "other" },
		func(v *AgentTriggers) { v.Record.Options.Paths = []string{"../secret"} }, func(v *AgentTriggers) { v.RunLimits.ConcurrentCalls = 2 },
		func(v *AgentTriggers) { v.Record.Window.Charged.Runs = 1 }, func(v *AgentTriggers) { v.Ceilings.MaxRuns = 0 },
		func(v *AgentTriggers) { triggerClientGrant(v, 60); v.Record.Grant.Model = "other" },
		func(v *AgentTriggers) { triggerClientGrant(v, 60); v.Owned = false },
	} {
		data, _ := json.Marshal(base)
		var invalid AgentTriggers
		_ = json.Unmarshal(data, &invalid)
		mutate(&invalid)
		current.Store(&invalid)
		if _, err := cl.ReadAgentTriggers(t.Context(), "bad", scope); err == nil {
			t.Fatal("invalid ambient receipt accepted")
		}
	}
	before := calls.Load()
	foreign := scope
	foreign.WorktreePath = "/tmp/task"
	if _, err := cl.ReadAgentTriggers(t.Context(), "task", foreign); err == nil {
		t.Fatal("task read dispatched")
	}
	if _, _, err := cl.RefreshAgentTriggers(t.Context(), "task-refresh", foreign); err == nil {
		t.Fatal("task refresh dispatched")
	}
	r := TriggerControl{TriggerBinding: base.TriggerBinding, ControlRevision: 1, Action: "grant", Confirm: true, ScopeHash: base.ScopeHash, GrantSeconds: 60}
	if _, err := cl.ControlAgentTriggers(t.Context(), "unconfirmed", scope, r); err == nil {
		t.Fatal("unconfirmed grant dispatched")
	}
	cl.Cancel("canceled")
	if _, err := cl.ReadAgentTriggers(t.Context(), "canceled", scope); !errors.Is(err, context.Canceled) {
		t.Fatal("queued cancellation lost", err)
	}
	if calls.Load() != before {
		t.Fatal("rejected admission reached transport")
	}
}

func TestTriggerClientRefreshAdvancesOnlyTheSameProject(t *testing.T) {
	scope, v := triggerClientFixture()
	v.ProjectRevision = "new-revision"
	var foreign atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/current/index" {
			id := v.ProjectIdentity
			if foreign.Load() {
				id.ProjectID = "other"
			}
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: id})
			return
		}
		if r.URL.Query().Get("project_revision") != "new-revision" {
			t.Error("refresh reused old revision")
		}
		_ = json.NewEncoder(w).Encode(v)
	}))
	t.Cleanup(server.Close)
	cl, _ := NewClient(server.URL)
	t.Cleanup(func() { _ = cl.Close() })
	value, index, err := cl.RefreshAgentTriggers(t.Context(), "refresh", scope)
	if err != nil || value.ProjectRevision != "new-revision" || index.ProjectIdentity != value.ProjectIdentity {
		t.Fatal("current revision was not captured", err)
	}
	foreign.Store(true)
	if _, _, err = cl.RefreshAgentTriggers(t.Context(), "foreign", scope); err == nil {
		t.Fatal("another project was adopted")
	}
}
