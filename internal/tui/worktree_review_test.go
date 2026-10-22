package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func recoveryState() *State {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	s.Repository = &GitRepository{ProjectIdentity: s.Project.ProjectIdentity, GitIdentity: GitIdentity{Head: "target", Branch: "main"}}
	s.GitWorkspace = &GitWorkspace{GitRepository: *s.Repository, Worktrees: []GitWorktree{{Path: "/repo/task", GitIdentity: GitIdentity{Head: "source", Branch: "fix"}, Integration: "merged"}}}
	s.Navigate(PageGit)
	return s
}

func TestCleanupOffersExplicitDiscardForUnfinishedWorktrees(t *testing.T) {
	for _, status := range []string{"draft", "failed", "interrupted", "running"} {
		t.Run(status, func(t *testing.T) {
			s := recoveryState()
			wt := &s.GitWorkspace.Worktrees[0]
			wt.Sessions = []ChangeHistoryEntry{{ID: "old-plan", State: "draft", WorkflowStatus: status, WorktreePath: wt.Path}}
			s.Change = &ChangeSession{ID: "old-plan", State: "draft", WorktreePath: wt.Path}
			s.ActiveChanges = []*ChangeSession{s.Change}
			s.ChangeHistory = append([]ChangeHistoryEntry(nil), wt.Sessions...)
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/projects/current/git/discard-worktree" || r.Method != http.MethodPost {
					t.Errorf("wrong removal route: %s %s", r.Method, r.URL.Path)
				}
				var intent WorktreeIntent
				if err := json.NewDecoder(r.Body).Decode(&intent); err != nil || intent.SourceHead != "source" || intent.SourceBranch != "fix" || intent.Head != "target" || !intent.Confirm {
					t.Errorf("discard lost captured identity/consent: %+v %v", intent, err)
				}
				_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "discarded", RemovedWorktree: "/repo/task"})
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			s.HandleKey(KeyEvent{Key: "c"})
			if s.Confirmation == nil || !s.Confirmation.Destructive || len(s.PendingActions) != 0 {
				t.Fatal("unfinished task blocked cleanup or was discarded without consent")
			}
			s.HandleKey(KeyEvent{Key: "Escape"})
			if len(s.PendingActions) != 0 || len(s.ActiveChanges) != 1 {
				t.Fatal("canceling cleanup altered the task")
			}
			s.HandleKey(KeyEvent{Key: "c"})
			wt.Head = "later-source"
			s.HandleKey(KeyEvent{Key: "y"})
			if len(s.ActiveChanges) != 1 || len(s.GitWorkspace.Worktrees) != 1 {
				t.Fatal("discard hid task before its removal receipt")
			}
			s.FlushActions(context.Background(), cl)
			if s.Error != "" || calls != 1 || len(s.GitWorkspace.Worktrees) != 0 || len(s.ActiveChanges) != 0 || len(s.ChangeHistory) != 0 || s.Change != nil {
				t.Fatalf("confirmed discard failed: error=%s calls=%d tasks=%v", s.Error, calls, s.ActiveChanges)
			}
		})
	}
}

func TestCleanupDiscardRetainsTasksOnFailureOrProjectChange(t *testing.T) {
	for _, outcome := range []string{"error", "no receipt", "project changed"} {
		t.Run(outcome, func(t *testing.T) {
			s := recoveryState()
			s.GitWorkspace.Worktrees[0].Integration = "behind"
			s.Change = &ChangeSession{ID: "old-plan", WorktreePath: "/repo/task"}
			s.ActiveChanges = []*ChangeSession{s.Change}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if outcome == "error" {
					http.Error(w, "worktree changed", http.StatusConflict)
					return
				}
				_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "discarded"})
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			s.HandleKey(KeyEvent{Key: "c"})
			s.HandleKey(KeyEvent{Key: "y"})
			if outcome == "project changed" {
				s.Project.ProjectID = "other"
			}
			s.FlushActions(context.Background(), cl)
			if s.Error == "" || len(s.GitWorkspace.Worktrees) != 1 || len(s.ActiveChanges) != 1 || (outcome == "project changed" && calls != 0) {
				t.Fatalf("failed discard lost task or crossed project scope: %s, calls=%d", s.Error, calls)
			}
		})
	}
}

