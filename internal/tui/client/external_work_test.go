package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
)

const externalFixtureID = "abcdefghijklmnopqrstuvwx12"

func externalClientFixture() (AgentSettingsScope, APIAction, Delegation, ResearchReport) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, WorktreePath: "/tmp/task space"}
	request := json.RawMessage(`{"operation_id":"create","method":"POST","url":"https://api.example/items","body":{"title":"reviewed"}}`)
	hash := sha256.Sum256(request)
	action := APIAction{ProjectIdentity: scope.ProjectIdentity, ID: externalFixtureID, Version: 1, Request: request, RequestHash: hex.EncodeToString(hash[:]), State: "candidate", CanApprove: true}
	remote := Delegation{ProjectIdentity: scope.ProjectIdentity, ID: externalFixtureID, Version: 1, RemoteID: "reviewer", Kind: "review", Skill: "review", State: "unknown", Advisory: true}
	report := ResearchReport{ProjectIdentity: scope.ProjectIdentity, ID: externalFixtureID, Title: "Question", State: "grounded", Model: "research-model"}
	return scope, action, remote, report
}

func TestExternalClientRoutesPreserveScopeAndReceiptGuards(t *testing.T) {
	scope, action, remote, report := externalClientFixture()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Query().Get("worktree_path") != scope.WorktreePath || r.URL.Query().Get("project_id") != scope.ProjectID || r.URL.Query().Get("project_revision") != scope.ProjectRevision {
			t.Errorf("lost scope: %s", r.URL)
		}
		if r.Method == http.MethodGet {
			switch r.URL.Path {
			case "/api/projects/current/api-actions":
				_ = json.NewEncoder(w).Encode([]APIAction{action})
			case "/api/projects/current/research":
				_ = json.NewEncoder(w).Encode(ResearchOverview{State: "available", Model: report.Model, Reports: []ResearchReport{report}})
			case "/api/projects/current/delegations":
				_ = json.NewEncoder(w).Encode([]Delegation{remote})
			case "/api/projects/current/delegations/" + externalFixtureID:
				_ = json.NewEncoder(w).Encode(remote)
			case "/api/projects/current/index":
				_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: scope.ProjectIdentity})
			default:
				t.Errorf("unexpected read: %s", r.URL.Path)
			}
			return
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if string(body["project_id"]) != `"project"` || string(body["project_revision"]) != `"revision"` {
			t.Error("missing body identity")
		}
		switch r.URL.Path {
		case "/api/projects/current/api-actions/" + externalFixtureID + "/control":
			if string(body["version"]) != "1" || string(body["request_hash"]) != `"`+action.RequestHash+`"` || len(body) != 6 {
				t.Errorf("lost exact API control: %v", body)
			}
			value := action
			value.Version++
			value.State = "unknown"
			_ = json.NewEncoder(w).Encode(value)
		case "/api/projects/current/research":
			if string(body["confirm_search"]) != "true" || string(body["confirm_source_sharing"]) != "true" || len(body) != 7 {
				t.Errorf("lost research consent: %v", body)
			}
			_ = json.NewEncoder(w).Encode(report)
		case "/api/projects/current/delegations":
			if string(body["confirm_delegate"]) != "true" || len(body) != 9 {
				t.Errorf("lost delegation intent: %v", body)
			}
			_ = json.NewEncoder(w).Encode(remote)
		case "/api/projects/current/delegations/" + externalFixtureID + "/control":
			if string(body["version"]) != "1" || string(body["confirm"]) != "true" || len(body) != 7 {
				t.Errorf("lost remote control guard: %v", body)
			}
			value := remote
			value.Version++
			_ = json.NewEncoder(w).Encode(value)
		default:
			t.Errorf("unexpected write: %s", r.URL.Path)
		}
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	if _, err := cl.ReadAPIActions(t.Context(), "actions", scope); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ReadResearch(t.Context(), "research", scope); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ReadDelegations(t.Context(), "delegations", scope); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.Delegation(t.Context(), "detail", scope, externalFixtureID); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ExternalIndex(t.Context(), "index", scope); err != nil {
		t.Fatal(err)
	}
	if value, err := cl.ControlAPIAction(t.Context(), "approve", scope, action.ID, APIActionControl{ProjectIdentity: scope.ProjectIdentity, Version: 1, RequestHash: action.RequestHash, Action: "approve", Confirm: true}); err != nil || value.State != "unknown" {
		t.Fatalf("uncertainty lost: %+v %v", value, err)
	}
	if _, err := cl.RunResearch(t.Context(), "run-research", scope, ResearchRequest{ProjectIdentity: scope.ProjectIdentity, Question: "Question", ConfirmSearch: true, ConfirmSourceSharing: true}); err != nil {
		t.Fatal(err)
	}
	use := IntegrationUse{ID: "reviewer", Grant: "grant", SchemaFingerprint: "card", ConfirmSourceSharing: true}
	if _, err := cl.StartDelegation(t.Context(), "delegate", scope, DelegationRequest{ProjectIdentity: scope.ProjectIdentity, Kind: "review", Skill: "review", Remote: use, ConfirmDelegate: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := cl.ControlDelegation(t.Context(), "refresh", scope, remote.ID, DelegationControl{ProjectIdentity: scope.ProjectIdentity, Version: 1, Action: "refresh", Remote: use, Confirm: true}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 9 {
		t.Fatalf("unexpected requests: %d", calls.Load())
	}
}

func TestExternalClientRejectsForeignTamperedAndCanceledReceipts(t *testing.T) {
	scope, action, remote, report := externalClientFixture()
	var mode atomic.Int64
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		switch mode.Load() {
		case 0:
			value := action
			value.Request = json.RawMessage(`{"changed":true}`)
			_ = json.NewEncoder(w).Encode([]APIAction{value})
		case 1:
			_ = json.NewEncoder(w).Encode([]APIAction{action, action})
		case 2:
			value := remote
			value.ProjectID = "foreign"
			_ = json.NewEncoder(w).Encode(value)
		case 3:
			value := report
			value.ProjectRevision = "old"
			_ = json.NewEncoder(w).Encode(value)
		case 4:
			value := remote
			value.RemoteID = "foreign"
			_ = json.NewEncoder(w).Encode(value)
		}
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	for i := int64(0); i < 2; i++ {
		mode.Store(i)
		if _, err := cl.ReadAPIActions(t.Context(), "bad-actions", scope); err == nil {
			t.Fatal("accepted tampered or duplicate action")
		}
	}
	mode.Store(2)
	if _, err := cl.Delegation(t.Context(), "foreign", scope, externalFixtureID); err == nil {
		t.Fatal("accepted foreign project")
	}
	mode.Store(3)
	if _, err := cl.RunResearch(t.Context(), "old", scope, ResearchRequest{ProjectIdentity: scope.ProjectIdentity}); err == nil {
		t.Fatal("accepted old revision for new research")
	}
	mode.Store(4)
	if _, err := cl.StartDelegation(t.Context(), "wrong-remote", scope, DelegationRequest{ProjectIdentity: scope.ProjectIdentity, Remote: IntegrationUse{ID: "reviewer"}, Kind: "review", Skill: "review"}); err == nil {
		t.Fatal("accepted another remote destination")
	}
	cl.Cancel("canceled")
	if _, err := cl.RunResearch(t.Context(), "canceled", scope, ResearchRequest{ProjectIdentity: scope.ProjectIdentity}); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", err)
	}
	for _, id := range []string{"../receipt", externalFixtureID + "/control", "", externalFixtureID + "?url=https://other.example"} {
		if _, err := cl.ControlDelegation(t.Context(), "invalid", scope, id, DelegationControl{ProjectIdentity: scope.ProjectIdentity}); err == nil {
			t.Fatal("invalid route reached daemon", id)
		}
	}
	if calls.Load() != 5 {
		t.Fatal("invalid or canceled request dispatched", calls.Load())
	}
}

func TestResearchReportURLIsLocalAndDoesNotFetch(t *testing.T) {
	scope, _, _, _ := externalClientFixture()
	for _, origin := range []string{"http://127.0.0.1:8080", "http://[::1]:8080", "https://localhost:8080"} {
		cl, err := NewClient(origin)
		if err != nil {
			t.Fatal(err)
		}
		location, err := cl.ResearchReportURL(scope, externalFixtureID)
		if err != nil {
			t.Fatal(err)
		}
		parsed, _ := url.Parse(location)
		if parsed.Query().Get("worktree_path") != scope.WorktreePath || parsed.Query().Get("project_revision") != scope.ProjectRevision || parsed.Path != "/api/projects/current/research/"+externalFixtureID+"/report" {
			t.Fatal("lost report scope", location)
		}
		_ = cl.Close()
	}
	for _, origin := range []string{"https://remote.example", "http://127.0.0.1.example", "https://local.example"} {
		cl, _ := NewClient(origin)
		if _, err := cl.ResearchReportURL(scope, externalFixtureID); err == nil {
			t.Fatal("opened nonlocal report", origin)
		}
		_ = cl.Close()
	}
	for _, path := range []string{"/research/send", "/research/" + externalFixtureID + "/report", "/delegations/" + externalFixtureID + "/retry", "/api-actions/" + externalFixtureID + "/approve"} {
		if allowedRequest(http.MethodPost, "/api/projects/current"+path, `{}`) {
			t.Fatal("unadmitted external route", path)
		}
	}
	if allowedRequest(http.MethodGet, "https://remote.example/research", "") || allowedRequest(http.MethodGet, "/api/projects/current/research/"+externalFixtureID+"/report", "") {
		t.Fatal("HTML or remote URL entered JSON client")
	}
}
