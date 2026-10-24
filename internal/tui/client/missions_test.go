package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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
	control := MissionMutation{ProjectIdentity: identity, CatalogRevision: saved.CatalogRevision, PlanHash: saved.Mission.PlanHash, Action: "review", ChildID: "refine", Confirm: true}
	if err := cl.DoJSON(t.Context(), "review", "POST", base+"/"+saved.Mission.ID+query, control, &saved); err != nil || len(saved.Mission.Reviewed) != 1 {
		t.Fatal(saved, err)
	}
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
