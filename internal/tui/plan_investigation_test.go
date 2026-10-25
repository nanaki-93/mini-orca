package tui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func connectionInvestigationState(t *testing.T) *State {
	t.Helper()
	s := connectionsTestState()
	model := s.Models.Scopes["code"]
	model.Capabilities.Tools = true
	s.Models.Scopes["code"] = model
	s.Index = &ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity, Files: []IndexedFile{{Path: "main.go", ContentHash: "captured-source"}}}
	s.openTaskPlan(&ChangeSeed{Title: "Review Run", Paths: []string{"main.go", "main_test.go"}, AcceptanceCriteria: []string{"Preserve behavior"}, Plan: &ChangePlanInput{Problem: "Missing coverage", Approach: "Inspect Run"}})
	s.Navigate(PageConnections)
	s.togglePlanConnection()
	if len(s.PlanDraft.Connections) != 1 || len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatal("connection selection dispatched work or failed", s.Error)
	}
	return s
}

func TestPlanConnectionConsentCapturesOnlySelectedGrantsAndSources(t *testing.T) {
	s := connectionInvestigationState(t)
	s.SelectedIdx = 1
	s.togglePlanConnection()
	s.Navigate(PagePlan)
	s.startPlanInvestigation(false)
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("local provider implicitly approved external source sharing")
	}
	for _, token := range []string{"https://mcp.example", "https://api.example", "main.go", "main_test.go", "admitted project context"} {
		if !strings.Contains(s.Confirmation.Message, token) {
			t.Fatal("source-sharing confirmation omitted destination or scope", token)
		}
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Target != "local" {
		t.Fatal("tool sharing fabricated remote-provider consent")
	}
	task, ok := s.preparePlanInvestigationTask(s.PendingActions[0])
	if !ok {
		t.Fatal(s.Error)
	}
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/current/plans/investigate" {
			t.Errorf("unexpected external dispatch: %s", r.URL)
		}
		var request PlanInvestigationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if len(request.Integrations) != 2 || request.Integrations[0] != (client.IntegrationUse{ID: "docs", Grant: "grant-mcp", SchemaFingerprint: "schema-mcp", ConfirmSourceSharing: true}) || request.Integrations[1].ID != "catalog" || request.ConfirmRemoteProvider || request.Sources[0].Hash != "captured-source" {
			t.Errorf("lost per-investigation authority: %+v", request)
		}
		value := investigationResult()
		value.ProjectIdentity = request.ProjectIdentity
		_ = json.NewEncoder(w).Encode(value)
	})
	result := executePlanInvestigation(t.Context(), cl, task)
	if result.err != nil {
		t.Fatal(result.err)
	}
	s.applyPlanInvestigation(result)
	if s.PlanDraft.InvestigationState != "ready" {
		t.Fatal("admitted investigation did not complete")
	}
	seed := copyTaskPlanSeed(s.PlanDraft.Seed)
	s.openTaskPlan(&seed)
	if len(s.PlanDraft.Connections) != 0 {
		t.Fatal("new task inherited an earlier source-sharing selection")
	}
}

func TestPlanConnectionsRejectChangedGrantExpiryProviderAndCheckout(t *testing.T) {
	for _, mutate := range []func(*State){
		func(s *State) { s.AgentConnections.Records[client.ConnectionMCP][0].Grant = "changed" },
		func(s *State) {
			expired := time.Unix(0, 0)
			s.AgentConnections.Records[client.ConnectionMCP][0].ExpiresAt = &expired
		},
		func(s *State) {
			model := s.Models.Scopes["code"]
			model.ProviderOrigin = "https://different.example"
			s.Models.Scopes["code"] = model
		},
		func(s *State) { s.PlanDraft.Seed.WorktreePath = "/repo/other" },
		func(s *State) { s.PlanDraft.Seed.Plan.Approach = "A different plan" },
	} {
		s := connectionInvestigationState(t)
		s.startPlanInvestigation(false)
		confirmation := s.Confirmation
		mutate(s)
		confirmation.OnConfirm(s)
		if len(s.PendingActions) != 0 || s.Error == "" {
			t.Fatal("changed authority reused old source-sharing consent")
		}
	}
	s := connectionInvestigationState(t)
	s.startPlanInvestigation(false)
	s.HandleKey(KeyEvent{Key: "Enter"})
	model := s.Models.Scopes["code"]
	model.Model = "changed-after-approval"
	s.Models.Scopes["code"] = model
	if _, ok := s.preparePlanInvestigationTask(s.PendingActions[0]); ok {
		t.Fatal("queued investigation retargeted after approval")
	}
}

