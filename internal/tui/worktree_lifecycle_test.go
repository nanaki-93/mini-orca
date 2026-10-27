package tui

import (
	"context"
	"encoding/json"
	"github.com/charmbracelet/x/ansi"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFailedWorktreeTaskRestoredWithoutReopeningIssue(t *testing.T) {
	s := recoveryState()
	identity := ProjectIdentity{ProjectID: "worktree", ProjectRevision: "worktree-revision"}
	target := FindingTarget{ProjectID: "p", Kind: "performance", ID: "duplicate-test", Path: "utils_test.go"}
	entry := ChangeHistoryEntry{ProjectIdentity: identity, ID: "failed-task", Title: "Duplicate test execution", Revision: 2, State: "draft", Freshness: "current", WorkflowStatus: "failed", WorkflowReason: "Go parse check failed", FindingTargets: []FindingTarget{target}}
	s.GitWorkspace.Worktrees[0].Sessions = []ChangeHistoryEntry{entry}
	s.Results = map[string]SectionResults{"performance": {Performance: []PerformanceReport{{ProjectIdentity: s.Project.ProjectIdentity, Path: target.Path, Findings: []PerformanceFinding{{ID: target.ID, Title: entry.Title}}}}}}
	s.mergeWorktreeSessionsIntoHistory()
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodGet || r.URL.Path != "/api/projects/current/changes/failed-task" || r.URL.Query().Get("worktree_path") != "/repo/task" || r.URL.Query().Get("project_id") != identity.ProjectID {
			t.Errorf("unexpected restore request: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode(ChangeSession{ProjectIdentity: identity, ID: entry.ID, Title: entry.Title, Revision: entry.Revision, State: entry.State, Freshness: entry.Freshness, FindingTargets: entry.FindingTargets, Workflow: &ChangeWorkflow{Status: "failed", Reason: entry.WorkflowReason}})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.syncActiveHistoryChanges(context.Background(), cl, "", "r")
	if requests != 1 || len(s.AgentItems()) != 1 || s.AgentItems()[0].Status != "failed" {
		t.Fatalf("failed task was not restored for recovery: requests=%d tasks=%+v", requests, s.AgentItems())
	}
	if issues := s.CollectAllIssues(); len(issues) != 1 || issues[0].Status != "in_progress" {
		t.Fatalf("failed task's issue returned to TODO: %+v", issues)
	}
	if status := worktreeStatus(s.GitWorkspace.Worktrees[0]); status != "FAILED" {
		t.Fatalf("same-base worktree shown as %s", status)
	}
	s.Navigate(PageWorktreeDetail)
	frame := ansi.Strip(s.Render(120, 40))
	var right []string
	for _, line := range strings.Split(frame, "\n") {
		if parts := strings.Split(line, "│"); len(parts) >= 4 {
			right = append(right, strings.TrimSpace(parts[3]))
		}
	}
	detail := strings.Join(strings.Fields(strings.Join(right, " ")), " ")
	if !strings.Contains(detail, entry.WorkflowReason) || !strings.Contains(detail, "Tasks [2]") {
		t.Fatalf("failure and recovery path missing: %s", frame)
	}
}

func TestWorktreeStatusDistinguishesUnfinishedTasksFromIntegration(t *testing.T) {
	for _, tc := range []struct{ workflow, want string }{{"", "DRAFT"}, {"running", "WORKING"}, {"failed", "FAILED"}, {"awaiting_human_review", "REVIEW"}, {"stale", "NEEDS_ATTENTION"}} {
		t.Run(tc.want, func(t *testing.T) {
			wt := GitWorktree{Integration: "merged", Sessions: []ChangeHistoryEntry{{State: "draft", WorkflowStatus: tc.workflow}}}
			if got := worktreeStatus(wt); got != tc.want {
				t.Fatalf("status = %s, want %s", got, tc.want)
			}
			wt.Locked = true
			if got := worktreeStatus(wt); got != "LOCKED" {
				t.Fatalf("task state hid lock: %s", got)
			}
		})
	}
}

func pendingWorktreeState() *State {
	s := recoveryState()
	targets := []FindingTarget{
		{ProjectID: "p", Kind: "semantic", ID: "bug", Path: "main.go"},
		{ProjectID: "p", Kind: "performance", ID: "perf", Path: "main.go"},
		{ProjectID: "p", Kind: "security", ID: "security", Path: "main.go", Source: "ai"},
	}
	entry := ChangeHistoryEntry{ID: "committed-task", ProjectIdentity: ProjectIdentity{ProjectID: "worktree", ProjectRevision: "worktree-revision"}, Title: "Fix findings", Revision: 3, State: "applied", Commit: &ChangeCommit{State: "committed", OID: "source"}, FindingTargets: targets}
	wt := &s.GitWorkspace.Worktrees[0]
	wt.Integration, wt.Mergeable = "up_to_date", true
	wt.Sessions, wt.Completed = []ChangeHistoryEntry{entry}, []ChangeHistoryEntry{entry}
	s.Findings = []Finding{{ID: "bug", Category: "bugs", Title: "Semantic bug", Status: "in_progress", Location: SourceAnchor{Path: "main.go"}}}
	s.Results = map[string]SectionResults{
		"performance": {Performance: []PerformanceReport{{ProjectIdentity: s.Project.ProjectIdentity, Path: "main.go", Findings: []PerformanceFinding{{ID: "perf", Title: "Performance issue"}}}}},
		"security":    {Security: []SecurityReport{{ProjectIdentity: s.Project.ProjectIdentity, Path: "main.go", Source: "ai", Findings: []SecurityFinding{{ID: "security", Title: "Security issue"}}}}},
	}
	return s
}

func TestPendingWorktreeLifecycleSurvivesRestart(t *testing.T) {
	fixture := pendingWorktreeState()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("restoring triggered a mutation: %s", r.URL)
		}
		switch r.URL.Path {
		case "/api/projects/current":
			_ = json.NewEncoder(w).Encode(fixture.Project)
		case "/api/projects/current/git/workspace":
			_ = json.NewEncoder(w).Encode(fixture.GitWorkspace)
		case "/api/projects/current/findings":
			_ = json.NewEncoder(w).Encode(FindingsResponse{ProjectIdentity: fixture.Project.ProjectIdentity, Findings: fixture.Findings})
		case "/api/projects/current/changes":
			_, _ = w.Write([]byte(`[]`))
		default:
			if strings.Contains(r.URL.Path, "/changes/committed-task") {
				t.Error("committed task was reloaded into the active pipeline")
			}
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s := NewState()
	if err := s.SyncFromDaemon(context.Background(), cl, ""); err != nil {
		t.Fatal(err)
	}
	s.Results = fixture.Results
	if len(s.AgentItems()) != 0 || len(s.CollectAllIssues()) != 0 || len(s.CompletedFindings) != 0 || worktreeStatus(s.GitWorkspace.Worktrees[0]) != "TO_MERGE" {
		t.Fatalf("restart lost pending merge ownership: tasks=%+v issues=%+v completed=%+v", s.AgentItems(), s.CollectAllIssues(), s.CompletedFindings)
	}
	for _, target := range fixture.GitWorkspace.Worktrees[0].Sessions[0].FindingTargets {
		if s.findCompletion(target) != nil {
			t.Fatal("applied worktree history was mistaken for merged completion")
		}
	}
}

func TestWorktreeMergeReceiptControlsCompletionAndCleanup(t *testing.T) {
	for _, outcome := range []string{"merged", "conflicts", "completion failed", "cleanup failed", "branch retained"} {
		t.Run(outcome, func(t *testing.T) {
			s := pendingWorktreeState()
			s.mergeWorktreeSessionsIntoHistory()
			result := mergeWorktreeResponse{State: "merged"}
			if outcome == "merged" || outcome == "cleanup failed" || outcome == "branch retained" {
				for _, target := range s.GitWorkspace.Worktrees[0].Sessions[0].FindingTargets {
					result.Completed = append(result.Completed, FindingCompletion{FindingTarget: target, CommitOID: "source"})
				}
			}
			if outcome == "merged" || outcome == "branch retained" {
				result.RemovedWorktree = "/repo/task"
			}
			if outcome == "conflicts" {
				result.State = "conflicts"
			} else if outcome != "merged" {
				result.Warnings = []string{outcome}
			}
			s.applyMergeWorktreeResponse("/repo/task", result)
			rows := s.CollectAllIssues()
			if len(rows) != len(result.Completed) {
				t.Fatalf("completion disagrees with daemon receipt: %+v", rows)
			}
			for _, row := range rows {
				if row.Status != "done" {
					t.Fatalf("confirmed completion not Done: %+v", row)
				}
			}
			if (len(s.GitWorkspace.Worktrees) == 0) != (result.RemovedWorktree != "") {
				t.Fatal("worktree removal disagrees with receipt")
			}
			if len(result.Warnings) > 0 && !strings.Contains(s.CmdOutput, outcome) {
				t.Fatal("cleanup warning hidden")
			}
		})
	}
}

func TestRejectPendingWorktreeReturnsAllLinkedIssueTypesToTodo(t *testing.T) {
	s := pendingWorktreeState()
	s.mergeWorktreeSessionsIntoHistory()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/current/execution-trust" {
			_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: true})
			return
		}
		if r.URL.Path != "/api/projects/current/git/discard-worktree" || r.Method != http.MethodPost {
			t.Errorf("unexpected reject action: %s %s", r.Method, r.URL)
		}
		_ = json.NewEncoder(w).Encode(mergeWorktreeResponse{State: "discarded", RemovedWorktree: "/repo/task"})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.HandleKey(KeyEvent{Key: "x"})
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("reject must expose the concrete discard decision before removing work")
	}
	s.Confirmation.OnConfirm(s)
	s.Confirmation = nil
	s.FlushActions(context.Background(), cl)
	rows := s.CollectAllIssues()
	if s.Error != "" || len(rows) != 3 || len(s.GitWorkspace.Worktrees) != 0 || len(s.AgentItems()) != 0 {
		t.Fatalf("reject did not restore issues: %+v error=%s", rows, s.Error)
	}
	for _, row := range rows {
		if row.Status != "open" {
			t.Fatalf("rejected issue not TODO: %+v", row)
		}
	}
}

func TestRejectCommandUsesWorktreeSelectionAndDoesNotRejectIntegratedWork(t *testing.T) {
	s := pendingWorktreeState()
	s.ExecuteCommand("reject")
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("reject command did not capture the selected pending worktree")
	}
	s.Confirmation = nil
	s.GitWorkspace.Worktrees[0].Integration = "merged"
	s.ExecuteCommand("reject")
	if s.Confirmation != nil || len(s.PendingActions) != 0 || !strings.Contains(s.CmdOutput, "already integrated") {
		t.Fatal("reject command offered to reopen already merged work")
	}
}
