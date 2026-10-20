package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHubAndSubViewKeyboardNavigation(t *testing.T) {
	s := NewState()
	if s.Page != PageSummary {
		t.Fatalf("expected initial page summary, got %s", s.Page)
	}

	// Switch to [2] Agent Studio
	s.HandleKey(KeyEvent{Key: "2"})
	if s.Page != PageAgents {
		t.Fatalf("expected page agents, got %s", s.Page)
	}

	// Tab cycles sub-views within Agent Studio: Agents -> Issues -> Features -> Last Run -> File Scope
	s.HandleKey(KeyEvent{Key: "Tab"})
	if s.Page != PageBugs {
		t.Fatalf("expected page bugs after Tab, got %s", s.Page)
	}
	s.HandleKey(KeyEvent{Key: "Tab", Shift: true})
	if s.Page != PageAgents {
		t.Fatalf("expected page agents after Shift+Tab, got %s", s.Page)
	}

	// Bracket keys cycle hubs
	s.HandleKey(KeyEvent{Key: "]"})
	if s.Page != PageEditor {
		t.Fatalf("expected page editor after ], got %s", s.Page)
	}
	s.HandleKey(KeyEvent{Key: "["})
	if s.Page != PageAgents {
		t.Fatalf("expected page agents after [, got %s", s.Page)
	}
}

