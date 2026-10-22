package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

func approveTaskPlanForTest(t *testing.T, s *State) {
	t.Helper()
	if s.Page != PagePlan || s.PlanDraft == nil {
		t.Fatalf("expected editable plan before generation, page=%s", s.Page)
	}
	if len(s.PlanDraft.Seed.AcceptanceCriteria) == 0 {
		s.ExecuteCommand(":criteria Resolve the selected findings and preserve unrelated behavior")
	}
	s.HandleKey(KeyEvent{Key: "g"})
	if s.Page != PageAgents || s.Change == nil {
		t.Fatalf("plan approval failed: %s", s.Error)
	}
}

func TestPlanSaveEditAndApproveHasExplicitDispatchBoundary(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	s.dispatchSingleIssue(IssueRow{Name: "Fix result", Title: "Fix result", Path: "main.go", Cause: "Wrong result", Solution: "Correct the return value", AcceptanceCriteria: []string{"Returns two"}})
	if s.Page != PagePlan || s.Change != nil || len(s.PendingActions) != 0 {
		t.Fatal("selecting a fix dispatched or generated code")
	}
	s.ExecuteCommand(":context README.md")
	s.ExecuteCommand(":boundaries Keep the public signature")
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		plan := &ChangePlan{Hash: "captured-plan"}
		if strings.HasSuffix(r.URL.Path, "/workflow") {
			var hash string
			_ = json.Unmarshal(body["plan_hash"], &hash)
			if hash != "captured-plan" || string(body["auto_repair"]) != "false" {
				t.Errorf("unapproved or automatic dispatch: %+v", body)
			}
		} else {
			if err := json.Unmarshal(body["plan"], &plan.ChangePlanInput); err != nil {
				t.Error(err)
			}
			if len(plan.ReadPaths) != 1 || plan.ReadPaths[0] != "README.md" {
				t.Errorf("lost read-only context: %+v", plan)
			}
		}
		_ = json.NewEncoder(w).Encode(ChangeSession{ID: "c", ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, Revision: 1, Hash: "h", State: "draft", Plan: plan})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	s.ExecuteCommand(":save-plan")
	s.FlushActions(context.Background(), cl)
	if len(calls) != 1 || calls[0] != "/api/projects/current/changes" || s.PlanDraft.Saved == nil {
		t.Fatalf("saving ran a workflow or failed: %v, %s", calls, s.Error)
	}
	s.ExecuteCommand(":approach Validate the input, then correct the return value")
	s.ExecuteCommand(":generate")
	s.FlushActions(context.Background(), cl)
	if len(calls) != 3 || !strings.HasSuffix(calls[1], "/plan") || !strings.HasSuffix(calls[2], "/workflow") || s.Error != "" {
		t.Fatalf("edit/approval did not use the saved plan: %v, %s", calls, s.Error)
	}
}

func TestPlanRequiresExplicitScopeAndRejectsChangedProject(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}}
	s.Index = &ProjectIndex{Files: []IndexedFile{{Path: "unrelated.go"}}}
	s.ExecuteCommand(":agent Add a feature")
	s.ExecuteCommand(":criteria New behavior is covered")
	s.ExecuteCommand(":generate")
	if len(s.PendingActions) != 0 || len(s.PlanDraft.Seed.Paths) != 0 {
		t.Fatal("empty scope selected an arbitrary indexed file")
	}
	s.ExecuteCommand(":scope feature.go; feature_test.go")
	s.Project.ProjectRevision = "changed"
	s.ExecuteCommand(":generate")
	if len(s.PendingActions) != 0 || !strings.Contains(s.Error, "Project changed") {
		t.Fatal("stale plan approval dispatched")
	}
	paths := []string{"a.go", "b.go", "c.go", "d.go", "e.go", "f.go", "g.go", "h.go"}
	if expanded := domain.EnsureChangeTestPaths(paths, nil); len(expanded) <= 8 || !domain.ContainsString(expanded, "h.go") {
		t.Fatalf("scope was silently truncated: %v", expanded)
	}
}

