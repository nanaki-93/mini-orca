package tui

import (
	"context"
	"encoding/json"
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
	if frame := s.Render(120, 40); !strings.Contains(frame, entry.WorkflowReason) || !strings.Contains(frame, "Work > Tasks") {
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