func TestIssuesFilteringSelectionAndAgentDispatch(t *testing.T) {
	s := NewState()
	s.Findings = []Finding{
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "b1",
			Severity:        "high",
			Title:           "Nil pointer dereference",
			Message:         "Check receiver before call",
			Category:        "bugs",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/a.go", StartLine: 12},
		},
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "s1",
			Severity:        "critical",
			Title:           "Command injection",
			Message:         "Validate argv",
			Category:        "security",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/b.go", StartLine: 44},
		},
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "b-wip",
			Severity:        "medium",
			Title:           "Active in-progress bug",
			Message:         "Being fixed",
			Category:        "bugs",
			Status:          "in_progress",
			Location:        SourceAnchor{Path: "internal/app/c.go", StartLine: 20},
		},
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "b-done",
			Severity:        "low",
			Title:           "Resolved bug",
			Message:         "Already fixed",
			Category:        "bugs",
			Status:          "fixed",
			Location:        SourceAnchor{Path: "internal/app/d.go", StartLine: 30},
		},
	}

	s.Navigate(PageBugs)
	// Default "open" filter must hide in_progress and done issues
	if got := len(s.FilteredIssues()); got != 2 {
		t.Fatalf("expected 2 open issues in ALL category (hiding in_progress and done), got %d", got)
	}

	// Cycle status filter with 's': open -> in_progress -> done -> open
	s.HandleKey(KeyEvent{Key: "s"})
	if s.IssueView != "in_progress" || len(s.FilteredIssues()) != 1 || s.FilteredIssues()[0].ID != "semantic:b-wip" {
		t.Fatalf("expected 1 in_progress issue after 's', got view=%s len=%d", s.IssueView, len(s.FilteredIssues()))
	}
	s.HandleKey(KeyEvent{Key: "s"})
	if s.IssueView != "done" || len(s.FilteredIssues()) != 1 || s.FilteredIssues()[0].ID != "semantic:b-done" {
		t.Fatalf("expected 1 done issue after second 's', got view=%s len=%d", s.IssueView, len(s.FilteredIssues()))
	}
	// Colon commands for filters (:filter, :status, :in-progress, :done, :open)
	s.ExecuteCommand(":in-progress")
	if s.IssueView != "in_progress" || len(s.FilteredIssues()) != 1 {
		t.Fatalf("expected :in-progress to switch to in_progress filter, got %s", s.IssueView)
	}
	s.ExecuteCommand(":filter done")
	if s.IssueView != "done" || len(s.FilteredIssues()) != 1 {
		t.Fatalf("expected :filter done to switch to done filter, got %s", s.IssueView)
	}
	s.ExecuteCommand(":status open")
	if s.IssueView != "open" || len(s.FilteredIssues()) != 2 {
		t.Fatalf("expected :status open to switch to open filter with 2 issues, got %s (%d)", s.IssueView, len(s.FilteredIssues()))
	}

	// Cycle issue type with 't': all -> bugs -> security -> performance
	s.HandleKey(KeyEvent{Key: "t"})
	if s.IssueCategory != "bugs" || len(s.FilteredIssues()) != 1 {
		t.Fatalf("expected bugs category with 1 open issue, got %s (%d)", s.IssueCategory, len(s.FilteredIssues()))
	}
	s.HandleKey(KeyEvent{Key: "T"})
	if s.IssueCategory != "all" {
		t.Fatalf("expected all category after 'T', got %s", s.IssueCategory)
	}

	// Select both issues with Space and 'j' + 'x', then batch dispatch with 'a'
	s.HandleKey(KeyEvent{Key: " "})
	s.HandleKey(KeyEvent{Key: "j"})
	s.HandleKey(KeyEvent{Key: "x"})
	if len(s.SelectedIssueIDs) != 2 {
		t.Fatalf("expected 2 selected issues, got %d", len(s.SelectedIssueIDs))
	}
	s.HandleKey(KeyEvent{Key: "a"})
	if len(s.PendingActions) == 0 || s.PendingActions[len(s.PendingActions)-1].Kind != "start-agent" {
		t.Fatalf("expected start-agent action, got %+v", s.PendingActions)
	}
	if s.Page != PageAgents || s.Change == nil {
		t.Fatalf("expected immediate transition to PageAgents with active ChangeSession, got page=%s change=%v", s.Page, s.Change)
	}
	if !strings.Contains(s.CmdOutput, "grouped agent fix for 2 issues") {
		t.Fatalf("unexpected command output: %s", s.CmdOutput)
	}

	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	workflowStarted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects/current/changes":
			_, _ = w.Write([]byte(`{"id":"chg-live","title":"Batch fix","state":"open","revision":1,"hash":"h1"}`))
		case "/api/projects/current/changes/chg-live/workflow":
			workflowStarted = true
			_, _ = w.Write([]byte(`{"id":"chg-live","title":"Batch fix","state":"running","revision":1,"hash":"h1","workflow":{"id":"wf-1","status":"running"}}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	s.FlushActions(context.Background(), cli)
	if !workflowStarted || s.Change.ID != "chg-live" {
		t.Fatalf("expected start-agent to open session and start workflow on daemon, workflowStarted=%v change=%+v", workflowStarted, s.Change)
	}
}

func TestEditorDiffHunkAndGuardedApplyUndo(t *testing.T) {
	s := NewState()
	s.Navigate(PageEditor)
	s.Change = &ChangeSession{
		ID:    "chg-42",
		Title: "Harden timeout",
		State: "checked",
		Changes: []FileChange{
			{
				Path:    "internal/api/request.go",
				Content: "package api\nvar timeout = 10\n",
				Diff: FileDiff{
					Lines: []DiffLine{
						{Kind: "context", OldLine: 1, NewLine: 1, Text: "package api"},
						{Kind: "delete", OldLine: 2, NewLine: 0, Text: "var timeout = 5"},
						{Kind: "add", OldLine: 0, NewLine: 2, Text: "var timeout = 10"},
					},
				},
			},
		},
		Checks: []Check{{Name: "go test", State: "passed"}},
	}
	s.ChangeReceipt = &ChangeMutation{SessionID: "chg-42", UndoAvailable: true}

	// Toggle inline explanations with 'e'
	s.HandleKey(KeyEvent{Key: "e"})
	if s.ShowInlineWhy {
		t.Fatal("expected ShowInlineWhy to toggle false")
	}

	// Reject current hunk with 'x' -> clears stale review/checks and enqueues update-proposal
	s.Change.ReviewedHash = "h-old"
	s.HandleKey(KeyEvent{Key: "x"})
	if !strings.Contains(s.Change.Changes[0].Content, "var timeout = 5") {
		t.Fatalf("expected hunk rejection to restore old line, got %q", s.Change.Changes[0].Content)
	}
	if s.Change.ReviewedHash != "" || len(s.Change.Checks) != 0 {
		t.Fatalf("expected hunk rejection to invalidate local review and checks, got reviewed=%q checks=%v", s.Change.ReviewedHash, s.Change.Checks)
	}
	if len(s.PendingActions) == 0 || s.PendingActions[len(s.PendingActions)-1].Kind != "update-proposal" {
		t.Fatalf("expected update-proposal action after hunk rejection, got %+v", s.PendingActions)
	}

	// Apply and commit with 'c' is blocked until re-verification checks pass
	s.HandleKey(KeyEvent{Key: "c"})
	if s.PendingActions[len(s.PendingActions)-1].Kind == "apply-commit" {
		t.Fatal("expected apply-commit to be blocked while verification checks are invalidated")
	}

	// Once verification checks pass on the updated diff, 'c' enqueues apply-commit
	s.Change.Checks = []Check{{Name: "go test", State: "passed"}}
	s.HandleKey(KeyEvent{Key: "c"})
	last := s.PendingActions[len(s.PendingActions)-1]
	if last.Kind != "apply-commit" || last.Target != "chg-42" {
		t.Fatalf("expected apply-commit action, got %+v", last)
	}

	// Request Undo with 'u'
	s.HandleKey(KeyEvent{Key: "u"})
	last = s.PendingActions[len(s.PendingActions)-1]
	if last.Kind != "undo" || last.Target != "chg-42" {
		t.Fatalf("expected undo action, got %+v", last)
	}
}

func TestColonCommandInputAndExecution(t *testing.T) {
	s := NewState()

	// Enter command mode with ':', type "perf", press Enter
	s.HandleKey(KeyEvent{Key: ":"})
	for _, ch := range []string{"p", "e", "r", "f"} {
		s.HandleKey(KeyEvent{Key: ch})
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageBugs || s.IssueCategory != "performance" {
		t.Fatalf("expected bugs page with performance filter, got page=%s category=%s", s.Page, s.IssueCategory)
	}

	s.ExecuteCommand(":open internal/app/service.go")
	if s.Page != PageEditor || s.File == nil || s.File.Path != "internal/app/service.go" {
		t.Fatalf("expected file opened in editor, got page=%s file=%+v", s.Page, s.File)
	}

	s.ExecuteCommand(":keys")
	if !s.ShowKeybindsModal {
		t.Fatal("expected :keys to open keybindings modal")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.ShowKeybindsModal {
		t.Fatal("expected Escape to close keybindings modal")
	}

	s.ExecuteCommand(":palette")
	if s.Palette != PaletteCRT {
		t.Fatalf("expected CRT palette, got %s", s.Palette)
	}

	s.ExecuteCommand(":quit")
	if !s.Quitting {
		t.Fatal("expected :quit to set Quitting=true")
	}
}

func TestIdentityGuardsAndDaemonSync(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"2.4.0"}`))
	})
	mux.HandleFunc("/api/projects/current", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-1","name":"mini-orca","path":"/repo"}`))
	})
	mux.HandleFunc("/api/projects/current/index", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-1","files":[{"path":"main.go","language":"Go"}]}`))
	})
	mux.HandleFunc("/api/projects/current/overview", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-1","analysis":{"purpose":"Test repo"}}`))
	})
	mux.HandleFunc("/api/projects/current/findings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-1","findings":[]}`))
	})
	mux.HandleFunc("/api/projects/current/git/workspace", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"available":true,"branch":"main","worktrees":[]}`))
	})
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-1","trusted":true}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	s := NewState()
	if err := s.SyncFromDaemon(context.Background(), client, ""); err != nil {
		t.Fatalf("SyncFromDaemon failed: %v", err)
	}
	if !s.Connected || s.Project == nil || s.Project.Name != "mini-orca" || s.Index == nil || len(s.Index.Files) != 1 {
		t.Fatalf("unexpected synced state: connected=%t project=%+v index=%+v", s.Connected, s.Project, s.Index)
	}

	// Stale project or revision must be rejected by identity guard
	if s.AcceptProjectResponse(ProjectIdentity{ProjectID: "other", ProjectRevision: "rev-1"}) {
		t.Fatal("expected different project_id to be rejected")
	}
	if s.AcceptProjectResponse(ProjectIdentity{ProjectID: "p1", ProjectRevision: "stale-rev"}) {
		t.Fatal("expected stale project_revision to be rejected")
	}
}