func TestCleanupUsesCapturedIdentityAndWaitsForRemovalReceipt(t *testing.T) {
	for _, outcome := range []string{"removed", "warning", "stale"} {
		t.Run(outcome, func(t *testing.T) {
			s := recoveryState()
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != "POST" || r.URL.Path != "/api/projects/current/git/cleanup-merged" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
				}
				var intent WorktreeIntent
				if err := json.NewDecoder(r.Body).Decode(&intent); err != nil {
					t.Error(err)
				}
				if intent.Head != "target" || intent.SourceHead != "source" || intent.SourceBranch != "fix" {
					t.Errorf("replaced reviewed identity: %+v", intent)
				}
				switch outcome {
				case "stale":
					http.Error(w, "state changed", http.StatusConflict)
				case "warning":
					_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "merged", Warnings: []string{"Worktree retained: user changes"}})
				default:
					_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "merged", RemovedWorktree: "/repo/task"})
				}
			}))
			defer srv.Close()
			cl, err := NewClient(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer cl.Close()
			s.HandleKey(KeyEvent{Key: "c"})
			if len(s.GitWorkspace.Worktrees) != 1 || len(s.PendingActions) != 1 {
				t.Fatal("cleanup hid worktree optimistically")
			}
			s.GitWorkspace.Worktrees[0].Head = "later-source"
			s.FlushActions(context.Background(), cl)
			if calls != 1 {
				t.Fatalf("stale cleanup retried automatically: %d", calls)
			}
			if (len(s.GitWorkspace.Worktrees) == 0) != (outcome == "removed") {
				t.Fatalf("wrong removal state: %+v", s.GitWorkspace.Worktrees)
			}
			if outcome == "warning" && !strings.Contains(s.CmdOutput, "retained") {
				t.Fatal("cleanup warning hidden")
			}
		})
	}
}

func TestConflictResponseRetainsWorktreeAndShowsRecovery(t *testing.T) {
	s := recoveryState()
	s.Page = PageAgents
	repository := *s.Repository
	repository.MergeHead, repository.Conflicts = "source", []string{"shared.go"}
	s.applyMergeWorktreeResponse("/repo/task", mergeWorktreeResponse{State: "conflicts", Error: "merge failed", Repository: &repository})
	if s.Page != PageGit || len(s.GitWorkspace.Worktrees) != 1 || s.Repository.MergeHead != "source" {
		t.Fatal("merge conflict or retained worktree hidden")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	frame := s.Render(120, 40)
	if !strings.Contains(frame, "shared.go") || !strings.Contains(frame, "external Git tool") {
		t.Fatalf("missing recovery guidance: %s", frame)
	}
	if len(s.PendingActions) != 0 {
		t.Fatal("inspecting recovery dispatched a mutation")
	}
}

func TestPlanBlocksOverlappingActiveTaskButAllowsIndependentFiles(t *testing.T) {
	s := NewState()
	s.ActiveChanges = []*ChangeSession{{ID: "active", Title: "First fix", State: "draft", Targets: []ChangeTarget{{Path: "shared.go"}}, Workflow: &ChangeWorkflow{Status: "running"}}}
	s.openTaskPlan(&ChangeSeed{Title: "Second fix", Paths: []string{"shared.go"}, Message: "Fix another cause", AcceptanceCriteria: []string{"Both fixes work"}})
	s.submitTaskPlan(true)
	if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "already in task") {
		t.Fatal("overlapping task dispatched")
	}
	s.openTaskPlan(&ChangeSeed{Title: "Independent fix", Paths: []string{"other.go"}, Message: "Fix independent cause", AcceptanceCriteria: []string{"Works"}})
	s.submitTaskPlan(true)
	if len(s.PendingActions) != 1 {
		t.Fatalf("independent task blocked: %s", s.Error)
	}
}

