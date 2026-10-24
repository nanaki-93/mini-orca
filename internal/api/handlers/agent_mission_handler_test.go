package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/llm"
)

func missionExecutionHandlerFixture(t *testing.T) (*ChangeHandler, *app.AgentMission, app.MissionExecutionRequest) {
	t.Helper()
	fixture := newChangeHandlerFixture(t, "http://127.0.0.1:1")
	root := fixture.manager.Root()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mini-orca/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"commit", "-m", "Initial fixture"}} {
		flags := []string{"-C", root, "-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgSign=false", "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}
		if out, err := exec.CommandContext(t.Context(), "git", append(flags, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v %s", err, out)
		}
	}
	if _, err := fixture.service.Reindex(); err != nil {
		t.Fatal(err)
	}
	saved, err := fixture.service.MutateMission(t.Context(), "", missionHandlerDraft(t, fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, child := range []string{"first", ""} {
		request := app.MissionMutation{ProjectID: saved.Mission.ProjectID, ProjectRevision: saved.Mission.ProjectRevision, CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, ChildID: child, Action: "review", Confirm: true}
		if child == "" {
			request.Action = "approve"
		}
		saved, err = fixture.service.MutateMission(t.Context(), saved.Mission.ID, request)
		if err != nil {
			t.Fatal(err)
		}
	}
	repo, err := fixture.service.GitRepository(t.Context(), saved.Mission.ProjectID, saved.Mission.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	return fixture, saved, app.MissionExecutionRequest{ProjectID: saved.Mission.ProjectID, ProjectRevision: saved.Mission.ProjectRevision, CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, Action: "prepare", Base: &agentstate.MissionBase{Head: repo.Head, Branch: repo.Branch}, Confirm: true}
}

func TestMissionExecutionHandlerExactLifecycleAndStrictRequests(t *testing.T) {
	fixture, saved, request := missionExecutionHandlerFixture(t)
	h := NewAgentMissionHandler(fixture.service)
	control := func(data []byte) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/missions/"+saved.Mission.ID+"/execution", bytes.NewReader(data))
		r.SetPathValue("missionID", saved.Mission.ID)
		w := httptest.NewRecorder()
		h.ControlExecution(w, r)
		return w
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{
		string(data) + ` {}`, `null`, `[]`, strings.Replace(string(data), `"action":"prepare"`, `"action":"prepare","action":"prepare"`, 1),
		strings.Replace(string(data), `"confirm":true`, `"confirm":true,"confirm_execution":true`, 1),
		strings.Replace(string(data), `"confirm":true`, `"confirm":false`, 1),
		strings.Replace(string(data), `"action":"prepare"`, `"action":"start"`, 1),
		strings.Replace(string(data), `"head":`, `"extra":false,"head":`, 1),
	} {
		if w := control([]byte(invalid)); w.Code != 400 {
			t.Fatalf("strict boundary: %d %s", w.Code, w.Body)
		}
	}
	query := url.Values{"project_id": {request.ProjectID}, "project_revision": {request.ProjectRevision}}
	read := func(values url.Values) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", "/missions/"+saved.Mission.ID+"/execution?"+values.Encode(), nil)
		r.SetPathValue("missionID", saved.Mission.ID)
		w := httptest.NewRecorder()
		h.Execution(w, r)
		return w
	}
	if w := read(query); w.Code != 404 {
		t.Fatalf("missing receipt: %d %s", w.Code, w.Body)
	}
	w := control(data)
	var prepared app.AgentMissionExecution
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &prepared) != nil || prepared.Stale || prepared.Execution.State != "prepared" || prepared.Execution.Revision != 1 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("prepare: %d %s", w.Code, w.Body)
	}
	if w := control(data); w.Code != 409 {
		t.Fatalf("replay: %d %s", w.Code, w.Body)
	}
	if w := read(query); w.Code != 200 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("inspect: %d %s", w.Code, w.Body)
	}
	if w := read(url.Values{"project_id": {"foreign"}, "project_revision": {request.ProjectRevision}}); w.Code != 409 || strings.Contains(w.Body.String(), "PRIVATE_CHILD_APPROACH") {
		t.Fatalf("scope: %d %s", w.Code, w.Body)
	}
	if err := os.WriteFile(filepath.Join(fixture.manager.Root(), "main.go"), []byte("package main\n// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	request.Action, request.CatalogRevision, request.Base = "cancel", 0, nil
	request.ExecutionID, request.ExecutionRevision = prepared.Execution.ID, prepared.Execution.Revision
	data, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	w = control(data)
	var canceled app.AgentMissionExecution
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &canceled) != nil || !canceled.Stale || canceled.SourceStatus != "changed" || canceled.Execution.State != "canceled" || canceled.Execution.Revision != 2 {
		t.Fatalf("cancel changed scope: %d %s", w.Code, w.Body)
	}
	if w := control(data); w.Code != 409 {
		t.Fatalf("cancel replay: %d %s", w.Code, w.Body)
	}
	files, err := filepath.Glob(filepath.Join(fixture.manager.Root(), ".mini-orca", "agents", "*", "mission-executions.json"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	if err := os.WriteFile(files[0], []byte(`{"version":99}`), 0600); err != nil {
		t.Fatal(err)
	}
	if w := read(query); w.Code != 500 || strings.Contains(w.Body.String(), "PRIVATE_CHILD_APPROACH") {
		t.Fatalf("corruption: %d %s", w.Code, w.Body)
	}
}

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

func TestMissionExecutionHandlerAdmitsStopControlsAndRejectsForgedGrants(t *testing.T) {
	fixture, saved, prepare := missionExecutionHandlerFixture(t)
	prepared, err := fixture.service.ControlMissionExecution(t.Context(), saved.Mission.ID, prepare)
	if err != nil {
		t.Fatal(err)
	}
	h := NewAgentMissionHandler(fixture.service)
	control := func(request app.MissionExecutionRequest) *httptest.ResponseRecorder {
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest("POST", "/", bytes.NewReader(data))
		r.SetPathValue("missionID", saved.Mission.ID)
		w := httptest.NewRecorder()
		h.ControlExecution(w, r)
		return w
	}
	request := app.MissionExecutionRequest{ProjectID: prepare.ProjectID, ProjectRevision: prepare.ProjectRevision, Action: "start", PlanHash: prepare.PlanHash, ExecutionID: prepared.Execution.ID, ExecutionRevision: prepared.Execution.Revision, Confirm: true}
	if w := control(request); w.Code != 400 {
		t.Fatal("start accepted missing execution consent", w.Code, w.Body)
	}
	for _, action := range []string{"pause", "cancel", "reconcile"} {
		invalid := request
		invalid.Action = action
		invalid.ConfirmedProfiles = []string{"function"}
		if w := control(invalid); w.Code != 400 {
			t.Fatal("control accepted provider grant", w.Code, w.Body)
		}
	}
	unchanged, err := fixture.service.MissionExecution(t.Context(), app.AgentRunQuery{ProjectID: request.ProjectID, ProjectRevision: request.ProjectRevision}, saved.Mission.ID)
	if err != nil || unchanged.Execution.Revision != 1 || unchanged.OwnerState != "idle" || len(unchanged.ReadyChildren) != 1 {
		t.Fatal(unchanged, err)
	}
	store, err := agentstate.Open(fixture.manager.Root())
	if err != nil {
		t.Fatal(err)
	}
	identity := agentstate.MissionExecutionIdentity{ProjectID: request.ProjectID, MissionID: saved.Mission.ID, ExecutionID: request.ExecutionID, Revision: request.ExecutionRevision, PlanHash: request.PlanHash}
	wave, err := store.BeginMissionWave(t.Context(), identity, "another-owner", func() error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	request.Action, request.ExecutionRevision = "pause", wave.Revision
	w := control(request)
	var paused app.AgentMissionExecution
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &paused) != nil || paused.Execution.State != "pausing" || paused.OwnerState != "waiting_for_owner" || paused.Execution.Wave.ID != wave.Wave.ID {
		t.Fatal(w.Code, w.Body)
	}
	request.Action, request.ExecutionRevision = "cancel", paused.Execution.Revision
	w = control(request)
	var canceled app.AgentMissionExecution
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &canceled) != nil || canceled.Execution.State != "canceling" || canceled.Execution.Children[0].WaveID != wave.Wave.ID {
		t.Fatal(w.Code, w.Body)
	}
	if w := control(request); w.Code != 409 {
		t.Fatal("stale control accepted", w.Code, w.Body)
	}
	identity.Revision = canceled.Execution.Revision
	finished, err := store.FinishMissionWave(t.Context(), identity, wave.Wave.ID, false, "stopped", func() error { return nil })
	if err != nil || finished.State != "canceled" || finished.Children[0].State != "interrupted" {
		t.Fatal(finished, err)
	}
	if _, err := os.Stat(filepath.Join(fixture.manager.Root(), ".worktrees")); !os.IsNotExist(err) {
		t.Fatal("stop control created worktree", err)
	}
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