func TestPlanConnectionsLimitAndIncomingAccessCannotBecomeTools(t *testing.T) {
	s := connectionInvestigationState(t)
	s.SelectedIdx = 1
	s.togglePlanConnection()
	third := s.AgentConnections.Records[client.ConnectionMCP][0]
	third.Config.ID = "third"
	s.AgentConnections.Records[client.ConnectionMCP] = append(s.AgentConnections.Records[client.ConnectionMCP], third)
	s.SelectedIdx = 1
	s.togglePlanConnection()
	if len(s.PlanDraft.Connections) != 2 || !strings.Contains(s.Error, "at most two") {
		t.Fatal("combined connection limit ignored")
	}
	s.PlanDraft.Connections = nil
	for index, row := range s.connectionRows() {
		if row.Key.Kind == client.ConnectionRemote || row.Key.Kind == client.ConnectionAccess {
			s.SelectedIdx = index
			s.togglePlanConnection()
			if len(s.PlanDraft.Connections) != 0 || !strings.Contains(s.Error, "only MCP and OpenAPI") {
				t.Fatal("remote or inbound access was treated as tool authority")
			}
		}
	}
}

func TestInvestigationConnectionGrantCannotFollowChangedWorktreeIndex(t *testing.T) {
	s := connectionInvestigationState(t)
	child := ProjectIdentity{ProjectID: "child", ProjectRevision: "child-rev"}
	path := "/repo/task"
	s.Change = &ChangeSession{ID: "task", ProjectIdentity: child, WorktreePath: path}
	s.PlanDraft.Seed.WorktreePath = path
	s.PlanDraft.Connections = nil
	s.AgentConnections.Scope = client.AgentSettingsScope{ProjectIdentity: child, WorktreePath: path}
	s.AgentConnections.Path = path
	s.togglePlanConnection()
	s.startPlanInvestigation(false)
	s.HandleKey(KeyEvent{Key: "Enter"})
	task, ok := s.preparePlanInvestigationTask(s.PendingActions[0])
	if !ok {
		t.Fatal(s.Error)
	}
	calls := 0
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/projects/current/index" || r.URL.Query().Get("worktree_path") != path {
			t.Errorf("changed scope reached provider admission: %s", r.URL)
		}
		_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: ProjectIdentity{ProjectID: "other", ProjectRevision: "other-revision"}})
	})
	result := executePlanInvestigation(t.Context(), cl, task)
	if result.err == nil || calls != 1 {
		t.Fatal("old connection grant followed a new indexed checkout")
	}
}

func TestInvestigationConnectionsRequireAdvertisedToolCapability(t *testing.T) {
	s := connectionInvestigationState(t)
	model := s.Models.Scopes["code"]
	model.Capabilities.Tools = false
	s.Models.Scopes["code"] = model
	s.startPlanInvestigation(false)
	if s.Confirmation != nil || len(s.PendingActions) != 0 || !strings.Contains(s.Error, "tool support") {
		t.Fatal("unknown adapter capability admitted integration tools")
	}
}

func investigationState(t *testing.T) *State {
	t.Helper()
	s := NewState()
	identity := ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}
	s.Project = &Project{ProjectIdentity: identity}
	s.Index = &ProjectIndex{ProjectIdentity: identity, Files: []IndexedFile{{Path: "main.go", ContentHash: "source-hash", LineCount: 3}}}
	s.Models = &ModelCatalog{Scopes: map[string]Model{"code": {Model: "fixture"}}}
	s.openTaskPlan(&ChangeSeed{Title: "Improve Run", Paths: []string{"main.go"}, AcceptanceCriteria: []string{"Preserve behavior"}, Plan: &ChangePlanInput{Problem: "Missing coverage", Approach: "Inspect Run"}})
	if len(s.PendingActions) != 0 {
		t.Fatal("opening a draft dispatched work")
	}
	return s
}

func investigationResult() PlanInvestigation {
	return PlanInvestigation{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, Version: "v1", InputHash: "input-hash", Approach: "Cover Run's boundary cases", Evidence: []PlanEvidence{{Path: "main.go", Hash: "source-hash", StartLine: 1, EndLine: 1, Quote: "package main", Observation: "Run belongs to the main package"}}, Assumptions: []string{"Keep public behavior"}, SuggestedScope: []string{"helper.go"}, AcceptanceCriteria: []string{"Boundary cases are covered"}}
}

func investigationClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	cl, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

func pendingInvestigation(t *testing.T, s *State) planInvestigationTask {
	t.Helper()
	s.ExecuteCommand(":investigate")
	s.ExecuteCommand(":investigate")
	if len(s.PendingActions) != 1 {
		t.Fatalf("investigation must dispatch once: %+v %s", s.PendingActions, s.Error)
	}
	task, ok := s.preparePlanInvestigationTask(s.PendingActions[0])
	s.PendingActions = nil
	if !ok {
		t.Fatal(s.Error)
	}
	return task
}