func TestSavedPlanBehindDestinationCannotGenerate(t *testing.T) {
	mutations := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			mutations++
			t.Errorf("unexpected mutation: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(GitWorkspace{GitRepository: GitRepository{GitIdentity: GitIdentity{Head: "new"}}, Worktrees: []GitWorktree{{Path: "/repo/old", GitIdentity: GitIdentity{Head: "old"}, Integration: "merged"}}})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	result := executeStartAgentTask(context.Background(), cl, startAgentTask{head: "new", act: ActionRequest{Kind: "start-agent", Seed: &ChangeSeed{WorktreePath: "/repo/old"}}})
	if result.err == nil || !strings.Contains(result.err.Error(), ":replan") || mutations != 0 {
		t.Fatalf("stale saved plan executed: %+v", result)
	}
}

func TestNewTaskCannotSilentlySwitchToAnAdvancedBase(t *testing.T) {
	creates := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects/current/git/worktrees":
			creates++
			http.Error(w, "destination advanced", http.StatusConflict)
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}})
		case "/api/projects/current/git/workspace":
			_ = json.NewEncoder(w).Encode(GitWorkspace{GitRepository: GitRepository{GitIdentity: GitIdentity{Head: "new", Branch: "main"}}})
		default:
			t.Errorf("task attempted generation or fallback after isolation failed: %s", r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	result := executeStartAgentTask(context.Background(), cl, startAgentTask{
		projectID: "p", projectRevision: "r", root: "/repo", head: "reviewed", branch: "main",
		targetBranch: "fix", reservedPath: "/repo/.worktrees/fix",
		act: ActionRequest{Kind: "start-agent", Seed: &ChangeSeed{Title: "Second fix", Paths: []string{"shared.go"}}},
	})
	if result.err == nil || creates != 1 || result.wtPath != "" {
		t.Fatalf("task changed base without review: %+v; create requests=%d", result, creates)
	}
}

func TestReplanDetachesSavedWorktreeAndRequiresFreshApproval(t *testing.T) {
	s := recoveryState()
	s.openTaskPlan(&ChangeSeed{Title: "Second fix", WorktreePath: "/repo/old", Paths: []string{"shared.go"}, Message: "Preserve the first fix", AcceptanceCriteria: []string{"Both fixes pass"}})
	s.PlanDraft.Saved = &ChangeSession{ID: "old-draft"}
	s.Project.ProjectRevision = "latest"
	s.ExecuteCommand(":replan")
	if s.Page != PagePlan || s.PlanDraft.Source.ProjectRevision != "latest" || s.PlanDraft.Saved != nil || s.PlanDraft.Seed.WorktreePath != "" || len(s.PendingActions) != 0 {
		t.Fatal("replan reused the old worktree or generated without approval")
	}
	if s.PlanDraft.Seed.Plan.Approach != "Preserve the first fix" || s.PlanDraft.Seed.Paths[0] != "shared.go" || s.PlanDraft.Seed.AcceptanceCriteria[0] != "Both fixes pass" {
		t.Fatal("replan lost the user's scope, approach or acceptance criteria")
	}
}

func TestInspectingFailedTaskNeverStartsRepair(t *testing.T) {
	s := NewState()
	s.Change = &ChangeSession{ID: "failed", State: "draft", Workflow: &ChangeWorkflow{Status: "failed"}}
	s.Navigate(PageAgents)
	s.AgentDetailOpen = true
	s.HandleKey(KeyEvent{Key: "Enter"})
	for _, action := range s.PendingActions {
		if action.Kind != "load-change" {
			t.Fatalf("inspection triggered %s", action.Kind)
		}
	}
}

