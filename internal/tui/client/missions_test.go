package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestMissionWireContractUsesRealHandlersAndGuardedRoutes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := project.NewManager(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Set(root, &project.Analysis{Name: "mission fixture", Path: root}); err != nil {
		t.Fatal(err)
	}
	service, err := app.New(config.Default(), manager)
	if err != nil {
		t.Fatal(err)
	}
	index, err := manager.Index()
	if err != nil {
		t.Fatal(err)
	}
	file, err := manager.IndexedFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	h := handlers.NewAgentMissionHandler(service)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects/current/missions", h.List)
	mux.HandleFunc("POST /api/projects/current/missions", h.Mutate)
	mux.HandleFunc("GET /api/projects/current/missions/{missionID}", h.Detail)
	mux.HandleFunc("POST /api/projects/current/missions/{missionID}", h.Mutate)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("worktree_path") != root {
			t.Error("request lost its selected checkout")
		}
		// The daemon's worktree router consumes this selector before handlers.
		query.Del("worktree_path")
		r.URL.RawQuery = query.Encode()
		mux.ServeHTTP(w, r)
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	identity := ProjectIdentity{ProjectID: index.ProjectID, ProjectRevision: index.ProjectRevision}
	query := "?" + url.Values{"project_id": {identity.ProjectID}, "project_revision": {identity.ProjectRevision}, "worktree_path": {root}}.Encode()
	base := "/api/projects/current/missions"
	var list MissionList
	if err := cl.DoJSON(t.Context(), "list", "GET", base+query, nil, &list); err != nil || list.ProjectIdentity != identity || list.CatalogRevision != 0 || len(list.Missions) != 0 {
		t.Fatal(list, err)
	}
	limits := AgentLimits{ModelCalls: 4, ToolCalls: 8, InputTokens: 4096, OutputTokens: 2048, ElapsedSeconds: 60, ConcurrentCalls: 1, Repairs: 1}
	request := MissionMutation{ProjectIdentity: identity, Action: "create", Plan: &MissionPlan{Goal: "Refine Run", Limits: limits, Sources: []MissionSource{{Path: "main.go", Hash: file.ContentHash, Exists: true}}, IntegrationOrder: []string{"refine"}, Children: []MissionChild{{ID: "refine", Title: "Refine Run", Approach: "Implement the scoped behavior", AcceptanceCriteria: []string{"Expected result"}, WritePaths: []string{"main.go"}, ReadPaths: []string{}, Dependencies: []string{}, Limits: limits}}}}
	var saved AgentMission
	if err := cl.DoJSON(t.Context(), "create", "POST", base+query, request, &saved); err != nil || saved.Mission.ProjectIdentity != identity || saved.CatalogRevision != 1 || saved.SourceStatus != "current" {
		t.Fatal(saved, err)
	}
	var detail AgentMission
	if err := cl.DoJSON(t.Context(), "detail", "GET", base+"/"+saved.Mission.ID+query, nil, &detail); err != nil || detail.Mission.PlanHash != saved.Mission.PlanHash || detail.Mission.Plan.Children[0].Limits != limits {
		t.Fatal(detail, err)
	}
	guardedScope := AgentSettingsScope{ProjectIdentity: identity, WorktreePath: root}
	if records, err := cl.ReadMissions(t.Context(), "owned-list", guardedScope); err != nil || len(records.Missions) != 1 {
		t.Fatal("typed mission catalog", records, err)
	}
	if record, err := cl.MissionDetail(t.Context(), "owned-detail", guardedScope, saved.Mission.ID); err != nil || record.Mission.PlanHash != saved.Mission.PlanHash {
		t.Fatal("typed mission detail", record, err)
	}
	control := MissionMutation{ProjectIdentity: identity, CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, Action: "review", ChildID: "refine", Confirm: true}
	reviewed, err := cl.MutateMission(t.Context(), "review", guardedScope, saved.Mission.ID, control)
	if err != nil || len(reviewed.Mission.Reviewed) != 1 {
		t.Fatal(reviewed, err)
	}
	saved = *reviewed
	body, _ := json.Marshal(control)
	response, err := cl.Request(t.Context(), "stale", "POST", base+"/"+saved.Mission.ID+query, string(body))
	if err != nil || response.Status != 409 {
		t.Fatal(response, err)
	}
	for _, target := range []string{base + "/../escape", base + "/id/execute", base + "/id/apply"} {
		if _, err := cl.Request(t.Context(), "rejected", "POST", target, `{}`); err == nil {
			t.Fatal("unimplemented mutation route allowed", target)
		}
	}
}