func TestPlanInvestigationRequiresAcceptanceAndPreservesExplicitScope(t *testing.T) {
	s := investigationState(t)
	before := planDraftFingerprint(s.PlanDraft)
	paths := append([]string(nil), s.PlanDraft.Seed.Paths...)
	s.PlanDraft.Saved = &ChangeSession{Plan: &ChangePlan{ApprovedHash: "approved"}}
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/projects/current/plans/investigate" {
			t.Errorf("unexpected side effect: %s %s", r.Method, r.URL)
		}
		var request PlanInvestigationRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.ProjectIdentity != s.Project.ProjectIdentity || request.ConfirmRemoteProvider || len(request.Sources) != len(paths) || request.Sources[0].Hash != "source-hash" || !request.Sources[0].Exists || request.Sources[1].Exists {
			t.Errorf("incorrect captured scope: %+v", request)
		}
		_ = json.NewEncoder(w).Encode(investigationResult())
	})
	task := pendingInvestigation(t, s)
	result := executePlanInvestigation(t.Context(), cl, task)
	if s.PlanDraft.InvestigationState != "running" || s.PlanDraft.Investigation != nil {
		t.Fatal("background request mutated UI state")
	}
	s.applyPlanInvestigation(result)
	if s.PlanDraft.InvestigationState != "ready" || planDraftFingerprint(s.PlanDraft) != before {
		t.Fatalf("unaccepted result changed draft: %s", s.Error)
	}
	s.ExecuteCommand(":accept-investigation")
	if s.PlanDraft.InvestigationState != "accepted" || s.PlanDraft.Seed.Plan.Approach != result.value.Approach || len(s.PlanDraft.Seed.Plan.Citations) != 1 || len(s.PlanDraft.Seed.Plan.Assumptions) != 1 || !reflect.DeepEqual(paths, s.PlanDraft.Seed.Paths) || len(s.PendingActions) != 0 || s.PlanDraft.Saved.Plan.ApprovedHash != "" {
		t.Fatal("acceptance lost evidence, expanded scope, dispatched or retained approval")
	}
	if s.PlanDraft.Seed.AcceptanceCriteria[0] != result.value.AcceptanceCriteria[0] || s.Change != nil {
		t.Fatal("acceptance did not remain an editable draft")
	}
}

func TestPlanInvestigationRemoteConfirmationBindsCurrentDraft(t *testing.T) {
	for _, edited := range []bool{false, true} {
		s := investigationState(t)
		s.Models.Scopes["code"] = Model{Model: "remote", RemoteProvider: true, ProviderOrigin: "https://provider.example"}
		s.HandleKey(KeyEvent{Key: "i"})
		if s.Confirmation == nil || len(s.PendingActions) != 0 || s.PlanDraft.InvestigationState != "" {
			t.Fatal("remote investigation bypassed confirmation")
		}
		if edited {
			s.PlanDraft.Seed.Paths = append(s.PlanDraft.Seed.Paths, "another.go")
		}
		s.Confirmation.OnConfirm(s)
		if edited {
			if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "changed") {
				t.Fatal("confirmation admitted a different scope")
			}
		} else if len(s.PendingActions) != 1 || s.PendingActions[0].Target != "remote-confirmed" {
			t.Fatal("explicit remote confirmation was lost")
		}
	}
}

func TestPlanInvestigationDiscardsLateResultsAndRetainsFailedDraft(t *testing.T) {
	for _, scenario := range []string{"edit", "project", "replacement", "failure"} {
		t.Run(scenario, func(t *testing.T) {
			s := investigationState(t)
			task := pendingInvestigation(t, s)
			result := planInvestigationResult{task: task, value: investigationResult()}
			switch scenario {
			case "edit":
				s.ExecuteCommand(":approach User's revised approach")
			case "project":
				s.Project.ProjectRevision = "next"
			case "replacement":
				s.openTaskPlan(&ChangeSeed{Title: "Different task", Paths: []string{"other.go"}})
			case "failure":
				result.err = errors.New("provider unavailable")
			}
			before := planDraftFingerprint(s.PlanDraft)
			s.applyPlanInvestigation(result)
			if s.PlanDraft.Investigation != nil || planDraftFingerprint(s.PlanDraft) != before || s.PlanDraft.Submitted {
				t.Fatal("late or failed result changed current draft")
			}
			s.acceptPlanInvestigation()
			if planDraftFingerprint(s.PlanDraft) != before {
				t.Fatal("stale suggestions were accepted")
			}
		})
	}
}