func TestDaemonWireContractsAndActions(t *testing.T) {
	var (
		capturedPaths      []string
		capturedLimits     map[string]int
		reviewedHash       string
		appliedHead        string
		undoConfirmed      bool
		mergedWorktree     string
		savedExclusions    []string
		bootstrapped       bool
		trustConfirmations []bool
		updatedProposal    string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProjectRevision string `json:"project_revision"`
			Confirm         bool   `json:"confirm"`
		}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ProjectRevision == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			trustConfirmations = append(trustConfirmations, body.Confirm)
			if !body.Confirm {
				_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","trusted":false}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","trusted":true}`))
	})
	mux.HandleFunc("/api/projects/current/changes", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Paths []string `json:"paths"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedPaths = body.Paths
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","title":"Fix","state":"checked","revision":1,"hash":"h-1","checks":[{"name":"test","state":"passed"}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","title":"Fix","state":"checked","revision":1,"hash":"h-1","checks":[{"name":"test","state":"passed"}],"workflow":{"id":"wf-1","status":"running"}}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","title":"Fix","state":"checked","revision":2,"hash":"h-2","checks":[{"name":"test","state":"passed"}],"workflow":{"id":"wf-1","status":"completed"}}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/messages", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Changes []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"changes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.Changes) > 0 {
			updatedProposal = body.Changes[0].Content
		}
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","title":"Fix","state":"draft","revision":3,"hash":"h-3","changes":[{"path":"internal/app/a.go","content":"package app\n"}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/review", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Hash string `json:"hash"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		reviewedHash = body.Hash
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","title":"Fix","state":"checked","revision":2,"hash":"h-2","reviewed_hash":"h-2","checks":[{"name":"test","state":"passed"}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/apply", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Head string `json:"head"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		appliedHead = body.Head
		_, _ = w.Write([]byte(`{"session_id":"chg-1","project_id":"p1","project_revision":"r2","state":"applied","hash":"h-2","undo_available":true}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/undo", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Confirm bool   `json:"confirm"`
			Hash    string `json:"hash"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		undoConfirmed = body.Confirm && body.Hash == "h-2"
		_, _ = w.Write([]byte(`{"session_id":"chg-1","project_id":"p1","project_revision":"r3","state":"reverted","hash":"h-2","undo_available":false}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/preview", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Limits map[string]int `json:"limits"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedLimits = body.Limits
		_, _ = w.Write([]byte(`{"preview_id":"prev-1","identity":{"project_id":"p1","project_revision":"r1","policy_fingerprint":"pf","provider_fingerprint":"prf","queue_id":"q1"},"limits":{"batch_files":64,"budget_seconds":300,"max_attempts_per_stage":2},"providers":[{"id":"prov-1"}]}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/run", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"identity":{"project_id":"p1","project_revision":"r1","policy_fingerprint":"pf","provider_fingerprint":"prf","queue_id":"q1","id":"run-1","generation":"g1"},"status":"running"}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/run/control", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"identity":{"project_id":"p1","project_revision":"r1","policy_fingerprint":"pf","provider_fingerprint":"prf","queue_id":"q1","id":"run-1","generation":"g2"},"status":"paused"}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/selection", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","selection_id":"sel-1","excluded_paths":[],"editable":true}`))
			return
		}
		var body struct {
			ExcludedPaths []string `json:"excluded_paths"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		savedExclusions = body.ExcludedPaths
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","selection_id":"sel-2","excluded_paths":["internal/app/a.go"],"editable":true}`))
	})
	mux.HandleFunc("/api/projects/current/git/merge", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mergedWorktree = body.Path
		_, _ = w.Write([]byte(`{"state":"merged"}`))
	})
	mux.HandleFunc("/api/projects/current/bootstrap/exec", func(w http.ResponseWriter, _ *http.Request) {
		bootstrapped = true
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/api/projects/current/instructions/proposal", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-inst","title":"Update AGENTS.md","state":"open"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	s.Index = &ProjectIndex{Files: []IndexedFile{{Path: "internal/app/a.go", Language: "Go"}}}
	s.Repository = &GitRepository{GitIdentity: GitIdentity{Head: "abc1234", Branch: "main"}, Available: true}
	s.GitWorkspace = &GitWorkspace{
		GitRepository: *s.Repository,
		Worktrees: []GitWorktree{
			{GitIdentity: GitIdentity{Head: "abc1234", Branch: "main"}, Path: "/repo", Current: true},
			{GitIdentity: GitIdentity{Head: "def5678", Branch: "fix/bug"}, Path: "/repo/.worktrees/fix", Mergeable: true},
		},
	}

	// 1. Start agent from single source file -> must automatically include companion _test.go
	s.dispatchSingleIssue(IssueRow{
		ID:       "semantic:b1",
		Name:     "Fix bug",
		Title:    "Fix bug",
		Path:     "internal/app/a.go",
		Category: "bugs",
		Target:   FindingTarget{Kind: "semantic", ID: "b1", Path: "internal/app/a.go"},
	})
	s.FlushActions(context.Background(), cli)
	if len(capturedPaths) != 2 || capturedPaths[1] != "internal/app/a_test.go" {
		t.Fatalf("expected companion test path internal/app/a_test.go in captured scope, got %v", capturedPaths)
	}

	// 2. PollActiveJobs refreshes running workflow to completed
	s.PollActiveJobs(context.Background(), cli)
	if s.Change.Revision != 2 || s.Change.Workflow.Status != "completed" {
		t.Fatalf("expected PollActiveJobs to refresh change session, got %+v", s.Change)
	}

	// 3. Apply & commit calls /review first and passes git head
	s.applyAndCommitProposal("fix: resolve issue")
	s.FlushActions(context.Background(), cli)
	if reviewedHash != "h-2" || appliedHead != "abc1234" || s.Project.ProjectRevision != "r2" {
		t.Fatalf("expected review+apply with git head and updated revision, got reviewed=%q head=%q rev=%q", reviewedHash, appliedHead, s.Project.ProjectRevision)
	}

	// 4. Guarded Undo sends confirm: true and hash
	s.requestGuardedUndo()
	s.FlushActions(context.Background(), cli)
	if !undoConfirmed || s.Project.ProjectRevision != "r3" {
		t.Fatalf("expected undo confirmation and revision r3, got confirmed=%v rev=%s", undoConfirmed, s.Project.ProjectRevision)
	}

	// 5. Analysis preview uses batch_files, budget_seconds, max_attempts_per_stage
	s.ExecuteCommand(":analyze")
	s.FlushActions(context.Background(), cli)
	if capturedLimits["batch_files"] != 64 || capturedLimits["budget_seconds"] != 300 || capturedLimits["max_attempts_per_stage"] != 2 {
		t.Fatalf("unexpected analysis limits payload: %+v", capturedLimits)
	}

	// 6. Pause and Resume analysis
	s.ExecuteCommand(":pause")
	s.FlushActions(context.Background(), cli)
	if s.Run == nil || s.Run.Status != "paused" {
		t.Fatalf("expected paused run, got %+v", s.Run)
	}
	s.ExecuteCommand(":resume")
	s.FlushActions(context.Background(), cli)

	// 7. File Scope toggle persists selection to daemon
	s.Navigate(PageAnalysisFiles)
	s.HandleKey(KeyEvent{Key: "x"})
	s.FlushActions(context.Background(), cli)
	if len(savedExclusions) != 1 || savedExclusions[0] != "internal/app/a.go" {
		t.Fatalf("expected saved exclusions [internal/app/a.go], got %v", savedExclusions)
	}

	// 8. Merge worktree & Bootstrap
	s.Navigate(PageGit)
	s.SelectedIdx = 1
	s.HandleKey(KeyEvent{Key: "m"})
	s.ExecuteCommand(":bootstrap")
	s.FlushActions(context.Background(), cli)
	if mergedWorktree != "/repo/.worktrees/fix" || !bootstrapped {
		t.Fatalf("expected worktree merge and bootstrap exec, got merged=%q bootstrapped=%v", mergedWorktree, bootstrapped)
	}

	// 9. Revoking workspace trust sends confirm: false to daemon
	s.WorkspaceTrust = true
	s.ExecuteCommand(":trust")
	s.FlushActions(context.Background(), cli)
	if s.WorkspaceTrust || len(trustConfirmations) == 0 || trustConfirmations[len(trustConfirmations)-1] != false {
		t.Fatalf("expected trust revocation to post confirm=false, got workspaceTrust=%v confirmations=%v", s.WorkspaceTrust, trustConfirmations)
	}

	// 10. Rejecting a hunk in PageEditor persists updated proposal to daemon and updates session hash/revision
	s.Project.ProjectRevision = "r1"
	s.Change = &ChangeSession{
		ID:       "chg-1",
		Revision: 2,
		Hash:     "h-2",
		Changes: []FileChange{{
			Path:    "internal/app/a.go",
			Content: "package app\nfunc Bad() {}\n",
			Diff: FileDiff{Lines: []DiffLine{
				{Kind: "context", OldLine: 1, NewLine: 1, Text: "package app"},
				{Kind: "add", OldLine: 0, NewLine: 2, Text: "func Bad() {}"},
			}},
		}},
	}
	s.Navigate(PageEditor)
	s.HandleKey(KeyEvent{Key: "x"})
	s.FlushActions(context.Background(), cli)
	if !strings.Contains(updatedProposal, "package app") || strings.Contains(updatedProposal, "Bad") || s.Change.Revision != 3 || s.Change.Hash != "h-3" {
		t.Fatalf("expected hunk rejection to sync proposal with daemon, got content=%q rev=%d hash=%q", updatedProposal, s.Change.Revision, s.Change.Hash)
	}
}

func TestAgentStartFailureClearsPreviewAndSurfacesDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/current/changes" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"type":"bad_request","message":"AI workflow request failed","detail":"working tree has unmerged conflicts"}`))
			return
		}
		_, _ = w.Write([]byte(`{"trusted":true}`))
	}))
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	s.Index = &ProjectIndex{Files: []IndexedFile{{Path: "main.go", Language: "Go"}}}
	s.ExecuteCommand(":agent fix main")
	s.FlushActions(context.Background(), cli)
	if s.Change != nil {
		t.Fatalf("expected failed agent start to clear fake preview session, got %+v", s.Change)
	}
	if !strings.Contains(s.Error, "working tree has unmerged conflicts") {
		t.Fatalf("expected detailed daemon error, got %q", s.Error)
	}
}

