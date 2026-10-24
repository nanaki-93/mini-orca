package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentSettingsRoutesAndCheckoutContracts(t *testing.T) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "child", ProjectRevision: "revision"}, WorktreePath: "/tmp/task space"}
	settings := AgentSettings{ProjectIdentity: scope.ProjectIdentity, State: "configured", Configuration: "configuration", Version: 9007199254740991, Limits: AgentLimits{InputTokens: 9007199254740993}}
	var observed []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observed = append(observed, r.Method+" "+r.URL.Path)
		if r.URL.Query().Get("worktree_path") != scope.WorktreePath || r.URL.Query().Get("project_id") != "child" || r.URL.Query().Get("project_revision") != "revision" {
			t.Errorf("lost checkout query: %s", r.URL)
		}
		switch r.URL.Path {
		case "/api/projects/current/agent-settings/control":
			var request AgentSettingsControl
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Version != settings.Version || !request.Confirm || request.Values == nil || request.Values.DisabledSkills == nil {
				t.Errorf("incomplete control: %+v %v", request, err)
			}
		case "/api/projects/current/agent-settings/skills/validate":
			var request AgentSkillValidationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
			}
			_ = json.NewEncoder(w).Encode(AgentSkillValidation{AgentSkillValidationRequest: request, State: "valid", Inspection: &AgentSkillInspection{Name: "review", Manifest: AgentSkillFile{Path: "skills/review/SKILL.md", Hash: "hash", SizeBytes: 42}}})
			return
		}
		_ = json.NewEncoder(w).Encode(settings)
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	result, err := cl.ReadAgentSettings(t.Context(), "read", scope)
	if err != nil || result.Limits.InputTokens != settings.Limits.InputTokens {
		t.Fatalf("read settings: %+v %v", result, err)
	}
	control := AgentSettingsControl{ProjectIdentity: scope.ProjectIdentity, Version: settings.Version, Configuration: settings.Configuration, Action: "replace", Confirm: true, Values: &AgentPreferences{DisabledSkills: []string{}}}
	if _, err := cl.ControlAgentSettings(t.Context(), "control", scope, control); err != nil {
		t.Fatal(err)
	}
	request := AgentSkillValidationRequest{ProjectIdentity: scope.ProjectIdentity, Version: settings.Version, Configuration: settings.Configuration, Directory: "skills/review"}
	validation, err := cl.ValidateAgentSkill(t.Context(), "validate", scope, request)
	if err != nil || validation.Inspection.Manifest.SizeBytes != 42 || len(observed) != 3 {
		t.Fatalf("validate: %+v %v requests=%v", validation, err, observed)
	}
	for _, methodPath := range [][2]string{{"GET", "/agent-settings/control"}, {"POST", "/agent-settings"}, {"POST", "/agent-settings/skills/enable"}, {"DELETE", "/agent-settings"}} {
		if allowedRequest(methodPath[0], "/api/projects/current"+methodPath[1], "") {
			t.Fatal("unadmitted settings route", methodPath)
		}
	}
}

func TestAgentSettingsRejectsForeignResponsesAndCanceledDispatch(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(AgentSettings{ProjectIdentity: ProjectIdentity{ProjectID: "other", ProjectRevision: "r"}, State: "configured", Configuration: "c"})
	}))
	defer server.Close()
	cl, _ := NewClient(server.URL)
	defer cl.Close()
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	if _, err := cl.ReadAgentSettings(t.Context(), "foreign", scope); err == nil {
		t.Fatal("accepted foreign response")
	}
	cl.Cancel("canceled")
	if _, err := cl.ReadAgentSettings(t.Context(), "canceled", scope); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-dispatch cancellation lost: %v", err)
	}
	if _, err := cl.ReadAgentSettings(t.Context(), "missing", AgentSettingsScope{}); err == nil || calls != 1 {
		t.Fatalf("invalid scope dispatched: %v calls=%d", err, calls)
	}
	if _, err := cl.ControlAgentSettings(t.Context(), "foreign-control", scope, AgentSettingsControl{}); err == nil || calls != 1 {
		t.Fatal("foreign control dispatched")
	}
}
