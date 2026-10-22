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
	if s.Error != "" || s.projectRevision() != "new" || s.Repository.Head != "merged" || calls != 3 || s.WorkspaceTrust {
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
	if s.Error != "" || s.WorkspaceTrust || s.projectRevision() != "resolved" || len(calls) != 4 || len(s.GitWorkspace.Worktrees) != 1 {
		t.Fatalf("index update silently trusted, removed worktree or failed: %s; calls=%v", s.Error, calls)
	}
	s.ExecuteCommand("trust")
	s.FlushActions(context.Background(), cl)
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
