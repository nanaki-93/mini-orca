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
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func missionHandlerDraft(t *testing.T, fixture *ChangeHandler) app.MissionMutation {
	t.Helper()
	index, err := fixture.manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := fixture.manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	limits := agentstate.MissionLimits{ModelCalls: 2, ToolCalls: 4, InputTokens: 2048, OutputTokens: 1024, ElapsedSeconds: 30, ConcurrentCalls: 1}
	return app.MissionMutation{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision, Action: "create", Plan: &agentstate.MissionPlan{Goal: "A scoped mission", Limits: limits,
		Sources:  []agentstate.MissionSource{{Path: "main.go", Hash: file.ContentHash, Exists: true}},
		Children: []agentstate.MissionChild{{ID: "first", Title: "First child", Approach: "PRIVATE_CHILD_APPROACH", AcceptanceCriteria: []string{"Runs correctly"}, WritePaths: []string{"main.go"}, ReadPaths: []string{}, Dependencies: []string{}, Limits: limits}}, IntegrationOrder: []string{"first"}}}
}

func missionHandlerMutation(t *testing.T, h *AgentMissionHandler, id string, body any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(data))
	r.SetPathValue("missionID", id)
	w := httptest.NewRecorder()
	h.Mutate(w, r)
	return w
}

func TestMissionHandlerCreateReviewReviseAndStrictBoundary(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	h := NewAgentMissionHandler(fixture.service)
	draft := missionHandlerDraft(t, fixture)
	w := missionHandlerMutation(t, h, "", draft)
	var value app.AgentMission
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || value.CatalogRevision != 1 || value.Mission.Approved {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	query := url.Values{"project_id": {draft.ProjectID}, "project_revision": {draft.ProjectRevision}}
	w = httptest.NewRecorder()
	h.List(w, httptest.NewRequest("GET", "/?"+query.Encode(), nil))
	if w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" || strings.Contains(w.Body.String(), "PRIVATE_CHILD_APPROACH") {
		t.Fatalf("list: %d %s", w.Code, w.Body)
	}
	control := app.MissionMutation{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, CatalogRevision: value.CatalogRevision, PlanHash: value.Mission.PlanHash, Action: "approve", Confirm: true}
	if w := missionHandlerMutation(t, h, value.Mission.ID, control); w.Code != 400 {
		t.Fatal("approved without review", w.Code)
	}
	control.Action, control.ChildID = "review", "first"
	w = missionHandlerMutation(t, h, value.Mission.ID, control)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || len(value.Mission.Reviewed) != 1 {
		t.Fatalf("review: %d %s", w.Code, w.Body)
	}
	if w := missionHandlerMutation(t, h, value.Mission.ID, control); w.Code != 409 {
		t.Fatal("stale review", w.Code)
	}
	control.Action, control.ChildID, control.CatalogRevision = "approve", "", value.CatalogRevision
	w = missionHandlerMutation(t, h, value.Mission.ID, control)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || !value.Mission.Approved {
		t.Fatalf("approve: %d %s", w.Code, w.Body)
	}
	draft.Action, draft.CatalogRevision, draft.PlanHash = "revise", value.CatalogRevision, value.Mission.PlanHash
	draft.Plan.Goal = "Revised mission"
	w = missionHandlerMutation(t, h, value.Mission.ID, draft)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil || value.Mission.Approved || len(value.Mission.Reviewed) != 0 {
		t.Fatalf("revise: %d %s", w.Code, w.Body)
	}
	r := httptest.NewRequest("GET", "/?"+query.Encode(), nil)
	r.SetPathValue("missionID", value.Mission.ID)
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "PRIVATE_CHILD_APPROACH") {
		t.Fatalf("detail: %d %s", w.Code, w.Body)
	}
	r.SetPathValue("missionID", "missing")
	w = httptest.NewRecorder()
	h.Detail(w, r)
	if w.Code != 404 {
		t.Fatal("missing mission", w.Code)
	}
	for _, suffix := range []string{"&unknown=1", "&project_id=other", "&cursor=unexpected"} {
		w := httptest.NewRecorder()
		h.List(w, httptest.NewRequest("GET", "/?"+query.Encode()+suffix, nil))
		if w.Code != 400 {
			t.Fatal("invalid query admitted", suffix, w.Code)
		}
	}
	for _, body := range []string{`{}`, `{} {}`, `{"unknown":true}`, `{"action":"create","action":"review"}`, `{"plan":{"limits":{"model_calls":1}}}`} {
		w := httptest.NewRecorder()
		h.Mutate(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
		if w.Code != 400 {
			t.Fatalf("invalid body: %d %s", w.Code, w.Body)
		}
	}
}