func TestDiffHorizontalScrollingAndReviewComparison(t *testing.T) {
	for _, mode := range []string{"hunk", "split", "unified"} {
		t.Run(mode, func(t *testing.T) {
			s := NewState()
			s.Navigate(PageEditor)
			s.DiffMode = mode
			s.Change = &ChangeSession{ID: "c", Hash: "h", State: "draft", LastReviewedHash: "previous", Changes: []FileChange{{Path: "main.go", Diff: FileDiff{Lines: []DiffLine{{Kind: "add", NewLine: 1, Text: strings.Repeat("x", 120) + "CRITICAL_SUFFIX"}}}}}, ReviewDiff: []FileChange{{Path: "main.go", Diff: FileDiff{Lines: []DiffLine{{Kind: "add", NewLine: 1, Text: "SINCE_REVIEW"}}}}}}
			for i := 0; i < 10; i++ {
				s.HandleKey(KeyEvent{Key: "l"})
			}
			if frame := s.Render(120, 40); !strings.Contains(frame, "CRITICAL_SUFFIX") {
				t.Fatalf("long changed line cannot be inspected in %s", mode)
			}
			s.HandleKey(KeyEvent{Key: "w"})
			if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "review-file" {
				t.Fatal("file review control did not record intent")
			}
			s.PendingActions = nil
			s.HandleKey(KeyEvent{Key: "z"})
			if frame := s.Render(120, 40); !strings.Contains(frame, "SINCE_REVIEW") {
				t.Fatal("comparison did not show changes since review")
			}
			s.HandleKey(KeyEvent{Key: "x"})
			if len(s.PendingActions) != 0 {
				t.Fatal("comparison hunk altered the full proposal")
			}
		})
	}
}

func TestDetailsRemainReachableBeyondInitialViewport(t *testing.T) {
	s := NewState()
	s.openTaskPlan(&ChangeSeed{Title: "Detailed task", Paths: []string{"main.go"}, Message: "Inspect behavior", AcceptanceCriteria: []string{"First", "Second", "Third", "FOURTH_CRITERION"}, Plan: &ChangePlanInput{Problem: "A problem", Approach: strings.Repeat("Detailed approach. ", 30), NonGoals: []string{"One", "Two", "Three", "FOURTH_BOUNDARY"}}})
	var frames string
	for section := 0; section < 4; section++ {
		for i := 0; i < 15; i++ {
			frames += s.Render(90, 24)
			s.HandleKey(KeyEvent{Key: "PageDown"})
		}
		s.HandleKey(KeyEvent{Key: "Tab"})
	}
	if !strings.Contains(frames, "FOURTH_CRITERION") || !strings.Contains(frames, "FOURTH_BOUNDARY") {
		t.Fatal("plan detail was truncated instead of scrollable")
	}
}

func TestLongDiffExplanationsDoNotHideCode(t *testing.T) {
	s := NewState()
	s.ShowInlineWhy = true
	s.Navigate(PageEditor)
	s.Change = &ChangeSession{ID: "c", Hash: "h", AcceptanceCriteria: []string{strings.Repeat("Acceptance detail ", 60) + "LAST_CRITERION"}, Changes: []FileChange{{Path: "main.go", Summary: strings.Repeat("Reason for the fix. ", 50), Criteria: []int{1}, Diff: FileDiff{Lines: []DiffLine{{Kind: "add", NewLine: 1, Text: "VISIBLE_CODE"}}}}}}
	var details string
	for i := 0; i < 35; i++ {
		frame := s.Render(90, 28)
		if !strings.Contains(frame, "VISIBLE_CODE") {
			t.Fatal("explanation displaced all code")
		}
		details += frame
		s.HandleKey(KeyEvent{Key: "PageDown"})
	}
	if !strings.Contains(details, "LAST_CRITERION") {
		t.Fatal("scrolling lost acceptance details")
	}
}

func TestLatePlanAndIssueSourceCannotReplaceCurrentProjectOrDiff(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "old"}}
	s.openTaskPlan(&ChangeSeed{Title: "Fix", Paths: []string{"main.go"}, Message: "Fix behavior", AcceptanceCriteria: []string{"Behavior works"}})
	s.submitTaskPlan(true)
	act := s.PendingActions[0]
	s.Project.ProjectRevision = "new"
	res := startAgentResult{task: startAgentTask{act: act, projectID: "p", projectRevision: "old"}, wfSession: ChangeSession{ID: "old-result"}}
	if err := s.applyStartAgentResult(res); err == nil || s.Change.ID == "old-result" || s.PlanDraft.Submitted || s.Error == "" {
		t.Fatal("late plan result adopted or left approval stuck")
	}
	s.Navigate(PageEditor)
	s.File = nil
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(FileInfo{Path: "main.go", Content: "package main"})
	}))
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()
	if err := s.fetchIssueSource(context.Background(), cl, "late-source", ActionRequest{Target: "main.go", Source: s.Project.ProjectIdentity}); err != nil {
		t.Fatal(err)
	}
	if s.File != nil || s.IssueFile != nil {
		t.Fatal("issue source replaced the diff after navigation")
	}
}
