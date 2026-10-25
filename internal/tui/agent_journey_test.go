package tui

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/client"
)

func TestAgentKeyboardJourneyInvestigationApprovalReviewAndApply(t *testing.T) {
	s := investigationState(t)
	s.Connected = true
	var requests, applied atomic.Int32
	cl := agentJourneyClient(t, &requests, &applied)
	enterJourneyCommand(s, "investigate")
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || s.PlanDraft.InvestigationState != "ready" || requests.Load() != 1 {
		t.Fatal("explicit investigation failed", s.Error)
	}
	enterJourneyCommand(s, "accept-investigation")
	if s.PlanDraft.InvestigationState != "accepted" || requests.Load() != 1 || len(s.PendingActions) != 0 {
		t.Fatal("accepting advice started work")
	}
	s.HandleKey(KeyEvent{Key: "g"})
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || s.Change == nil || s.Change.ID != "journey" || requests.Load() != 3 {
		t.Fatal("plan approval did not reach the checked task", s.Error, requests.Load())
	}
	s.AgentDetailOpen = true
	run := AgentRun{ID: "native", RootID: "native", ChangeSessionID: s.Change.ID, WorkflowID: s.Change.Workflow.ID, ProjectID: "p", ProjectRevision: "r", Status: "waiting_input"}
	s.applyAgentProgress(&agentProgressView{Request: *s.agentProgressRequest(), Detail: &AgentRunDetail{Run: run}, Status: "current"})
	for range 4 {
		s.HandleKey(KeyEvent{Key: "ArrowRight"})
		assertFrameSize(t, s.Render(52, 16), 52, 16)
	}
	if len(s.PendingActions) != 0 || applied.Load() != 0 {
		t.Fatal("execution inspection started an action")
	}
	enterJourneyCommand(s, "run-input approve")
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || s.Change.Workflow.Input.Status != "approved" || applied.Load() != 0 {
		t.Fatal("human input applied files or failed", s.Error)
	}
	s.HandleKey(KeyEvent{Key: "d"})
	if s.Page != PageEditor || !s.isViewingDiff() {
		t.Fatal("diff is not reachable after human input")
	}
	s.HandleKey(KeyEvent{Key: "a"})
	if len(s.PendingActions) != 0 || applied.Load() != 0 {
		t.Fatal("Apply bypassed per-file diff review")
	}
	s.HandleKey(KeyEvent{Key: "w"})
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || !allProposalFilesReviewed(s.Change) {
		t.Fatal("file review was not recorded", s.Error)
	}
	s.HandleKey(KeyEvent{Key: "a"})
	if s.Confirmation != nil {
		s.HandleKey(KeyEvent{Key: "Enter"})
	}
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || applied.Load() != 1 || s.ChangeReceipt == nil || s.ChangeReceipt.State != "applied" {
		t.Fatal("explicit reviewed Apply failed", s.Error)
	}
}

func enterJourneyCommand(s *State, command string) {
	s.HandleKey(KeyEvent{Key: ":"})
	s.HandleKey(KeyEvent{Text: command})
	s.HandleKey(KeyEvent{Key: "Enter"})
}