func TestSubmenuNavigationUpdatesPageAndSectionState(t *testing.T) {
	s := NewState()

	// Section 2: Tab cycles cleanly across the 5 sidebar submenu items without getting stuck on PageIssueDetail
	s.HandleKey(KeyEvent{Key: "2"})
	wantSec2 := []Page{PageBugs, PageFeatures, PageAnalysisRun, PageAnalysisFiles, PageAgents}
	for _, want := range wantSec2 {
		s.HandleKey(KeyEvent{Key: "Tab"})
		if s.Page != want {
			t.Fatalf("Section 2 Tab cycle: got %s, want %s", s.Page, want)
		}
	}

	// Section 4: h/l and j/k update both Page and DocsSection
	s.HandleKey(KeyEvent{Key: "4"})
	if s.Page != PageDocs || s.DocsSection != "context" {
		t.Fatalf("Section 4 initial state: page=%s section=%s", s.Page, s.DocsSection)
	}
	s.HandleKey(KeyEvent{Key: "j"})
	if s.Page != PageDocsComponents || s.DocsSection != "components" {
		t.Fatalf("Section 4 after 'j': page=%s section=%s", s.Page, s.DocsSection)
	}
	s.HandleKey(KeyEvent{Key: "l"})
	if s.Page != PageDocsDecisions || s.DocsSection != "decisions" {
		t.Fatalf("Section 4 after 'l': page=%s section=%s", s.Page, s.DocsSection)
	}
	s.HandleKey(KeyEvent{Key: "l"})
	if s.Page != PageDiagrams {
		t.Fatalf("Section 4 after second 'l': page=%s, want PageDiagrams", s.Page)
	}

	// Section 5: Tab updates both Page and RulesSection, while Left/Right arrows change options on the active page
	s.HandleKey(KeyEvent{Key: "5"})
	if s.Page != PageInstructions || s.RulesSection != "trust" {
		t.Fatalf("Section 5 initial state: page=%s section=%s", s.Page, s.RulesSection)
	}
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Page != PageInstructions || s.WorkspaceTrust {
		t.Fatalf("expected ArrowRight on PageInstructions to toggle WorkspaceTrust without navigating away, got page=%s trust=%v", s.Page, s.WorkspaceTrust)
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Page != PageInstructions || !s.WorkspaceTrust {
		t.Fatalf("expected ArrowLeft on PageInstructions to toggle WorkspaceTrust back to true, got page=%s trust=%v", s.Page, s.WorkspaceTrust)
	}
	s.HandleKey(KeyEvent{Key: "Tab"})
	if s.Page != PageModels || s.RulesSection != "models" {
		t.Fatalf("Section 5 after Tab: page=%s section=%s", s.Page, s.RulesSection)
	}
	s.HandleKey(KeyEvent{Key: "Tab"})
	if s.Page != PageBootstrap || s.RulesSection != "bootstrap" {
		t.Fatalf("Section 5 after second Tab: page=%s section=%s", s.Page, s.RulesSection)
	}
	s.HandleKey(KeyEvent{Key: "l"})
	if s.Page != PageLessons || s.RulesSection != "lessons" {
		t.Fatalf("Section 5 after 'l': page=%s section=%s", s.Page, s.RulesSection)
	}

	// Verify Left/Right arrows change options on PageBugs and PageAnalysisRun without navigating away
	s.Navigate(PageBugs)
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Page != PageBugs || s.IssueView != "in_progress" {
		t.Fatalf("expected ArrowRight on PageBugs to change status filter to in_progress, got page=%s view=%s", s.Page, s.IssueView)
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Page != PageBugs || s.IssueView != "open" {
		t.Fatalf("expected ArrowLeft on PageBugs to change status filter back to open, got page=%s view=%s", s.Page, s.IssueView)
	}
	s.Navigate(PageAnalysisRun)
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Page != PageAnalysisRun || s.EffectiveAnalysisConfig().BatchFiles != 128 {
		t.Fatalf("expected ArrowRight on PageAnalysisRun to step BatchFiles to 128, got page=%s batch=%d", s.Page, s.EffectiveAnalysisConfig().BatchFiles)
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Page != PageAnalysisRun || s.EffectiveAnalysisConfig().BatchFiles != 64 {
		t.Fatalf("expected ArrowLeft on PageAnalysisRun to step BatchFiles back to 64, got page=%s batch=%d", s.Page, s.EffectiveAnalysisConfig().BatchFiles)
	}
}

func TestEndToEndBugFixWorkflowAndLiveStageTracking(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	s.Index = &ProjectIndex{Files: []IndexedFile{{Path: "internal/app/fix.go", Language: "Go"}}}
	s.Findings = []Finding{
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "bug-42",
			Severity:        "high",
			Title:           "Context leak in worker",
			Message:         "Always defer cancel()",
			Category:        "bugs",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/fix.go", StartLine: 18},
		},
	}

	// 1. Start on PageBugs: 1 open issue
	s.Navigate(PageBugs)
	if len(s.FilteredIssues()) != 1 {
		t.Fatalf("expected 1 open bug initially, got %d", len(s.FilteredIssues()))
	}

	// 2. Press Enter to fix the bug -> navigates to PageAgents, bug becomes in_progress (hidden from open)
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageAgents {
		t.Fatalf("expected PageAgents after Enter on bug, got %s", s.Page)
	}
	openCnt, inProgCnt, doneCnt := s.IssueStatusCounts()
	if openCnt != 0 || inProgCnt != 1 || doneCnt != 0 {
		t.Fatalf("expected counts open=0 in_progress=1 done=0, got %d/%d/%d", openCnt, inProgCnt, doneCnt)
	}

	// 3. Simulate workflow reaching awaiting_human_review with diff and passed checks
	s.Change = &ChangeSession{
		ID:             "chg-42",
		Title:          "Context leak in worker",
		State:          "checked",
		Revision:       2,
		Hash:           "h-42",
		ReviewedHash:   "h-42",
		FindingTargets: []FindingTarget{{ProjectID: "p1", Kind: "semantic", ID: "bug-42", Path: "internal/app/fix.go"}},
		Changes: []FileChange{
			{
				Path: "internal/app/fix.go",
				Diff: FileDiff{Lines: []DiffLine{{Kind: "add", NewLine: 19, Text: "defer cancel()"}}},
			},
		},
		Checks: []Check{{Name: "go test", State: "passed"}},
		Workflow: &ChangeWorkflow{
			ID:     "wf-42",
			Status: "awaiting_human_review",
			Stages: []ChangeWorkflowStage{
				{Name: "create", Status: "completed"},
				{Name: "test", Status: "completed"},
				{Name: "review", Status: "completed"},
				{Name: "human_review", Status: "awaiting_human_review"},
			},
			Review: &ChangeWorkflowReview{Verdict: "approve", Summary: "Clean context cancellation fix"},
		},
	}
	agentFrame := s.Render(120, 36)
	if !strings.Contains(agentFrame, "Ready to Review & Apply") || !strings.Contains(agentFrame, "Clean context cancellation fix") {
		t.Fatalf("expected PageAgents to display critic summary and Ready to Review & Apply:\n%s", agentFrame)
	}

	// 4. Press Enter (or 'd') on PageAgents -> opens Diff Inspector (PageEditor)
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageEditor {
		t.Fatalf("expected PageEditor after Enter on ready agent, got %s", s.Page)
	}

	// 5. Apply & commit -> bug moves to 'done'
	s.recordAppliedFindingCompletions(s.Change.FindingTargets, &ChangeCommit{OID: "deadbeef"})
	s.Change.State = "applied"
	openCnt, inProgCnt, doneCnt = s.IssueStatusCounts()
	if openCnt != 0 || inProgCnt != 0 || doneCnt != 1 {
		t.Fatalf("expected counts open=0 in_progress=0 done=1 after apply, got %d/%d/%d", openCnt, inProgCnt, doneCnt)
	}
}

func TestModelInspectionAndScopeRoutingManagement(t *testing.T) {
	s := NewState()
	temp := float32(0.15)
	maxTok := 4096
	s.Models = &ModelCatalog{
		Scopes: map[string]Model{
			"analyze": {
				Scope: "analyze", Profile: "analyze", Model: "qwen2.5-coder:32b",
				ReasoningEffort: "high", ProviderOrigin: "http://127.0.0.1:11434",
				RemoteProvider: false, Temperature: &temp, MaxTokens: &maxTok,
				ContextMaxTokens: 32768, Timeout: "5m0s", MaxRetries: 3,
			},
			"bug": {
				Scope: "bug", Profile: "bug", Model: "qwen2.5-coder:14b",
				ReasoningEffort: "medium", ProviderOrigin: "http://127.0.0.1:11434",
				RemoteProvider: false, ContextMaxTokens: 16384, Timeout: "3m0s", MaxRetries: 2,
			},
			"function": {
				Scope: "function", Profile: "function", Model: "qwen2.5-coder:7b",
				ReasoningEffort: "low", ProviderOrigin: "http://127.0.0.1:11434",
				RemoteProvider: false, ContextMaxTokens: 8192, Timeout: "1m0s", MaxRetries: 1,
			},
		},
	}
	s.AvailableModels = &AvailableModels{
		Models: []ModelChoice{
			{
				ID: "agy:gemini-2.5-pro", Name: "gemini-2.5-pro", Source: "discovered",
				Model: Model{
					Model: "gemini-2.5-pro", ReasoningEffort: "high",
					ProviderOrigin: "cli://agy", RemoteProvider: true,
					ContextMaxTokens: 131072, Timeout: "5m0s", MaxRetries: 2,
				},
			},
		},
		Pi:  ModelDiscovery{Status: "unavailable"},
		Agy: ModelDiscovery{Status: "ok"},
	}

	s.ExecuteCommand(":models")
	if s.Page != PageModels {
		t.Fatalf("expected :models to navigate to PageModels, got %s", s.Page)
	}

	// Row 0 is ANALYZE scope; cycling model with 'm' should change analyze assignment
	s.SelectedIdx = 0
	s.HandleKey(KeyEvent{Key: "m"})
	if s.AssignedScopeModelID("analyze") == "analyze" {
		t.Fatalf("expected 'm' to cycle analyze scope model away from default, got %s", s.AssignedScopeModelID("analyze"))
	}

	// 'x' resets selected scope back to default
	s.HandleKey(KeyEvent{Key: "x"})
	if s.AssignedScopeModelID("analyze") != "analyze" {
		t.Fatalf("expected 'x' to reset analyze scope to 'analyze', got %s", s.AssignedScopeModelID("analyze"))
	}

	// Direct colon command :model bug agy:gemini-2.5-pro
	s.ExecuteCommand(":model bug agy:gemini-2.5-pro")
	if got := s.AssignedScopeModelID("bug"); got != "agy:gemini-2.5-pro" {
		t.Fatalf("expected bug scope assigned to agy:gemini-2.5-pro, got %s", got)
	}
	effBug := s.EffectiveScopeModel("bug")
	if !effBug.RemoteProvider || effBug.ProviderOrigin != "cli://agy" || effBug.ContextMaxTokens != 131072 {
		t.Fatalf("unexpected effective bug model metadata: %+v", effBug)
	}

	// Verify workflowModelsPayload passes discovered catalog model ID to daemon
	wfModels, confirmed := s.workflowModelsPayload()
	if wfModels["create"] != "agy:gemini-2.5-pro" || !containsString(confirmed, "agy:gemini-2.5-pro") {
		t.Fatalf("expected workflowModelsPayload to include agy:gemini-2.5-pro, got models=%v confirmed=%v", wfModels, confirmed)
	}

	// Render PageModels and verify Model Inspector details are visible
	frame := s.Render(120, 36)
	for _, token := range []string{"SELECTED SCOPE & MODEL INSPECTOR", "agy:gemini-2.5-pro", "cli://agy", "Agy(ok)"} {
		if !strings.Contains(frame, token) {
			t.Fatalf("expected PageModels render to contain %q:\n%s", token, frame)
		}
	}

	// Apply preset via :preset deep-audit
	s.ExecuteCommand(":preset deep-audit")
	if s.ActivePresetID != "deep-audit" || s.AssignedScopeModelID("analyze") != "remote-pro" {
		t.Fatalf("expected deep-audit preset active, got preset=%s analyze=%s", s.ActivePresetID, s.AssignedScopeModelID("analyze"))
	}
}

func TestStaleSessionSkippingErrorClearingAndPausedAnalysisResume(t *testing.T) {
	var (
		resumedPreviewRefresh bool
		resumedAction         string
		reanalyzeClearResults bool
		openChangeAttempts    int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"version":"2.4.0"}`))
	})
	mux.HandleFunc("/api/projects/current", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-current","name":"pwd-manager","path":"/repo"}`))
	})
	mux.HandleFunc("/api/projects/current/index", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-current","files":[{"path":"src/main.rs","language":"Rust"}]}`))
	})
	mux.HandleFunc("/api/projects/current/reindex", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-current","files":[{"path":"src/main.rs","language":"Rust"}]}`))
	})
	mux.HandleFunc("/api/projects/current/overview", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-current","analysis":{"purpose":"Password vault"},"analysis_run":{"identity":{"project_id":"p1","project_revision":"rev-current","id":"run-paused","queue_id":"q1"},"plan":{"refresh":true,"limits":{"batch_files":64,"budget_seconds":300,"max_attempts_per_stage":2}},"status":"paused","elapsed_seconds":300,"window_files_completed":7,"reason":"The file dispatch allowance ended; preview and resume the remaining work."}}`))
	})
	mux.HandleFunc("/api/projects/current/findings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"rev-current","findings":[{"id":"bug-stale","severity":"high","title":"Unwrapped option","message":"Check None","category":"bugs","status":"open","location":{"path":"src/main.rs","start_line":10}}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`[{"id":"chg-stale","project_id":"p1","project_revision":"rev-old","title":"Batch fix","revision":8,"state":"draft","freshness":"stale","workflow_status":"stale","finding_targets":[{"project_id":"p1","kind":"semantic","id":"bug-stale","path":"src/main.rs"}]}]`))
			return
		}
		openChangeAttempts++
		var body struct {
			ProjectRevision string `json:"project_revision"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.ProjectRevision != "rev-current" {
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"message":"project or file changed; refresh the proposal"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chg-fresh","project_id":"p1","project_revision":"rev-current","title":"Unwrapped option","state":"open","revision":1,"hash":"h-fresh"}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-fresh/workflow", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-fresh","project_id":"p1","project_revision":"rev-current","title":"Unwrapped option","state":"running","revision":1,"hash":"h-fresh","workflow":{"id":"wf-fresh","status":"running"}}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/preview", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Refresh      bool           `json:"refresh"`
			ClearResults bool           `json:"clear_results"`
			ResumeRun    map[string]any `json:"resume_run"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if len(body.ResumeRun) > 0 {
			resumedPreviewRefresh = body.Refresh
		} else {
			reanalyzeClearResults = body.ClearResults
		}
		_, _ = w.Write([]byte(`{"preview_id":"prev-resume","identity":{"project_id":"p1","project_revision":"rev-current","id":"run-paused","queue_id":"q1"},"limits":{"batch_files":64,"budget_seconds":300,"max_attempts_per_stage":2},"providers":[{"id":"local"}]}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/run/control", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Action string `json:"action"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		resumedAction = body.Action
		_, _ = w.Write([]byte(`{"identity":{"project_id":"p1","project_revision":"rev-current","id":"run-paused","queue_id":"q1"},"status":"running"}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/run", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"identity":{"project_id":"p1","project_revision":"rev-current","id":"run-new","queue_id":"q2"},"status":"running"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	s := NewState()
	if err := s.SyncFromDaemon(context.Background(), cli, ""); err != nil {
		t.Fatalf("SyncFromDaemon failed: %v", err)
	}

	// 1. Stale historical change session must NOT be auto-loaded or hijack open findings
	if s.Change != nil {
		t.Fatalf("expected stale historical session to be skipped on startup, got %+v", s.Change)
	}
	if openIssues := s.FilteredIssues(); len(openIssues) != 1 || openIssues[0].Status != "open" {
		t.Fatalf("expected finding from stale session to remain open, got %+v", openIssues)
	}

	// 2. Navigating between pages clears s.Error so errors never stay stuck on every page
	s.Error = "project or file changed; refresh the proposal"
	s.Navigate(PageBugs)
	if s.Error != "" {
		t.Fatalf("expected Navigate to clear stale s.Error, got %q", s.Error)
	}

	// 3. Starting an agent when local revision is stale auto-syncs revision and succeeds on retry
	s.Project.ProjectRevision = "rev-stale-local"
	s.HandleKey(KeyEvent{Key: "Enter"})
	s.FlushActions(context.Background(), cli)
	if openChangeAttempts != 2 || s.Change == nil || s.Change.ID != "chg-fresh" || s.Error != "" {
		t.Fatalf("expected auto-resync retry to succeed on 2nd attempt, attempts=%d change=%+v err=%q", openChangeAttempts, s.Change, s.Error)
	}

	// 4. Pressing 'r' or Enter on PageAnalysisRun when paused resumes with matching plan.Refresh
	s.Navigate(PageAnalysisRun)
	s.HandleKey(KeyEvent{Key: "Enter"})
	s.FlushActions(context.Background(), cli)
	if !resumedPreviewRefresh || resumedAction != "resume" || s.Run.Status != "running" {
		t.Fatalf("expected Enter on paused PageAnalysisRun to resume with plan.Refresh=true, got refresh=%v action=%q status=%s",
			resumedPreviewRefresh, resumedAction, s.Run.Status)
	}

	// 5. Pressing 'n' (Reanalyze) while paused sends clear_results=true so daemon accepts replacement
	s.Run.Status = "paused"
	s.HandleKey(KeyEvent{Key: "n"})
	s.FlushActions(context.Background(), cli)
	if !reanalyzeClearResults {
		t.Fatal("expected 'n' (reanalyze) while paused to send clear_results=true")
	}
}

func TestLessonsGraduationAndInstructionProposalWorkflow(t *testing.T) {
	mux := http.NewServeMux()
	var capturedKind, capturedContent string
	var checksRunTests *bool
	var reviewed bool

	mux.HandleFunc("/api/projects/current/instructions", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r1","path":"AGENTS.md","exists":true,"existing_content":"# Project instructions\n"}`))
	})
	mux.HandleFunc("/api/projects/current/instructions/proposal", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Kind    string `json:"kind"`
			Content string `json:"content"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedKind = body.Kind
		capturedContent = body.Content
		if body.Kind != "instructions" {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":"invalid_request","message":"provide a task title, kind and one to eight paths"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chg-inst","project_id":"p1","project_revision":"r1","kind":"instructions","title":"Update AGENTS.md","state":"proposed","revision":1,"hash":"h-inst","changes":[{"path":"AGENTS.md","content":` + mustJSON(body.Content) + `}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-inst/checks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RunTests bool `json:"run_tests"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		checksRunTests = &body.RunTests
		_, _ = w.Write([]byte(`{"id":"chg-inst","project_id":"p1","project_revision":"r1","kind":"instructions","title":"Update AGENTS.md","state":"checked","revision":2,"hash":"h-inst","checks":[{"name":"text: AGENTS.md","state":"passed"}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-inst/review", func(w http.ResponseWriter, _ *http.Request) {
		reviewed = true
		_, _ = w.Write([]byte(`{"id":"chg-inst","project_id":"p1","project_revision":"r1","kind":"instructions","title":"Update AGENTS.md","state":"reviewed","revision":3,"hash":"h-inst","reviewed_hash":"h-inst","checks":[{"name":"text: AGENTS.md","state":"passed"}]}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	s.Navigate(PageLessons)
	s.HandleKey(KeyEvent{Key: "Enter"})
	s.FlushActions(context.Background(), cli)

	if s.Error != "" {
		t.Fatalf("expected lesson graduation to succeed without error, got %q", s.Error)
	}
	if capturedKind != "instructions" {
		t.Fatalf("expected kind='instructions' in proposal payload, got %q", capturedKind)
	}
	if !strings.Contains(capturedContent, "# Project instructions") || !strings.Contains(capturedContent, "## Harness² Lessons & Guardrails") {
		t.Fatalf("expected existing AGENTS.md content merged with graduated lesson, got %q", capturedContent)
	}
	if checksRunTests == nil || *checksRunTests {
		t.Fatalf("expected fast instruction checks with run_tests=false, got %v", checksRunTests)
	}
	if !reviewed || s.Change == nil || s.Change.State != "reviewed" {
		t.Fatalf("expected instruction proposal to be reviewed and ready, got reviewed=%v change=%+v", reviewed, s.Change)
	}

	// Graduating the same lesson again is a safe no-op without error
	s.HandleKey(KeyEvent{Key: "g"})
	if s.Error != "" || !strings.Contains(s.CmdOutput, "already graduated") {
		t.Fatalf("expected duplicate lesson graduation to report already graduated without error, got err=%q out=%q", s.Error, s.CmdOutput)
	}

	// Pressing 'd' on PageLessons jumps to PageEditor to inspect the AGENTS.md diff
	s.HandleKey(KeyEvent{Key: "d"})
	if s.Page != PageEditor {
		t.Fatalf("expected 'd' on PageLessons to open PageEditor diff, got %s", s.Page)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestAnalysisRunConfigKeysCommandsAndDispatch(t *testing.T) {
	mux := http.NewServeMux()
	var previewLimits map[string]int
	var previewFeatures, previewRefresh, previewClear bool
	var runLimits map[string]int
	var runFeatures, runRefresh, runClear bool
	var runModels map[string]string

	mux.HandleFunc("/api/projects/current/analysis/preview", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Limits          map[string]int `json:"limits"`
			IncludeFeatures bool           `json:"include_features"`
			Refresh         bool           `json:"refresh"`
			ClearResults    bool           `json:"clear_results"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		previewLimits = body.Limits
		previewFeatures = body.IncludeFeatures
		previewRefresh = body.Refresh
		previewClear = body.ClearResults
		_, _ = w.Write([]byte(`{"preview_id":"prev-cfg","identity":{"project_id":"p1","project_revision":"r1","queue_id":"q1"},"providers":[{"id":"local"}]}`))
	})
	mux.HandleFunc("/api/projects/current/analysis/run", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Limits          map[string]int    `json:"limits"`
			IncludeFeatures bool              `json:"include_features"`
			Refresh         bool              `json:"refresh"`
			ClearResults    bool              `json:"clear_results"`
			Models          map[string]string `json:"models"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		runLimits = body.Limits
		runFeatures = body.IncludeFeatures
		runRefresh = body.Refresh
		runClear = body.ClearResults
		runModels = body.Models
		_, _ = w.Write([]byte(`{"identity":{"project_id":"p1","project_revision":"r1","id":"run-cfg","queue_id":"q1"},"status":"running"}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "mini-orca"}
	s.AvailableModels = &AvailableModels{
		Models: []ModelChoice{{ID: "pi:qwen", Name: "Qwen"}},
	}
	s.Navigate(PageAnalysisRun)

	// Hotkeys on PageAnalysisRun cycle batch, budget, attempts, features, and cache policy
	s.HandleKey(KeyEvent{Key: "b"}) // 64 -> 128
	s.HandleKey(KeyEvent{Key: "t"}) // 300 -> 600
	s.HandleKey(KeyEvent{Key: "a"}) // 2 -> 3
	s.HandleKey(KeyEvent{Key: "f"}) // true -> false
	s.HandleKey(KeyEvent{Key: "c"}) // incremental -> refresh
	s.HandleKey(KeyEvent{Key: "c"}) // refresh -> clean reanalyze (refresh=true, clear_results=true)

	cfg := s.EffectiveAnalysisConfig()
	if cfg.BatchFiles != 128 || cfg.BudgetSeconds != 600 || cfg.MaxAttemptsPerStage != 3 || cfg.IncludeFeatures || !cfg.Refresh || !cfg.ClearResults {
		t.Fatalf("unexpected analysis config after hotkeys: %+v", cfg)
	}

	// Render PageAnalysisRun and verify config table + execution plan panel appear
	frame := s.Render(120, 36)
	for _, tok := range []string{"ANALYSIS RUN CONFIGURATION", "128 files / batch", "600s (10m 0s)", "3 attempts / stage", "CLEAN"} {
		if !strings.Contains(frame, tok) {
			t.Fatalf("expected PageAnalysisRun frame to contain %q:\n%s", tok, frame)
		}
	}

	// Colon commands override individual or combined analysis config parameters
	s.ExecuteCommand(":batch 256")
	s.ExecuteCommand(":budget 900s")
	s.ExecuteCommand(":attempts 4")
	s.ExecuteCommand(":model bug pi:qwen")
	s.ExecuteCommand(":analyze")
	s.FlushActions(context.Background(), cli)

	if previewLimits["batch_files"] != 256 || previewLimits["budget_seconds"] != 900 || previewLimits["max_attempts_per_stage"] != 4 {
		t.Fatalf("expected custom preview limits 256/900/4, got %+v", previewLimits)
	}
	if previewFeatures || !previewRefresh || !previewClear {
		t.Fatalf("expected preview flags features=false refresh=true clear=true, got features=%v refresh=%v clear=%v",
			previewFeatures, previewRefresh, previewClear)
	}
	if runLimits["batch_files"] != 256 || !runRefresh || !runClear || runFeatures || runModels["code"] != "pi:qwen" {
		t.Fatalf("expected run start payload to carry custom config and models, got limits=%+v refresh=%v clear=%v features=%v models=%+v",
			runLimits, runRefresh, runClear, runFeatures, runModels)
	}

	// Reset via 'x' restores defaults
	s.Navigate(PageAnalysisRun)
	s.HandleKey(KeyEvent{Key: "x"})
	if def := s.EffectiveAnalysisConfig(); def.BatchFiles != 64 || def.BudgetSeconds != 300 || def.MaxAttemptsPerStage != 2 || !def.IncludeFeatures || def.Refresh || def.ClearResults {
		t.Fatalf("expected 'x' to reset analysis config to defaults, got %+v", def)
	}
}

func TestCustomModelPresetSaveCycleApplyDeleteAndPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".mini-orca"), 0o755); err != nil {
		t.Fatal(err)
	}

	s := NewState()
	s.Project = &Project{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
		Name:            "mini-orca",
		Path:            tmpDir,
	}
	s.AvailableModels = &AvailableModels{
		Models: []ModelChoice{
			{ID: "pi:qwen", Name: "Qwen 2.5 Coder"},
			{ID: "agy:claude", Name: "Claude Sonnet"},
		},
	}

	s.Navigate(PageModels)
	s.ExecuteCommand(":model analyze pi:qwen")
	s.ExecuteCommand(":model bug agy:claude")
	s.ExecuteCommand(":model function local")

	// 1. Save named preset via :preset-save
	s.ExecuteCommand(":preset-save Rust Audit")
	if len(s.CustomPresets) != 1 || s.CustomPresets[0].ID != "rust-audit" || s.ActivePresetID != "rust-audit" {
		t.Fatalf("expected custom preset 'rust-audit' saved and active, got presets=%+v active=%q", s.CustomPresets, s.ActivePresetID)
	}

	// 2. Quick-save preset via 's' hotkey and prompt via 'n' hotkey
	s.ExecuteCommand(":model bug pi:qwen")
	s.HandleKey(KeyEvent{Key: "s"})
	if len(s.CustomPresets) != 2 {
		t.Fatalf("expected 's' hotkey to save a 2nd custom preset, got %+v", s.CustomPresets)
	}
	s.HandleKey(KeyEvent{Key: "n"})
	if !s.CmdActive || s.CmdInput != "preset-save " {
		t.Fatalf("expected 'n' hotkey to prefill :preset-save in command bar, got active=%v input=%q", s.CmdActive, s.CmdInput)
	}
	s.HandleKey(KeyEvent{Key: "Escape"})

	// 3. Switch to built-in preset, then re-apply custom preset via :preset rust-audit
	s.ExecuteCommand(":preset local-default")
	if s.AssignedScopeModelID("bug") != "local" {
		t.Fatalf("expected local-default preset to set bug=local, got %q", s.AssignedScopeModelID("bug"))
	}
	s.ExecuteCommand(":preset rust-audit")
	if s.AssignedScopeModelID("analyze") != "pi:qwen" || s.AssignedScopeModelID("bug") != "agy:claude" {
		t.Fatalf("expected rust-audit preset to restore pi:qwen and agy:claude, got %+v", s.ScopeModels)
	}

	// 4. Verify custom presets are persisted in .mini-orca/tui-presets.json and reloaded
	s2 := NewState()
	s2.Project = s.Project
	s2.loadCustomPresetsFromDisk()
	if len(s2.CustomPresets) != 2 || s2.CustomPresets[0].ID != "rust-audit" {
		t.Fatalf("expected custom presets to reload from .mini-orca/tui-presets.json, got %+v", s2.CustomPresets)
	}

	// 5. Render PageModels and verify custom preset row is displayed
	frame := s.Render(120, 36)
	if !strings.Contains(frame, "Rust Audit") || !strings.Contains(frame, "Save Preset") {
		t.Fatalf("expected PageModels frame to display saved custom preset 'Rust Audit':\n%s", frame)
	}

	// 6. Delete custom preset via 'd' hotkey
	s.SelectedIdx = len(CanonicalModelScopes()) + len(DefaultModelPresets())
	s.HandleKey(KeyEvent{Key: "d"})
	if len(s.CustomPresets) != 1 {
		t.Fatalf("expected 'd' to delete selected custom preset, remaining=%+v", s.CustomPresets)
	}
}