func TestMissionHandlerStalenessAndStorageFailureKeepTheirMeaning(t *testing.T) {
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	h := NewAgentMissionHandler(fixture.service)
	draft := missionHandlerDraft(t, fixture)
	w := missionHandlerMutation(t, h, "", draft)
	var value app.AgentMission
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &value) != nil {
		t.Fatal(w.Body)
	}
	if err := os.WriteFile(filepath.Join(fixture.manager.Root(), "main.go"), []byte("package main\nfunc Changed() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	control := app.MissionMutation{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, CatalogRevision: value.CatalogRevision, PlanHash: value.Mission.PlanHash, Action: "review", ChildID: "first", Confirm: true}
	if w := missionHandlerMutation(t, h, value.Mission.ID, control); w.Code != 409 {
		t.Fatalf("stale source: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	respondMission(w, nil, app.ErrMissionUnavailable)
	if w.Code != 500 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("storage failure became client error", w.Code)
	}
}

func TestMissionPlanningHandlerPreservesStrictScopeAndUnapprovedReceipt(t *testing.T) {
	var calls atomic.Int32
	answer := ""
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_ = json.NewEncoder(w).Encode(llm.ChatResponse{Choices: []llm.ChatChoice{{FinishReason: "stop", Message: llm.ChatMessage{Content: answer}}}})
	}))
	defer provider.Close()
	fixture := newChangeHandlerFixture(t, provider.URL)
	draft := missionHandlerDraft(t, fixture)
	data, err := json.Marshal(map[string]any{"version": "mission-planner-v1", "status": "planned", "reason": "", "children": draft.Plan.Children, "integration_order": draft.Plan.IntegrationOrder})
	if err != nil {
		t.Fatal(err)
	}
	answer = string(data)
	h := NewAgentMissionHandler(fixture.service)
	request := app.MissionPlanningRequest{ProjectID: draft.ProjectID, ProjectRevision: draft.ProjectRevision, Goal: draft.Plan.Goal, Sources: draft.Plan.Sources, Limits: draft.Plan.Limits, PlanningLimits: agentstate.MissionLimits{ModelCalls: 2, ToolCalls: 4, InputTokens: 128 << 10, OutputTokens: 128 << 10, ElapsedSeconds: 60, ConcurrentCalls: 1}}
	data, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	h.Plan(w, httptest.NewRequest("POST", "/missions/plan", bytes.NewReader(data)))
	var result app.MissionPlanningResult
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Mission.Approved || result.CatalogRevision != 1 || result.Usage.ModelCalls != 1 || result.PromptVersion != "mission-planner-v1" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("planning contract: %d %s", w.Code, w.Body)
	}
	w = httptest.NewRecorder()
	h.Plan(w, httptest.NewRequest("POST", "/missions/plan", bytes.NewReader(data)))
	if w.Code != 409 || calls.Load() != 1 {
		t.Fatal("stale planner editor dispatched", w.Code, calls.Load())
	}
	for _, invalid := range []string{`{}`, `{} {}`, `{"unknown":true}`, strings.Replace(string(data), `"repairs":0`, `"repairs":null`, 1), strings.Replace(string(data), `"goal":`, `"goal":"duplicate","goal":`, 1)} {
		w := httptest.NewRecorder()
		h.Plan(w, httptest.NewRequest("POST", "/missions/plan", strings.NewReader(invalid)))
		if w.Code != 400 || calls.Load() != 1 {
			t.Fatalf("invalid planner request: %d %s", w.Code, w.Body)
		}
	}
	w = httptest.NewRecorder()
	respondMission(w, nil, app.ErrMissionPlanningIncomplete)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "mission planning incomplete") {
		t.Fatal("incomplete response lost its meaning", w.Body)
	}
}
