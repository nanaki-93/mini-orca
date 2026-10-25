package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/api/handlers"
	"github.com/nanaki-93/mini-orca/v2/internal/app"
	"github.com/nanaki-93/mini-orca/v2/internal/app/agentstate"
	"github.com/nanaki-93/mini-orca/v2/internal/config"
	"github.com/nanaki-93/mini-orca/v2/internal/project"
)

func TestMissionWireContractUsesRealHandlersAndGuardedRoutes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc Run() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".mini-orca/\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-b", "main"}, {"add", "."}, {"commit", "-m", "Initial fixture"}} {
		flags := []string{"-C", root, "-c", "core.hooksPath=" + os.DevNull, "-c", "commit.gpgSign=false", "-c", "user.name=Test", "-c", "user.email=test@example.invalid"}
		if out, err := exec.CommandContext(t.Context(), "git", append(flags, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git fixture: %v %s", err, out)
		}
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
	mux.HandleFunc("GET /api/projects/current/missions/{missionID}/execution", h.Execution)
	mux.HandleFunc("POST /api/projects/current/missions/{missionID}/execution", h.ControlExecution)
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
	control.CatalogRevision, control.ChildID, control.Action = saved.CatalogRevision, "", "approve"
	approved, err := cl.MutateMission(t.Context(), "approve", guardedScope, saved.Mission.ID, control)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := service.GitRepository(t.Context(), identity.ProjectID, identity.ProjectRevision)
	if err != nil {
		t.Fatal(err)
	}
	prepare := MissionExecutionRequest{ProjectIdentity: identity, Action: "prepare", CatalogRevision: approved.CatalogRevision, PlanHash: approved.Mission.PlanHash, Base: &GitIdentity{Head: repo.Head, Branch: repo.Branch}, Confirm: true}
	target := base + "/" + saved.Mission.ID + "/execution" + query
	if absent, err := cl.MissionExecution(t.Context(), "unprepared", guardedScope, saved.Mission.ID); err != nil || absent != nil {
		t.Fatal("unprepared execution", absent, err)
	}
	var execution AgentMissionExecution
	if err := cl.DoJSON(t.Context(), "prepare", "POST", target, prepare, &execution); err != nil || execution.Stale || execution.Execution.Mission.ProjectIdentity != identity || execution.Execution.Mission.PlanHash != approved.Mission.PlanHash || execution.Execution.CatalogRevision != approved.CatalogRevision || execution.Execution.Revision != 1 || execution.Execution.State != "prepared" || execution.Execution.Base != *prepare.Base || len(execution.Execution.Children) != 1 || execution.Execution.Children[0].Limits != limits {
		t.Fatal("execution wire contract", execution, err)
	}
	if err := cl.DoJSON(t.Context(), "read-execution", "GET", target, nil, &execution); err != nil || execution.Execution.State != "prepared" {
		t.Fatal(execution, err)
	}
	if typed, err := cl.MissionExecution(t.Context(), "typed-execution", guardedScope, saved.Mission.ID); err != nil || typed.Execution.ID != execution.Execution.ID {
		t.Fatal("typed execution", typed, err)
	}
	prepare.Action, prepare.Base, prepare.CatalogRevision = "cancel", nil, 0
	prepare.ExecutionID, prepare.ExecutionRevision = execution.Execution.ID, execution.Execution.Revision
	canceled, err := cl.ControlMissionExecution(t.Context(), "cancel-execution", guardedScope, saved.Mission.ID, prepare)
	if err != nil || canceled.Execution.State != "canceled" || canceled.Execution.Revision != 2 {
		t.Fatal(canceled, err)
	}
	for _, method := range []string{"DELETE", "PUT", "PATCH"} {
		if _, err := cl.Request(t.Context(), "rejected-execution-method", method, target, `{}`); err == nil {
			t.Fatal("unimplemented execution method allowed", method)
		}
	}
	if _, err := cl.Request(t.Context(), "rejected-execution-nesting", "POST", base+"/"+saved.Mission.ID+"/execution/apply"+query, `{}`); err == nil {
		t.Fatal("execution route allowed nested mutation")
	}
}

func TestMissionSchedulerWireCarriesConsentOwnershipAndRetainedEvidence(t *testing.T) {
	identity := ProjectIdentity{ProjectID: "project", ProjectRevision: "revision"}
	planHash := strings.Repeat("a", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/projects/current/missions/mission/execution" || r.URL.Query().Get("worktree_path") != "/selected/checkout" {
			t.Error("lost selected mission checkout")
		}
		var request app.MissionExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if request.ProjectID != identity.ProjectID || request.ProjectRevision != identity.ProjectRevision || request.ExecutionID != "execution" || request.ExecutionRevision != 7 || request.PlanHash != planHash || !request.Confirm {
			t.Error("lost control identity", request)
		}
		if request.Action == "start" && (!request.ConfirmExecution || len(request.ConfirmedProfiles) != 1 || request.ConfirmedProfiles[0] != "code") {
			t.Error("lost start consent")
		}
		if request.Action != "start" && (request.ConfirmExecution || len(request.ConfirmedProfiles) != 0) {
			t.Error("stop control leaked start grants")
		}
		response := app.AgentMissionExecution{SourceStatus: "current", OwnerState: "active", ReadyChildren: []string{}, Execution: agentstate.MissionExecution{ID: "execution", Revision: 8, State: "running", ElapsedMillis: 123, Failure: "Retained failure", Wave: &agentstate.MissionWave{ID: "wave", Owner: "owner", Children: []string{"child"}, StartedAt: time.Now().UTC(), AllowanceMillis: 60000}, Children: []agentstate.MissionExecutionChild{{ID: "child", State: "running", WaveID: "wave", Directory: "execution-child", Branch: "codex/execution-child", BaseHead: strings.Repeat("a", 40), ProjectID: "child-project", ProjectRevision: "child-revision", SessionID: "proposal", WorkflowID: "workflow", ProposalHash: "hash", ProposalRevision: 3, Reason: "Retained reason", UpdatedAt: time.Now().UTC()}}}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	for _, action := range []string{"start", "pause", "cancel", "reconcile"} {
		request := MissionExecutionRequest{ProjectIdentity: identity, Action: action, ExecutionID: "execution", ExecutionRevision: 7, PlanHash: planHash, Confirm: true}
		if action == "start" {
			request.ConfirmExecution = true
			request.ConfirmedProfiles = []string{"code"}
		}
		var response AgentMissionExecution
		if err := cl.DoJSON(t.Context(), action, "POST", "/api/projects/current/missions/mission/execution?worktree_path=%2Fselected%2Fcheckout", request, &response); err != nil {
			t.Fatal(err)
		}
		if response.OwnerState != "active" || response.ReadyChildren == nil || response.Execution.Wave == nil || response.Execution.Wave.ID != "wave" || response.Execution.Wave.AllowanceMillis != 60000 || response.Execution.ElapsedMillis != 123 || response.Execution.Failure != "Retained failure" || len(response.Execution.Children) != 1 {
			t.Fatal("lost execution receipt", response)
		}
		child := response.Execution.Children[0]
		if child.ProjectID != "child-project" || child.ProjectRevision != "child-revision" || child.SessionID != "proposal" || child.WorkflowID != "workflow" || child.ProposalHash != "hash" || child.ProposalRevision != 3 || child.Directory != "execution-child" || child.State != "running" || child.WaveID != "wave" || child.Reason != "Retained reason" || child.UpdatedAt == "" {
			t.Fatal("lost child evidence", child)
		}
	}
}

func TestMissionIntegrationWirePreservesCheckpointArchiveAndGitOutcome(t *testing.T) {
	identity := ProjectIdentity{ProjectID: "project", ProjectRevision: "before"}
	base := GitIdentity{Head: strings.Repeat("a", 40), Branch: "main"}
	source := GitIdentity{Head: strings.Repeat("b", 40), Branch: "codex/child"}
	planHash, proposalHash := strings.Repeat("a", 64), "sha256:"+strings.Repeat("c", 64)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/projects/current/missions/mission/execution" || r.URL.Query().Get("worktree_path") != "/selected/root" {
			t.Error("integration lost selected checkout")
		}
		var request app.MissionExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		input := request.Integration
		if !request.Confirm || !request.ConfirmExecution || request.Action != "integrate" || request.ExecutionID != "execution" || request.ExecutionRevision != 7 || request.PlanHash != planHash || request.Base == nil || request.Base.Head != base.Head || input == nil {
			t.Error("lost exact integration admission", request)
			return
		}
		if input.ChildID != "child" || input.Path != "/selected/root/.worktrees/child" || input.Source.Head != source.Head || input.Source.Branch != source.Branch || input.ProposalHash != proposalHash || input.ProposalRevision != 3 || input.WorkflowID != "workflow" || input.SessionID != "proposal" || input.IntegrationID != "" {
			t.Error("lost committed proposal", input)
		}
		before := agentstate.MissionCheckpoint{ProjectRevision: "before", Base: agentstate.MissionBase{Head: base.Head, Branch: base.Branch}, Sources: []agentstate.MissionSource{{Path: "new.go"}}, Instructions: map[string]string{"new.go": "instructions"}}
		after := before
		after.ProjectRevision, after.Base.Head = "after", source.Head
		after.Sources = []agentstate.MissionSource{{Path: "new.go", Exists: true, Hash: proposalHash}}
		evidence := agentstate.MissionProposalEvidence{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca-missions", User: "local", ID: "execution"}, Name: "child-proposal.json"}, Version: 4, Hash: strings.Repeat("d", 64)}
		entry := agentstate.MissionIntegration{ID: "integration", ChildID: "child", Owner: "owner", State: "merged", Source: input.Source, SessionID: input.SessionID, WorkflowID: input.WorkflowID, ProposalHash: input.ProposalHash, ProposalRevision: input.ProposalRevision, Evidence: evidence, Before: before, After: &after, Cleanup: "worktree_removed", Failure: "Branch retained", StartedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		response := app.AgentMissionExecution{SourceStatus: "current", OwnerState: "idle", ReadyChildren: []string{}, Execution: agentstate.MissionExecution{ID: "execution", Revision: 10, Base: before.Base, Current: after, Integrations: []agentstate.MissionIntegration{entry}, State: "completed", Children: []agentstate.MissionExecutionChild{{ID: "child", State: "integrated"}}}, Git: &app.GitMutationResult{State: "merged", RemovedWorktree: input.Path, Warnings: []string{"Branch retained"}}}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	request := MissionExecutionRequest{ProjectIdentity: identity, Action: "integrate", PlanHash: planHash, ExecutionID: "execution", ExecutionRevision: 7, Base: &base, Confirm: true, ConfirmExecution: true, Integration: &MissionIntegrationRequest{ChildID: "child", SessionID: "proposal", WorkflowID: "workflow", ProposalHash: proposalHash, ProposalRevision: 3, Path: "/selected/root/.worktrees/child", Source: source}}
	var result AgentMissionExecution
	if err := cl.DoJSON(t.Context(), "integrate", "POST", "/api/projects/current/missions/mission/execution?worktree_path=%2Fselected%2Froot", request, &result); err != nil {
		t.Fatal(err)
	}
	if result.Execution.Current.ProjectRevision != "after" || result.Execution.Base != base || result.Execution.Current.Base.Head != source.Head || len(result.Execution.Integrations) != 1 || result.Execution.Children[0].State != "integrated" || result.Git == nil || len(result.Git.Warnings) != 1 {
		t.Fatal("lost integration checkpoint or partial cleanup", result)
	}
	entry := result.Execution.Integrations[0]
	if entry.State != "merged" || entry.Cleanup != "worktree_removed" || entry.After == nil || entry.After.Sources[0].Hash != proposalHash || entry.Before.Sources[0].Exists || entry.Evidence.Key.Session.ID != "execution" || entry.Evidence.Key.Name != "child-proposal.json" || entry.Evidence.Version != 4 || entry.Evidence.Hash != strings.Repeat("d", 64) || entry.ProposalRevision != 3 || entry.StartedAt == "" || entry.Failure != "Branch retained" {
		t.Fatal("lost retained integration evidence", entry)
	}
}

func TestMissionRevalidationWirePreservesFreshCandidateAndHistoricalEvidence(t *testing.T) {
	identity := ProjectIdentity{ProjectID: "project", ProjectRevision: "current"}
	base := GitIdentity{Head: strings.Repeat("b", 40), Branch: "main"}
	previous := MissionIntegrationRequest{ChildID: "child", SessionID: "old-session", WorkflowID: "historical-model", ProposalHash: "sha256:" + strings.Repeat("a", 64), ProposalRevision: 7, Path: "/selected/.worktrees/original", Source: GitIdentity{Head: strings.Repeat("a", 40), Branch: "codex/original"}}
	request := MissionExecutionRequest{ProjectIdentity: identity, Action: "revalidate", PlanHash: strings.Repeat("c", 64), ExecutionID: "execution", ExecutionRevision: 12, Base: &base, Confirm: true, ConfirmExecution: true, Revalidation: &MissionRevalidationRequest{Previous: previous}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/projects/current/missions/mission/execution" || r.URL.Query().Get("worktree_path") != "/selected" {
			t.Error("revalidation lost owned route")
		}
		var input app.MissionExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		if input.Action != "revalidate" || input.Revalidation == nil || input.Integration != nil || len(input.ConfirmedProfiles) != 0 || !input.ConfirmExecution || input.ExecutionRevision != 12 {
			t.Error("changed revalidation admission", input)
			return
		}
		old := input.Revalidation.Previous
		if old.SessionID != previous.SessionID || old.WorkflowID != previous.WorkflowID || old.ProposalRevision != 7 || old.Source.Head != previous.Source.Head || old.Path != previous.Path || input.Revalidation.RevalidationID != "" {
			t.Error("lost old candidate identity", old)
		}
		now := time.Now().UTC()
		entry := agentstate.MissionRevalidation{ID: "revalidation-owned", Owner: "owner", State: "awaiting_review", Previous: agentstate.MissionExecutionChild{ID: old.ChildID, SessionID: old.SessionID, WorkflowID: old.WorkflowID, ProposalHash: old.ProposalHash, ProposalRevision: old.ProposalRevision}, Candidate: agentstate.MissionExecutionChild{ID: "child", Generation: 1, State: "awaiting_review", SessionID: "fresh-session", BaseHead: base.Head, Directory: "execution-child-r1", Branch: "codex/execution-child-r1", ProposalRevision: 1}, Source: old.Source, Before: agentstate.MissionCheckpoint{ProjectRevision: "current", Base: *input.Base}, AllowanceMillis: 15000, StartedAt: now, UpdatedAt: now}
		entry.Evidence = agentstate.MissionProposalEvidence{Key: agentstate.ArtifactKey{Session: agentstate.Key{App: "mini-orca-missions", User: "local", ID: "execution"}, Name: "child-proposal.json"}, Hash: strings.Repeat("e", 64), Version: 2}
		result := app.AgentMissionExecution{SourceStatus: "current", OwnerState: "idle", ReadyChildren: []string{}, Execution: agentstate.MissionExecution{ID: "execution", Revision: 15, Current: entry.Before, Revalidations: []agentstate.MissionRevalidation{entry}, Children: []agentstate.MissionExecutionChild{entry.Candidate}, State: "awaiting_review", ElapsedMillis: 2000}}
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Error(err)
		}
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	var result AgentMissionExecution
	if err := cl.DoJSON(t.Context(), "revalidate", "POST", "/api/projects/current/missions/mission/execution?worktree_path=%2Fselected", request, &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Execution.Revalidations) != 1 || result.Execution.Children[0].Generation != 1 || result.Execution.Current.Base != base || result.Git != nil {
		t.Fatal("lost candidate receipt", result)
	}
	entry := result.Execution.Revalidations[0]
	if entry.Previous.WorkflowID != "historical-model" || entry.Candidate.WorkflowID != "" || entry.Candidate.SessionID != "fresh-session" || entry.Candidate.BaseHead != base.Head || entry.Evidence.Version != 2 || entry.AllowanceMillis != 15000 || entry.StartedAt == "" {
		t.Fatal("historical evidence became current authority", entry)
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
		_ = json.NewEncoder(w).Encode(MissionPlanningResult{AgentMission: AgentMission{CatalogRevision: 8, Mission: MissionRecord{ProjectIdentity: identity, ID: "mission-fixture", PlanHash: strings.Repeat("a", 64)}, SourceStatus: "current"}, InputHash: strings.Repeat("b", 64), PromptVersion: "mission-planner-v1", PlanningLimits: limits, Usage: AgentUsage{ModelCalls: 1, UnknownUsageCalls: 1, InputTokens: 1024}, ContextManifest: ContextManifest{Scope: "code", TokenLimit: 8192}})
	}))
	defer server.Close()
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	target := "/api/projects/current/missions/plan?" + url.Values{"project_id": {identity.ProjectID}, "project_revision": {identity.ProjectRevision}, "worktree_path": {"/tmp/selected checkout"}}.Encode()
	var result MissionPlanningResult
	if err := cl.DoJSON(t.Context(), "planning", "POST", target, request, &result); err != nil || result.CatalogRevision != 8 || result.Mission.ProjectIdentity != identity || result.Mission.Approved || result.Usage.UnknownUsageCalls != 1 || result.ContextManifest.Scope != "code" {
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