func TestMissionPlanningClientCarriesBothBudgetsAndCheckoutIdentity(t *testing.T) {
	identity := ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}
	limits := AgentLimits{ModelCalls: 4, ToolCalls: 8, InputTokens: 65536, OutputTokens: 32768, ElapsedSeconds: 60, ConcurrentCalls: 1}
	request := MissionPlanningRequest{ProjectIdentity: identity, CatalogRevision: 7, Goal: "Plan selected scope", Sources: []MissionSource{{Path: "new.go"}}, Limits: limits, PlanningLimits: limits, ConfirmRemoteProvider: true}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/projects/current/missions/plan" || r.URL.Query().Get("worktree_path") != "/tmp/selected checkout" {
			t.Error("planning route lost checkout ownership")
		}
		var received MissionPlanningRequest
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil || received.ProjectIdentity != identity || received.CatalogRevision != 7 || received.Limits != limits || received.PlanningLimits != limits || !received.ConfirmRemoteProvider {
			t.Error("planning request lost its identity or budget", err)
		}
		_ = json.NewEncoder(w).Encode(MissionPlanningResult{AgentMission: AgentMission{CatalogRevision: 8, Mission: MissionRecord{ProjectIdentity: identity, ID: "mission-fixture", PlanHash: strings.Repeat("a", 64)}, SourceStatus: "current"}, InputHash: strings.Repeat("b", 64), PromptVersion: "mission-planner-v1", PlanningLimits: limits, Usage: AgentUsage{ModelCalls: 1, UnknownUsageCalls: 1, InputTokens: 1024}, ContextManifest: ContextManifest{Scope: "function", TokenLimit: 8192}})
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	target := "/api/projects/current/missions/plan?" + url.Values{"project_id": {identity.ProjectID}, "project_revision": {identity.ProjectRevision}, "worktree_path": {"/tmp/selected checkout"}}.Encode()
	var result MissionPlanningResult
	if err := cl.DoJSON(t.Context(), "planning", "POST", target, request, &result); err != nil || result.CatalogRevision != 8 || result.Mission.ProjectIdentity != identity || result.Mission.Approved || result.Usage.UnknownUsageCalls != 1 || result.ContextManifest.Scope != "function" {
		t.Fatal(result, err)
	}
	if _, err := cl.Request(t.Context(), "invalid-method", "PATCH", target, `{}`); err == nil {
		t.Fatal("unsupported planning method dispatched")
	}
}

func missionClientFixture() (AgentSettingsScope, MissionPlanningRequest, MissionPlanningResult) {
	scope := AgentSettingsScope{ProjectIdentity: ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}, WorktreePath: "/selected/task"}
	limits := AgentLimits{ModelCalls: 2, ToolCalls: 4, InputTokens: 4096, OutputTokens: 2048, ElapsedSeconds: 60, ConcurrentCalls: 1}
	request := MissionPlanningRequest{ProjectIdentity: scope.ProjectIdentity, Goal: "New helper", Sources: []MissionSource{{Path: "new.go"}}, Limits: limits, PlanningLimits: limits, ConfirmRemoteProvider: true}
	plan := MissionPlan{Goal: request.Goal, Sources: request.Sources, Limits: limits, Children: []MissionChild{{ID: "helper", Title: "Helper", Approach: "Pure function", WritePaths: []string{"new.go"}, AcceptanceCriteria: []string{"Returns expected value"}, Limits: limits}}, IntegrationOrder: []string{"helper"}}
	result := MissionPlanningResult{AgentMission: AgentMission{CatalogRevision: 1, SourceStatus: "current", Mission: MissionRecord{ProjectIdentity: scope.ProjectIdentity, ID: "mission-abcdefghijklmnopqrst", PlanHash: strings.Repeat("a", 64), Plan: plan}}, InputHash: strings.Repeat("b", 64), PromptVersion: "mission-planner-v1", PlanningLimits: limits}
	return scope, request, result
}

func TestMissionClientRejectsForeignAndChangedPlanningReceipts(t *testing.T) {
	for name, mutate := range map[string]func(*MissionPlanningResult){
		"project":     func(r *MissionPlanningResult) { r.Mission.ProjectID = "foreign" },
		"revision":    func(r *MissionPlanningResult) { r.Mission.ProjectRevision = "foreign" },
		"hash":        func(r *MissionPlanningResult) { r.Mission.PlanHash = "invalid" },
		"approved":    func(r *MissionPlanningResult) { r.Mission.Approved = true },
		"reviewed":    func(r *MissionPlanningResult) { r.Mission.Reviewed = []string{"helper"} },
		"goal":        func(r *MissionPlanningResult) { r.Mission.Plan.Goal = "Changed goal" },
		"empty graph": func(r *MissionPlanningResult) { r.Mission.Plan.Children = nil },
		"catalog":     func(r *MissionPlanningResult) { r.CatalogRevision = 2 },
		"budget":      func(r *MissionPlanningResult) { r.PlanningLimits.ModelCalls++ },
		"prompt":      func(r *MissionPlanningResult) { r.PromptVersion = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			scope, request, result := missionClientFixture()
			mutate(&result)
			var calls int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/projects/current/missions/plan" || r.URL.Query().Get("worktree_path") != scope.WorktreePath {
					t.Error("mission planner request lost its owned route")
				}
				_ = json.NewEncoder(w).Encode(result)
			}))
			defer server.Close()
			cl, err := NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			if _, err := cl.PlanMission(t.Context(), "test", scope, request); err == nil {
				t.Fatal("mismatched planner receipt accepted")
			}
			request.ProjectRevision = "changed"
			if _, err := cl.PlanMission(t.Context(), "changed", scope, request); err == nil || calls != 1 {
				t.Fatal("foreign request scope dispatched", calls, err)
			}
		})
	}
}

func TestMissionClientRejectsMutationReceiptsThatKeepOldApproval(t *testing.T) {
	scope, _, planning := missionClientFixture()
	result := planning.AgentMission
	result.Mission.Approved = true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(result)
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	request := MissionMutation{ProjectIdentity: scope.ProjectIdentity, Action: "create", Plan: &result.Mission.Plan}
	if _, err := cl.MutateMission(t.Context(), "create", scope, "", request); err == nil {
		t.Fatal("create inherited approval")
	}
	request.Action, request.Plan, request.PlanHash, request.ChildID = "review", nil, result.Mission.PlanHash, "helper"
	request.Confirm = true
	if _, err := cl.MutateMission(t.Context(), "review", scope, result.Mission.ID, request); err == nil {
		t.Fatal("child review became final approval")
	}
}
