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
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
)

func TestAgentSettingsHTTPRequiresCurrentConfirmedCompleteValues(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.ModelScopes.Function.APIKey = "SETTINGS_SECRET_MUST_NOT_LEAK"
	service, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewAgentSettingsHandler(service)
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	query := url.Values{"project_id": {index.ProjectID}, "project_revision": {index.ProjectRevision}}
	w := httptest.NewRecorder()
	handler.Get(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	var view app.AgentSettings
	if err := json.Unmarshal(w.Body.Bytes(), &view); err != nil || w.Code != 200 || view.State != "configured" || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), cfg.ModelScopes.Function.APIKey) {
		t.Fatal("settings inventory", w.Code, w.Body.String(), err)
	}
	post := func(data []byte) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handler.Control(w, httptest.NewRequest("POST", "/", bytes.NewReader(data)))
		return w
	}
	request := app.AgentSettingsControl{ProjectID: view.ProjectID, ProjectRevision: view.ProjectRevision, Configuration: view.Configuration, Version: view.Version, Action: "replace", Values: &agentstate.AgentPreferences{Roles: view.Roles, Limits: view.Limits, DisabledSkills: []string{}}}
	data, _ := json.Marshal(request)
	if w := post(data); w.Code != http.StatusBadRequest {
		t.Fatal("missing confirmation accepted", w.Code, w.Body.String())
	}
	request.Confirm = true
	data, _ = json.Marshal(request)
	for _, bad := range [][]byte{
		append(append([]byte{}, data...), []byte(` {}`)...),
		[]byte(strings.TrimSuffix(string(data), "}") + `,"apply":true}`),
		[]byte(strings.TrimSuffix(string(data), "}") + `,"confirm":false}`),
		[]byte(strings.Replace(string(data), `"limits":{`, `"limits":{"unlimited":true,`, 1)),
		[]byte(strings.Replace(string(data), `"repairs":2`, `"repairs":null`, 1)),
		[]byte(strings.Replace(string(data), `,"repairs":2`, ``, 1)),
		[]byte(strings.Replace(string(data), `"disabled_skills":[]`, `"disabled_skills":null`, 1)),
	} {
		if w := post(bad); w.Code != http.StatusBadRequest {
			t.Fatal("ambiguous settings accepted", w.Code, w.Body.String())
		}
	}
	request.Values.Limits.ModelCalls = 10
	data, _ = json.Marshal(request)
	w = post(data)
	var saved app.AgentSettings
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Code != 200 || saved.Version != 1 || saved.Limits.ModelCalls != 10 || saved.State != "overridden" {
		t.Fatal("replace", w.Code, w.Body.String(), err)
	}
	if w := post(data); w.Code != http.StatusConflict {
		t.Fatal("stale version accepted", w.Code, w.Body.String())
	}
	request.Action, request.Version, request.Values = "reset", saved.Version, nil
	data, _ = json.Marshal(request)
	w = post(data)
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || w.Code != 200 || saved.State != "configured" || saved.Version != 2 {
		t.Fatal("reset", w.Code, w.Body.String(), err)
	}
	for _, suffix := range []string{"", "?" + query.Encode() + "&connect=true", "?" + query.Encode() + "&project_id=another"} {
		w := httptest.NewRecorder()
		handler.Get(w, httptest.NewRequest("GET", "/"+suffix, nil))
		if w.Code != http.StatusBadRequest {
			t.Fatal("invalid settings query accepted", suffix, w.Code)
		}
	}
}

func TestAgentSkillValidationHTTPIsReadOnlyAndBoundToSettings(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	root := fixture.manager.Root()
	directory := filepath.Join(root, "skills", "review-code")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "SKILL.md"), []byte("---\nname: review-code\ndescription: Review code.\nallowed-tools: read_snippet\n---\nInspect the captured sources.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.service.Reindex(); err != nil {
		t.Fatal(err)
	}
	cfg := scopedHandlerConfig("http://127.0.0.1:1")
	cfg.AgentRuntime.Skills = []string{"skills/review-code"}
	service, err := app.New(cfg, fixture.manager)
	if err != nil {
		t.Fatal(err)
	}
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	view, err := service.AgentSettings(t.Context(), app.AgentRunQuery{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision})
	if err != nil {
		t.Fatal(err)
	}
	request := app.AgentSkillValidationRequest{ProjectID: view.ProjectID, ProjectRevision: view.ProjectRevision, Configuration: view.Configuration, Version: view.Version, Directory: "skills/review-code"}
	handler := NewAgentSettingsHandler(service)
	post := func() *httptest.ResponseRecorder {
		data, _ := json.Marshal(request)
		w := httptest.NewRecorder()
		handler.ValidateSkill(w, httptest.NewRequest("POST", "/", bytes.NewReader(data)))
		return w
	}
	w := post()
	var result app.AgentSkillValidation
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil || w.Code != 200 || result.State != "valid" || len(result.Inspection.RequiredTools) != 1 || strings.Contains(w.Body.String(), "Inspect the captured sources") {
		t.Fatal("validation", w.Code, w.Body.String(), err)
	}
	request.Version++
	if w := post(); w.Code != http.StatusConflict {
		t.Fatal("stale validation accepted", w.Code)
	}
	if _, err := os.Stat(filepath.Join(root, ".mini-orca", "agents")); !os.IsNotExist(err) {
		t.Fatal("skill validation created state", err)
	}
}