func TestWorktreeRefreshReadsCurrentIdentityWithoutMutations(t *testing.T) {
	s := recoveryState()
	s.WorkspaceTrust = true
	current := ProjectIdentity{ProjectID: "p", ProjectRevision: "new"}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" {
			t.Errorf("refresh performed mutation: %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(Project{ProjectIdentity: current})
		case "/api/projects/current/git/workspace":
			if r.URL.Query().Get("project_revision") != "new" {
				t.Error("refresh used stale project identity")
			}
			_ = json.NewEncoder(w).Encode(GitWorkspace{GitRepository: GitRepository{ProjectIdentity: current, GitIdentity: GitIdentity{Head: "merged", Branch: "main"}}, Worktrees: []GitWorktree{{Path: "/repo/task", Integration: "merged"}}})
		case "/api/projects/current/index":
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: current})
		case "/api/projects/current/execution-trust":
			_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: false})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.HandleKey(KeyEvent{Key: "r"})
	s.FlushActions(context.Background(), cl)
	if s.Error != "" || s.projectRevision() != "new" || s.Repository.Head != "merged" || calls != 4 || s.WorkspaceTrust {
		t.Fatalf("refresh lost identity or inherited trust: %+v", s)
	}
}

func TestExternalMergeRecoveryUpdatesIndexBeforeTrustAndCleanup(t *testing.T) {
	s := recoveryState()
	s.WorkspaceTrust = true
	current := ProjectIdentity{ProjectID: "p", ProjectRevision: "resolved"}
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		switch r.URL.Path {
		case "/api/projects/current/reindex":
			if r.Method != http.MethodPost || len(calls) != 1 {
				t.Error("index update must be the first explicit recovery action")
			}
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: current})
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(Project{ProjectIdentity: current})
		case "/api/projects/current/git/workspace":
			_ = json.NewEncoder(w).Encode(GitWorkspace{GitRepository: GitRepository{ProjectIdentity: current, GitIdentity: GitIdentity{Head: "merged", Branch: "main"}}, Worktrees: []GitWorktree{{Path: "/repo/task", GitIdentity: GitIdentity{Head: "source", Branch: "fix"}, Integration: "merged"}}})
		case "/api/projects/current/index":
			_ = json.NewEncoder(w).Encode(ProjectIndex{ProjectIdentity: current})
		case "/api/projects/current/execution-trust":
			if r.Method == http.MethodGet {
				_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: true})
				return
			}
			var body struct {
				Revision string `json:"project_revision"`
				Confirm  bool   `json:"confirm"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Revision != "resolved" || !body.Confirm {
				t.Errorf("trust did not use resolved revision: %+v %v", body, err)
			}
			_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: true})
		case "/api/projects/current/git/cleanup-merged":
			var body WorktreeIntent
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.ProjectRevision != "resolved" || body.Head != "merged" || body.SourceHead != "source" {
				t.Errorf("cleanup did not capture resolved identity: %+v %v", body, err)
			}
			_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "merged", RemovedWorktree: "/repo/task"})
		default:
			t.Errorf("unexpected recovery request: %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.HandleKey(KeyEvent{Key: "R"})
	s.FlushActions(context.Background(), cl)
	if s.Error != "" || !s.WorkspaceTrust || s.projectRevision() != "resolved" || len(calls) != 5 || len(s.GitWorkspace.Worktrees) != 1 {
		t.Fatalf("index update lost saved trust, removed worktree or failed: %s; calls=%v", s.Error, calls)
	}
	s.HandleKey(KeyEvent{Key: "c"})
	s.FlushActions(context.Background(), cl)
	if s.Error != "" || len(s.GitWorkspace.Worktrees) != 0 || len(calls) != 7 {
		t.Fatalf("external merge recovery failed: %s; calls=%v", s.Error, calls)
	}
}

func TestDiscardRetainsTaskWithoutWorktreeRemovalReceipt(t *testing.T) {
	s := recoveryState()
	s.Change = &ChangeSession{ID: "pending", WorktreePath: "/repo/task"}
	s.ActiveChanges = []*ChangeSession{s.Change}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "discarded"})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	err = s.dispatchRejectChange(context.Background(), cl, "discard", ActionRequest{Target: "pending", Seed: &ChangeSeed{WorktreePath: "/repo/task"}})
	if err == nil || len(s.ActiveChanges) != 1 || len(s.GitWorkspace.Worktrees) != 1 {
		t.Fatal("discard hid task without a removal receipt")
	}
}