func agentJourneyClient(t *testing.T, requests, applied *atomic.Int32) *Client {
	t.Helper()
	identity := ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}
	change := ChangeSession{
		ID: "journey", ProjectIdentity: identity, Title: "Cover Run", Revision: 1,
		Hash: "proposal", State: "checked", Freshness: "current", Plan: &ChangePlan{Hash: "plan"},
		Checks: []Check{{Name: "tests", Required: true, State: "passed"}},
		Changes: []FileChange{{Path: "main.go", Content: "package main\n",
			Diff: FileDiff{Lines: []DiffLine{{Kind: "add", NewLine: 1, Text: "package main"}}}}},
		Workflow: &ChangeWorkflow{
			ID: "graph", Status: "awaiting_human_review",
			Stages: []ChangeWorkflowStage{
				{Name: "create", Status: "completed"}, {Name: "generate_tests", Status: "completed"},
				{Name: "verify", Status: "completed"}, {Name: "review", Status: "completed"},
				{Name: "human_review", Status: "waiting_input"},
			},
			Input: &client.ChangeInput{
				ProjectID: "p", ProjectRevision: "r", Revision: 1, Hash: "proposal",
				WorkflowID: "graph", RequestID: "request", RunID: "input",
				Status: "waiting", Action: "review_proposal",
				Scope: []client.ChangeInputScope{{Path: "main.go", SourceHash: "source-hash", ProposalHash: "proposed-hash"}},
			},
		},
	}
	return investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodPost {
			t.Errorf("unexpected journey request: %s %s", r.Method, r.URL)
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		var response any = change
		switch r.URL.Path {
		case "/api/projects/current/plans/investigate":
			response = investigationResult()
		case "/api/projects/current/changes":
			if err := json.Unmarshal(body["plan"], &change.Plan.ChangePlanInput); err != nil || len(change.Plan.Citations) != 1 {
				t.Error("accepted investigation evidence was not carried into the plan", err)
			}
			response = change
		case "/api/projects/current/changes/journey/workflow":
			if string(body["plan_hash"]) != `"plan"` {
				t.Error("generation lost approved plan identity")
			}
		case "/api/projects/current/agent-runs/input/input":
			if string(body["decision"]) != `"approve"` || string(body["request_id"]) != `"request"` || string(body["hash"]) != `"proposal"` {
				t.Error("human decision lost exact input binding")
			}
			change.Workflow.Input.Status, change.ReviewedHash = "approved", change.Hash
			response = change
		case "/api/projects/current/changes/journey/review-file":
			change.FileReviewHash, change.ReviewedFiles = change.Hash, []string{"main.go"}
			response = change
		case "/api/projects/current/changes/journey/review":
			if !allProposalFilesReviewed(&change) {
				t.Error("complete review preceded file review")
			}
		case "/api/projects/current/changes/journey/apply":
			if change.Workflow.Input.Status != "approved" || !allProposalFilesReviewed(&change) || string(body["confirm"]) != "true" || string(body["hash"]) != `"proposal"` {
				t.Error("Apply lost review or explicit consent")
			}
			applied.Add(1)
			response = ChangeMutation{SessionID: change.ID, ProjectIdentity: identity, Hash: change.Hash, State: "applied", UndoAvailable: true}
		default:
			t.Errorf("unexpected journey side effect: %s", r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Error(err)
		}
	})
}

func TestAgentIntegrationDisconnectRetainsWorktreeUntilExplicitRecovery(t *testing.T) {
	s := pendingWorktreeState()
	var mutations atomic.Int32
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			mutations.Add(1)
			http.Error(w, "integration response unavailable", http.StatusServiceUnavailable)
			return
		}
		switch r.URL.Path {
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(s.Project)
		case "/api/projects/current/git/workspace":
			_ = json.NewEncoder(w).Encode(s.GitWorkspace)
		case "/api/projects/current/index":
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: s.Project.ProjectIdentity})
		default:
			t.Errorf("unexpected recovery read: %s", r.URL)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	})
	s.HandleKey(KeyEvent{Key: "m"})
	if s.Confirmation != nil {
		s.HandleKey(KeyEvent{Key: "Enter"})
	}
	s.FlushActions(t.Context(), cl)
	if s.Error == "" || mutations.Load() != 1 || len(s.GitWorkspace.Worktrees) != 1 || len(s.CompletedFindings) != 0 {
		t.Fatal("unavailable integration response fabricated completion", s.Error)
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	s.HandleKey(KeyEvent{Key: "Enter"})
	assertFrameSize(t, s.Render(52, 16), 52, 16)
	if len(s.PendingActions) != 0 {
		t.Fatal("recovery inspection replayed a mutation")
	}
	s.HandleKey(KeyEvent{Key: "r"})
	s.FlushActions(t.Context(), cl)
	if s.Error != "" || mutations.Load() != 1 || len(s.GitWorkspace.Worktrees) != 1 || strings.Contains(s.CmdOutput, "completed") {
		t.Fatal("refresh retried integration or lost the retained checkout", s.Error)
	}
}