func TestPlanInvestigationCancellationReachesHTTPAndRejectsLateResult(t *testing.T) {
	s := investigationState(t)
	started := make(chan struct{})
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
	})
	task := pendingInvestigation(t, s)
	done := make(chan planInvestigationResult, 1)
	go func() { done <- executePlanInvestigation(t.Context(), cl, task) }()
	<-started
	s.ExecuteCommand(":cancel-investigation")
	s.FlushActions(t.Context(), cl)
	result := <-done
	if !errors.Is(result.err, context.Canceled) {
		t.Fatal(result.err)
	}
	s.applyPlanInvestigation(result)
	s.applyPlanInvestigation(planInvestigationResult{task: task, value: investigationResult()})
	if s.PlanDraft.InvestigationState != "canceled" || s.PlanDraft.Investigation != nil || s.PlanDraft.Seed.Plan.Approach != "Inspect Run" {
		t.Fatal("cancellation lost the draft or accepted a late result")
	}
}

func TestPlanInvestigationWorktreeUsesItsCapturedIdentity(t *testing.T) {
	s := investigationState(t)
	s.PlanDraft.Seed.WorktreePath = "/private/work tree"
	worktree := ProjectIdentity{ProjectID: "worktree", ProjectRevision: "worktree-revision"}
	requests := 0
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("worktree_path") != s.PlanDraft.Seed.WorktreePath {
			t.Error("worktree scope lost")
		}
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/index") {
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: worktree, Files: s.Index.Files})
			return
		}
		var body PlanInvestigationRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ProjectIdentity != worktree {
			t.Errorf("worktree identity lost: %+v %v", body, err)
		}
		response := investigationResult()
		response.ProjectIdentity = worktree
		_ = json.NewEncoder(w).Encode(response)
	})
	task := pendingInvestigation(t, s)
	s.applyPlanInvestigation(executePlanInvestigation(t.Context(), cl, task))
	if requests != 2 || s.PlanDraft.InvestigationState != "ready" || s.Project.ProjectID != "p" {
		t.Fatalf("worktree investigation changed main project: %s", s.Error)
	}
}

func TestPlanInvestigationFramesStayBoundedAndPassive(t *testing.T) {
	s := investigationState(t)
	task := pendingInvestigation(t, s)
	s.applyPlanInvestigation(planInvestigationResult{task: task, value: investigationResult()})
	for _, palette := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s.Palette = palette
		for _, dims := range [][2]int{{52, 16}, {80, 24}, {100, 20}, {160, 40}} {
			for section := 0; section < 4; section++ {
				s.DetailSection = section
				assertFrameSize(t, s.Render(dims[0], dims[1]), dims[0], dims[1])
			}
		}
	}
	if len(s.PendingActions) != 0 || s.PlanDraft.InvestigationState != "ready" {
		t.Fatal("viewing suggestions dispatched or accepted them")
	}
}

func TestPlanInvestigationBubbleCommandOwnsBackgroundCompletion(t *testing.T) {
	s := investigationState(t)
	calls := 0
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(investigationResult())
	})
	task := pendingInvestigation(t, s)
	m := NewBubbleModel(t.Context(), s, cl, 100, 30)
	command, handled := m.prepareAsyncActionCmd("request", task.action)
	if !handled || command == nil || calls != 0 || m.activeAgentStarts != 1 {
		t.Fatal("preparing asynchronous command dispatched or lost activity")
	}
	message := command()
	if calls != 1 || s.PlanDraft.InvestigationState != "running" {
		t.Fatal("background command published outside Update")
	}
	m.Update(message)
	if s.PlanDraft.InvestigationState != "ready" || m.activeAgentStarts != 0 {
		t.Fatal("Update did not publish completed suggestions")
	}
}

func TestPlanInvestigationAcceptedCitationsFollowScopeAndReplanning(t *testing.T) {
	for _, command := range []string{":scope other.go", ":context", ":replan"} {
		t.Run(command, func(t *testing.T) {
			s := investigationState(t)
			if command == ":context" {
				s.PlanDraft.Seed.Plan.ReadPaths = []string{"README.md"}
			}
			task := pendingInvestigation(t, s)
			value := investigationResult()
			if command == ":context" {
				value.Evidence[0].Path = "README.md"
			}
			s.applyPlanInvestigation(planInvestigationResult{task: task, value: value})
			s.acceptPlanInvestigation()
			s.ExecuteCommand(command)
			if len(s.PlanDraft.Seed.Plan.Citations) != 0 || s.PlanDraft.Seed.Plan.Evidence != "" || s.PlanDraft.Submitted || len(s.PendingActions) != 0 {
				t.Fatal("scope recovery retained obsolete citations or dispatched work")
			}
			s.ExecuteCommand(":investigate")
			if s.PlanDraft.InvestigationState != "running" || len(s.PendingActions) != 1 {
				t.Fatalf("revised plan cannot be investigated: %s", s.Error)
			}
		})
	}
}
