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

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
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

	// Verify Left/Right arrows on PageAgents (Pipeline view) stay on PageAgents and do not move to Issues
	s.Navigate(PageAgents)
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Page != PageAgents {
		t.Fatalf("expected ArrowRight on PageAgents to stay on PageAgents, got %s", s.Page)
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Page != PageAgents {
		t.Fatalf("expected ArrowLeft on PageAgents to stay on PageAgents, got %s", s.Page)
	}

	// Verify Left/Right arrows do NOT change status filter on PageBugs, while 's'/'S' does
	s.Navigate(PageBugs)
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Page != PageBugs || s.IssueView != "open" {
		t.Fatalf("expected ArrowRight on PageBugs not to change status filter, got page=%s view=%s", s.Page, s.IssueView)
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Page != PageBugs || s.IssueView != "open" {
		t.Fatalf("expected ArrowLeft on PageBugs not to change status filter, got page=%s view=%s", s.Page, s.IssueView)
	}
	s.HandleKey(KeyEvent{Key: "s"})
	if s.Page != PageBugs || s.IssueView != "in_progress" {
		t.Fatalf("expected 's' on PageBugs to change status filter to in_progress, got page=%s view=%s", s.Page, s.IssueView)
	}
	s.HandleKey(KeyEvent{Key: "S"})
	if s.Page != PageBugs || s.IssueView != "open" {
		t.Fatalf("expected 'S' on PageBugs to change status filter back to open, got page=%s view=%s", s.Page, s.IssueView)
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

func TestArrowOptionOnlyModifiesSelectedRowOnAnalysisRun(t *testing.T) {
	s := NewState()
	s.Run = &AnalysisRun{
		Status: "completed",
		Sections: []Section{
			{Category: "bugs", Status: "completed"},
			{Category: "security", Status: "completed"},
		},
	}
	s.Navigate(PageAnalysisRun)
	// Row 0 is a section row ("bugs"), NOT the Batch Files Limit row.
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	s.HandleKey(KeyEvent{Key: " "})
	if s.EffectiveAnalysisConfig().BatchFiles != 64 {
		t.Fatalf("expected ArrowRight/ArrowLeft/Space on section row 0 to leave BatchFiles at 64, got %d", s.EffectiveAnalysisConfig().BatchFiles)
	}

	// Move down to row 2 (Batch Files Limit, after the 2 section rows) and press ArrowRight.
	s.SelectedIdx = 2
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.EffectiveAnalysisConfig().BatchFiles != 128 {
		t.Fatalf("expected ArrowRight on Batch Files Limit row (idx=2) to step BatchFiles to 128, got %d", s.EffectiveAnalysisConfig().BatchFiles)
	}
	// Move down to row 3 (Time Budget) and press ArrowRight: should step BudgetSeconds, not BatchFiles.
	s.SelectedIdx = 3
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.EffectiveAnalysisConfig().BatchFiles != 128 || s.EffectiveAnalysisConfig().BudgetSeconds != 600 {
		t.Fatalf("expected ArrowRight on Time Budget row (idx=3) to step BudgetSeconds to 600 and keep BatchFiles at 128, got batch=%d budget=%d",
			s.EffectiveAnalysisConfig().BatchFiles, s.EffectiveAnalysisConfig().BudgetSeconds)
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

	// 4. Press 'i' (or Enter) on PageAgents -> opens Agent Detail view (AgentDetailOpen=true) on every row; 'd' opens Diff Inspector (PageEditor)
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Page != PageAgents || !s.AgentDetailOpen {
		t.Fatalf("expected AgentDetailOpen=true on PageAgents after 'i' on ready agent, got page=%s detail=%v", s.Page, s.AgentDetailOpen)
	}
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Page != PageAgents || s.AgentDetailOpen {
		t.Fatalf("expected second 'i' on PageAgents to return to Pipeline list, got page=%s detail=%v", s.Page, s.AgentDetailOpen)
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageAgents || !s.AgentDetailOpen {
		t.Fatalf("expected AgentDetailOpen=true on PageAgents after Enter on ready agent, got page=%s detail=%v", s.Page, s.AgentDetailOpen)
	}
	s.HandleKey(KeyEvent{Key: "d"})
	if s.Page != PageEditor {
		t.Fatalf("expected PageEditor after pressing 'd' on ready agent, got %s", s.Page)
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
	if wfModels["create"] != "agy:gemini-2.5-pro" || !domain.ContainsString(confirmed, "agy:gemini-2.5-pro") {
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

	// 4. Verify custom presets, active preset ID, and selected scope models are persisted and reloaded
	s.ExecuteCommand(":model function agy:claude")
	s2 := NewState()
	s2.Project = s.Project
	s2.loadCustomPresetsFromDisk()
	if len(s2.CustomPresets) != 2 || s2.CustomPresets[0].ID != "rust-audit" {
		t.Fatalf("expected custom presets to reload from .mini-orca/tui-presets.json, got %+v", s2.CustomPresets)
	}
	if s2.ActivePresetID != "rust-audit" ||
		s2.AssignedScopeModelID("analyze") != "pi:qwen" ||
		s2.AssignedScopeModelID("bug") != "agy:claude" ||
		s2.AssignedScopeModelID("function") != "agy:claude" {
		t.Fatalf("expected active preset and selected scope models to persist across restarts, got preset=%q scopes=%+v", s2.ActivePresetID, s2.ScopeModels)
	}

	// 5. Render PageModels and verify custom preset row is displayed
	frame := s.Render(120, 36)
	if !strings.Contains(frame, "Rust Audit") || !strings.Contains(frame, "Save Preset") {
		t.Fatalf("expected PageModels frame to display saved custom preset 'Rust Audit':\n%s", frame)
	}

	// 6. Delete custom preset via 'd' hotkey
	s.SelectedIdx = len(domain.CanonicalModelScopes()) + len(domain.DefaultModelPresets())
	s.HandleKey(KeyEvent{Key: "d"})
	if len(s.CustomPresets) != 1 {
		t.Fatalf("expected 'd' to delete selected custom preset, remaining=%+v", s.CustomPresets)
	}
}

func TestBatchTwoHighIssuesFixAndRetryAfterTimeout(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "pwd-manager"}
	s.Index = &ProjectIndex{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
		Files: []IndexedFile{
			{Path: "internal/utils/utils.go", Language: "Go"},
			{Path: "internal/utils/utils_test.go", Language: "Go"},
		},
	}
	s.Results = map[string]SectionResults{
		"security": {
			Security: []SecurityReport{
				{
					Path:   "internal/utils/utils.go",
					Source: "ai",
					Findings: []SecurityFinding{
						{
							ID:                "security:0953f1",
							Title:             "Unvalidated derived key length",
							Severity:          "high",
							ObservedCondition: "verifyPBKDF2 uses the stored hash byte length directly.",
							Remediation:       "Validate fixed salt and derived key lengths before comparison.",
							VerificationIdea:  "Reject single-byte hex derived key in unit test.",
							Triage:            "open",
							SourceAnchor:      SourceAnchor{Path: "internal/utils/utils.go", StartLine: 84, Symbol: "verifyPBKDF2"},
						},
					},
				},
			},
		},
		"performance": {
			Performance: []PerformanceReport{
				{
					Path: "internal/utils/utils.go",
					Findings: []PerformanceFinding{
						{
							ID:               "performance:5bf047",
							Title:            "Per-iteration HMAC allocation in PBKDF2",
							PotentialImpact:  "high",
							ObservedPattern:  "pbkdf2SHA256 allocates hmac.New on every iteration.",
							Recommendation:   "Instantiate HMAC once outside the loop and reuse via Reset.",
							VerificationPlan: "Verify PBKDF2 output vectors and reduced allocations.",
							StartLine:        101,
							Symbol:           "pbkdf2SHA256",
						},
					},
				},
			},
		},
	}

	s.Navigate(PageBugs)
	s.HandleKey(KeyEvent{Key: " "})
	s.HandleKey(KeyEvent{Key: "j"})
	s.HandleKey(KeyEvent{Key: " "})
	s.HandleKey(KeyEvent{Key: "Enter"})

	var workflowMessages []string
	var openCriteria [][]string
	checksCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/projects/current/changes" && r.Method == http.MethodPost:
			var body struct {
				AcceptanceCriteria []string `json:"acceptance_criteria"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			openCriteria = append(openCriteria, body.AcceptanceCriteria)
			_, _ = w.Write([]byte(`{"id":"chg-batch","project_id":"p1","project_revision":"r1","title":"Batch fix (2 issues)","state":"draft","revision":1,"hash":"h1","targets":[{"path":"internal/utils/utils.go","exists":true,"content":"package utils\n"},{"path":"internal/utils/utils_test.go","exists":true,"content":"package utils\n"}],"finding_targets":[{"project_id":"p1","kind":"security","id":"security:0953f1","path":"internal/utils/utils.go","source":"ai"},{"project_id":"p1","kind":"performance","id":"performance:5bf047","path":"internal/utils/utils.go"}]}`))
		case r.URL.Path == "/api/projects/current/changes/chg-batch/workflow" && r.Method == http.MethodPost:
			var body struct {
				Message string `json:"message"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			workflowMessages = append(workflowMessages, body.Message)
			_, _ = w.Write([]byte(`{"id":"chg-batch","project_id":"p1","project_revision":"r1","title":"Batch fix (2 issues)","state":"draft","revision":2,"hash":"h1","targets":[{"path":"internal/utils/utils.go","exists":true,"content":"package utils\n"},{"path":"internal/utils/utils_test.go","exists":true,"content":"package utils\n"}],"finding_targets":[{"project_id":"p1","kind":"security","id":"security:0953f1","path":"internal/utils/utils.go","source":"ai"},{"project_id":"p1","kind":"performance","id":"performance:5bf047","path":"internal/utils/utils.go"}],"workflow":{"id":"wf-1","status":"failed","reason":"Workflow timed out. Reduce scope or check the selected provider.","stages":[{"name":"create","status":"failed"}]}}`))
		case r.URL.Path == "/api/projects/current/changes/chg-batch/checks":
			checksCalled = true
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"type":"bad_request","message":"AI workflow request failed","detail":"prepare a proposal before checking"}`))
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

	// 1. First dispatch includes both findings' details and acceptance criteria
	s.FlushActions(context.Background(), cli)
	if len(workflowMessages) != 1 || len(openCriteria) != 1 || len(openCriteria[0]) != 2 {
		t.Fatalf("expected 1 workflow dispatch with 2 acceptance criteria, got msgs=%d criteria=%+v", len(workflowMessages), openCriteria)
	}
	for _, want := range []string{
		"Unvalidated derived key length",
		"internal/utils/utils.go:84 · verifyPBKDF2",
		"Validate fixed salt and derived key lengths",
		"Per-iteration HMAC allocation in PBKDF2",
		"internal/utils/utils.go:101 · pbkdf2SHA256",
		"Instantiate HMAC once outside the loop",
	} {
		if !strings.Contains(workflowMessages[0], want) {
			t.Fatalf("expected batch workflow message to contain %q, got:\n%s", want, workflowMessages[0])
		}
	}

	// 2. When Stage 1 times out with empty Changes (but non-empty Targets), pressing 'r' retries the workflow instead of calling /checks
	s.HandleKey(KeyEvent{Key: "r"})
	s.FlushActions(context.Background(), cli)
	if checksCalled || s.Error != "" {
		t.Fatalf("expected 'r' after Stage 1 timeout to retry workflow without calling /checks, checksCalled=%v err=%q", checksCalled, s.Error)
	}
	if len(workflowMessages) != 2 || !strings.Contains(workflowMessages[1], "Unvalidated derived key length") || !strings.Contains(workflowMessages[1], "Per-iteration HMAC allocation in PBKDF2") {
		t.Fatalf("expected retry workflow message to preserve both findings' full details, got:\n%v", workflowMessages)
	}
}

func TestIssuesSpacebarBatchEnterAndSingleRowFixKey(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "demo"}
	s.Findings = []Finding{
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "bug-1",
			Severity:        "high",
			Title:           "First high bug",
			Message:         "First cause",
			Category:        "bugs",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/a.go", StartLine: 10},
		},
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "bug-2",
			Severity:        "high",
			Title:           "Second high bug",
			Message:         "Second cause",
			Category:        "bugs",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/b.go", StartLine: 20},
		},
		{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			ID:              "bug-3",
			Severity:        "medium",
			Title:           "Third medium bug",
			Message:         "Third cause",
			Category:        "bugs",
			Status:          "open",
			Location:        SourceAnchor{Path: "internal/app/c.go", StartLine: 30},
		},
	}

	s.Navigate(PageBugs)
	// Select bug-1 and bug-2 with Spacebar
	s.HandleKey(KeyEvent{Key: " "})
	s.HandleKey(KeyEvent{Key: "j"})
	s.HandleKey(KeyEvent{Key: " "})
	if len(s.SelectedIssueIDs) != 2 {
		t.Fatalf("expected 2 issues selected with Spacebar, got %d", len(s.SelectedIssueIDs))
	}

	// Move to bug-3 and press 'f' to fix ONLY the currently highlighted row (bug-3)
	s.HandleKey(KeyEvent{Key: "j"})
	s.HandleKey(KeyEvent{Key: "f"})
	if s.Page != PageAgents {
		t.Fatalf("expected 'f' on PageBugs to navigate to PageAgents, got %s", s.Page)
	}
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "start-agent" || s.PendingActions[0].Seed == nil || s.PendingActions[0].Seed.Title != "Third medium bug" {
		t.Fatalf("expected 'f' to dispatch only selected row 'Third medium bug', got %+v", s.PendingActions)
	}
	// Spacebar selection for bug-1 and bug-2 remains intact until batch is run
	if len(s.SelectedIssueIDs) != 2 {
		t.Fatalf("expected remaining 2 Spacebar-selected issues to stay selected, got %d", len(s.SelectedIssueIDs))
	}

	// Return to PageBugs and press Enter -> runs the 2 Spacebar-selected issues in batch
	s.PendingActions = nil
	s.Navigate(PageBugs)
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageAgents {
		t.Fatalf("expected Enter on PageBugs to navigate to PageAgents, got %s", s.Page)
	}
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "start-agent" || s.PendingActions[0].Seed == nil || len(s.PendingActions[0].Seed.FindingTargets) != 2 {
		t.Fatalf("expected Enter to dispatch batch fix for 2 issues, got %+v", s.PendingActions)
	}
	if len(s.SelectedIssueIDs) != 0 {
		t.Fatalf("expected batch dispatch to clear SelectedIssueIDs, got %d", len(s.SelectedIssueIDs))
	}
}

func TestConcurrentRunningAgentsTrackedWithoutConflict(t *testing.T) {
	var openedCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"trusted":true}`))
	})
	mux.HandleFunc("/api/projects/current/changes", func(w http.ResponseWriter, _ *http.Request) {
		openedCount++
		if openedCount == 1 {
			_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","revision":1,"hash":"h1","state":"draft","title":"Mission 1","targets":[{"path":"a.go"},{"path":"a_test.go"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"chg-2","project_id":"p1","project_revision":"r1","revision":1,"hash":"h1","state":"draft","title":"Mission 2","targets":[{"path":"a.go"},{"path":"a_test.go"}]}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1/workflow", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","revision":2,"hash":"h1","state":"draft","title":"Mission 1","targets":[{"path":"a.go"},{"path":"a_test.go"}],"workflow":{"id":"chg-1-2","status":"running","stages":[{"name":"create","status":"running"},{"name":"test","status":"pending"},{"name":"review","status":"pending"},{"name":"human_review","status":"blocked"}]}}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-2/workflow", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"id":"chg-2","project_id":"p1","project_revision":"r1","revision":2,"hash":"h1","state":"draft","title":"Mission 2","targets":[{"path":"a.go"},{"path":"a_test.go"}],"workflow":{"id":"chg-2-2","status":"running","stages":[{"name":"create","status":"running"},{"name":"test","status":"pending"},{"name":"review","status":"pending"},{"name":"human_review","status":"blocked"}]}}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-1", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-1","project_id":"p1","project_revision":"r1","revision":3,"hash":"h2","state":"draft","title":"Mission 1","workflow":{"id":"chg-1-2","status":"awaiting_human_review","stages":[{"name":"create","status":"completed"},{"name":"test","status":"completed"},{"name":"review","status":"completed"},{"name":"human_review","status":"waiting"}]}}`))
	})
	mux.HandleFunc("/api/projects/current/changes/chg-2", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"chg-2","project_id":"p1","project_revision":"r1","revision":3,"hash":"h2","state":"draft","title":"Mission 2","workflow":{"id":"chg-2-2","status":"awaiting_human_review","stages":[{"name":"create","status":"completed"},{"name":"test","status":"completed"},{"name":"review","status":"completed"},{"name":"human_review","status":"waiting"}]}}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	cl, err := NewClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer cl.Close()

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "demo"}
	s.Index = &ProjectIndex{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
		Files:           []IndexedFile{{Path: "a.go", Language: "Go"}, {Path: "a_test.go", Language: "Go"}},
	}
	s.Findings = []Finding{
		{ID: "f1", Category: "bugs", Severity: "high", Title: "Mission 1", Location: SourceAnchor{Path: "a.go", StartLine: 10}, Status: "open"},
		{ID: "f2", Category: "bugs", Severity: "high", Title: "Mission 2", Location: SourceAnchor{Path: "a.go", StartLine: 20}, Status: "open"},
	}

	// Start first agent with 'f'
	s.Navigate(PageBugs)
	s.SelectedIdx = 0
	s.HandleKey(KeyEvent{Key: "f"})
	s.FlushActions(context.Background(), cl)

	// Start second agent while first is still running
	s.Navigate(PageBugs)
	s.SelectedIdx = 0
	s.HandleKey(KeyEvent{Key: "f"})
	s.FlushActions(context.Background(), cl)

	items := s.AgentItems()
	if len(items) != 2 {
		t.Fatalf("expected 2 concurrent agents tracked in AgentItems, got %d (%+v)", len(items), items)
	}
	// PollActiveJobs refreshes both running agents
	s.PollActiveJobs(context.Background(), cl)
	for _, chg := range s.ActiveChanges {
		if chg.Workflow == nil || chg.Workflow.Status != "awaiting_human_review" {
			t.Fatalf("expected PollActiveJobs to update both running workflows, got %+v", chg)
		}
	}
	// Navigating with k/j in PageAgents switches s.Change between the two sessions
	s.Navigate(PageAgents)
	s.HandleKey(KeyEvent{Key: "k"})
	if s.Change == nil || s.Change.ID != "chg-1" {
		t.Fatalf("expected 'k' on PageAgents to select chg-1, got %+v", s.Change)
	}
	s.HandleKey(KeyEvent{Key: "j"})
	if s.Change == nil || s.Change.ID != "chg-2" {
		t.Fatalf("expected 'j' on PageAgents to select chg-2, got %+v", s.Change)
	}
}

func TestWorkflowErrorDiagnosticsAndDebugInspector(t *testing.T) {
	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "demo"}
	s.Change = &ChangeSession{
		ID:        "chg-err",
		Title:     "Fix nil pointer in parser",
		State:     "draft",
		Freshness: "current",
		Targets:   []ChangeTarget{{Path: "internal/parser/parser.go", Exists: true}},
		Checks: []Check{
			{
				Name:     "go test",
				State:    "failed",
				ExitCode: 1,
				Command:  []string{"go", "test", "./..."},
				Output:   "--- FAIL: TestParse (0.01s)\n    parser_test.go:42: expected nil error, got EOF",
			},
		},
		Workflow: &ChangeWorkflow{
			ID:           "wf-err",
			Status:       "failed",
			Reason:       "Tests or source checks failed. Inspect the check evidence before running again. (go test: --- FAIL: TestParse (0.01s))",
			ProbeSummary: "Check failure in go test: TestParse failed at parser_test.go:42",
			Stages: []ChangeWorkflowStage{
				{Name: "create", Status: "completed", Model: &Model{Model: "qwen2.5-coder"}},
				{Name: "test", Status: "failed", Reason: "go test failed with exit code 1", Model: &Model{Model: "qwen2.5-coder"}},
				{Name: "review", Status: "skipped"},
				{Name: "human_review", Status: "blocked"},
			},
		},
	}

	s.Navigate(PageAgents)
	frame := s.Render(120, 44)
	for _, want := range []string{
		"ACTIVE AGENT MISSION CONTROL",
		"2. Verify & Test",
		"go test failed with exit code 1",
		"--- FAIL: TestParse",
		"Check failure in go test",
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("expected PageAgents error diagnostics panel to contain %q:\n%s", want, frame)
		}
	}
	if strings.Contains(frame, "qwen2.5-coder") {
		t.Fatalf("expected Pipeline page to omit model names, got:\n%s", frame)
	}

	// Pressing Enter on a failed workflow opens the full Workflow Debug & Check Inspector instead of PageEditor
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.Page != PageAgents || !s.AgentDetailOpen {
		t.Fatalf("expected Enter on failed workflow to open AgentDetailOpen on PageAgents, got page=%s detail=%v", s.Page, s.AgentDetailOpen)
	}
	debugFrame := s.Render(120, 44)
	for _, want := range []string{
		"WORKFLOW DEBUG & CHECK INSPECTOR",
		"qwen2.5-coder",
		"└─ Error:",
		"go test failed with exit code 1",
		"parser_test.go:42: expected nil error, got EOF",
	} {
		if !strings.Contains(debugFrame, want) {
			t.Fatalf("expected debug inspector frame to contain %q:\n%s", want, debugFrame)
		}
	}
}

func TestStandardWorktreeCommitAndMergeLifecycle(t *testing.T) {
	tmpRoot := t.TempDir()
	var createdWtPath, createdBranch, scopedOpenWt, appliedCommitMsg, mergedWtPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ExecutionTrust{Trusted: true})
	})
	mux.HandleFunc("/api/projects/current/git/worktrees", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		createdWtPath, _ = body["path"].(string)
		createdBranch, _ = body["target_branch"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{"state": "created", "path": createdWtPath})
	})
	mux.HandleFunc("/api/projects/current", func(w http.ResponseWriter, r *http.Request) {
		wt := r.URL.Query().Get("worktree_path")
		if wt != "" {
			_ = json.NewEncoder(w).Encode(Project{
				ProjectIdentity: ProjectIdentity{ProjectID: "p-wt", ProjectRevision: "r-wt-1"},
				Name:            "demo-wt",
				Path:            wt,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(Project{
			ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
			Name:            "demo",
			Path:            tmpRoot,
		})
	})
	mux.HandleFunc("/api/projects/current/changes", func(w http.ResponseWriter, r *http.Request) {
		scopedOpenWt = r.URL.Query().Get("worktree_path")
		_ = json.NewEncoder(w).Encode(ChangeSession{
			ProjectIdentity: ProjectIdentity{ProjectID: "p-wt", ProjectRevision: "r-wt-1"},
			ID:              "chg-wt-1",
			Title:           "Fix race condition",
			Kind:            "fix",
			Revision:        1,
			Hash:            "h-wt-1",
			State:           "draft",
			FindingTargets:  []FindingTarget{{ProjectID: "p1", Kind: "semantic", ID: "b-race", Path: "internal/app/a.go"}},
		})
	})
	mux.HandleFunc("/api/projects/current/changes/chg-wt-1/workflow", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ChangeSession{
			ProjectIdentity: ProjectIdentity{ProjectID: "p-wt", ProjectRevision: "r-wt-1"},
			ID:              "chg-wt-1",
			Title:           "Fix race condition",
			Kind:            "fix",
			Revision:        2,
			Hash:            "h-wt-2",
			State:           "ready",
			ReviewedHash:    "h-wt-2",
			FindingTargets:  []FindingTarget{{ProjectID: "p1", Kind: "semantic", ID: "b-race", Path: "internal/app/a.go"}},
			Changes:         []FileChange{{Path: "internal/app/a.go", Content: "package app\n"}},
			Checks:          []Check{{Name: "go test", Required: true, State: "passed"}},
			Workflow:        &ChangeWorkflow{ID: "wf-wt-1", Status: "awaiting_human_review"},
		})
	})
	mux.HandleFunc("/api/projects/current/changes/chg-wt-1/review", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ChangeSession{
			ProjectIdentity: ProjectIdentity{ProjectID: "p-wt", ProjectRevision: "r-wt-1"},
			ID:              "chg-wt-1",
			Title:           "Fix race condition",
			Kind:            "fix",
			Revision:        2,
			Hash:            "h-wt-2",
			State:           "ready",
			ReviewedHash:    "h-wt-2",
			FindingTargets:  []FindingTarget{{ProjectID: "p1", Kind: "semantic", ID: "b-race", Path: "internal/app/a.go"}},
			Changes:         []FileChange{{Path: "internal/app/a.go", Content: "package app\n"}},
			Checks:          []Check{{Name: "go test", Required: true, State: "passed"}},
			Workflow:        &ChangeWorkflow{ID: "wf-wt-1", Status: "awaiting_human_review"},
		})
	})
	mux.HandleFunc("/api/projects/current/changes/chg-wt-1/apply", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		appliedCommitMsg, _ = body["commit_message"].(string)
		_ = json.NewEncoder(w).Encode(ChangeMutation{
			ProjectIdentity: ProjectIdentity{ProjectID: "p-wt", ProjectRevision: "r-wt-2"},
			SessionID:       "chg-wt-1",
			State:           "applied",
			Hash:            "h-wt-2",
			UndoAvailable:   true,
			Commit: &ChangeCommit{
				GitIdentity: GitIdentity{Head: "commit999", Branch: createdBranch},
				State:       "committed",
				OID:         "commit999",
				Message:     appliedCommitMsg,
			},
		})
	})
	mux.HandleFunc("/api/projects/current/git/merge", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		mergedWtPath, _ = body["path"].(string)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"state": "merged",
			"index": ProjectIndex{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r2"}},
			"repository": GitRepository{
				GitIdentity:     GitIdentity{Head: "merged999", Branch: "main"},
				ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r2"},
				Available:       true,
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	cli, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	s := NewState()
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"}, Name: "demo", Path: tmpRoot}
	s.Repository = &GitRepository{GitIdentity: GitIdentity{Head: "base123", Branch: "main"}, Available: true, Root: tmpRoot}
	s.GitWorkspace = &GitWorkspace{
		GitRepository: *s.Repository,
		Worktrees:     []GitWorktree{{GitIdentity: GitIdentity{Head: "base123", Branch: "main"}, Path: tmpRoot, Current: true}},
	}
	s.Findings = []Finding{{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "r1"},
		ID:              "b-race",
		Category:        "bugs",
		Severity:        "high",
		Title:           "Fix race condition",
		Status:          "open",
		Location:        SourceAnchor{Path: "internal/app/a.go", StartLine: 10},
	}}

	// 1. Dispatch issue fix -> creates worktree & opens scoped session
	s.dispatchSingleIssue(IssueRow{
		ID:       "semantic:b-race",
		Name:     "Fix race condition",
		Title:    "Fix race condition",
		Path:     "internal/app/a.go",
		Category: "bugs",
		Target:   FindingTarget{ProjectID: "p1", Kind: "semantic", ID: "b-race", Path: "internal/app/a.go"},
	})
	s.FlushActions(context.Background(), cli)

	if createdWtPath == "" || !strings.HasPrefix(createdBranch, "orca/fix-") || scopedOpenWt != createdWtPath {
		t.Fatalf("expected standard worktree creation on issue fix, got wt=%q branch=%q scoped=%q", createdWtPath, createdBranch, scopedOpenWt)
	}
	if !s.HasActiveProposal() || s.Change.WorktreePath != createdWtPath {
		t.Fatalf("expected active proposal in worktree %q, got %+v", createdWtPath, s.Change)
	}
	if len(s.AgentItems()) != 1 {
		t.Fatalf("expected 1 active pipeline item before commit, got %d", len(s.AgentItems()))
	}

	// 2. Apply & commit -> commits with unique code, merges worktree into main, deletes from pipeline, and clears active proposal
	s.applyAndCommitProposal("Fix race condition")
	s.FlushActions(context.Background(), cli)

	if mergedWtPath != createdWtPath {
		t.Fatalf("expected worktree %q to be merged, got %q", createdWtPath, mergedWtPath)
	}
	if !strings.Contains(appliedCommitMsg, "[ORC-") {
		t.Fatalf("expected commit message to include unique ORC code, got %q", appliedCommitMsg)
	}
	if !strings.Contains(s.CmdOutput, "Committed & merged proposal") || !strings.Contains(s.CmdOutput, "into main") {
		t.Fatalf("expected explicit commit & merge confirmation in CmdOutput, got %q", s.CmdOutput)
	}
	if s.HasActiveProposal() || s.Change != nil {
		t.Fatalf("expected active proposal to be cleared after commit and merge, got %+v", s.Change)
	}
	if len(s.AgentItems()) != 0 {
		t.Fatalf("expected completed issue to be deleted from pipeline items, got %+v", s.AgentItems())
	}
	frame := s.Render(120, 40)
	if strings.Contains(frame, "Active Proposal:") {
		t.Fatalf("expected Active Proposal banner to be gone after commit and merge:\n%s", frame)
	}
	if len(s.CompletedFindings) != 1 || s.CompletedFindings[0].CommitCode == "" {
		t.Fatalf("expected persisted completed finding with commit code, got %+v", s.CompletedFindings)
	}
}

func TestIssueDetailTargetsCursorAndIsolatesShortcuts(t *testing.T) {
	s := NewState()
	s.Index = &ProjectIndex{
		Files: []IndexedFile{
			{
				Path:      "internal/app/second.go",
				Language:  "Go",
				LineCount: 120,
				Symbols: []SymbolInfo{
					{
						Name:      "ValidateToken",
						Kind:      "func",
						Signature: "func ValidateToken(tok string) error",
						StartLine: 20,
						EndLine:   45,
					},
				},
			},
			{
				Path:     "internal/app/second_test.go",
				Language: "Go",
			},
		},
	}
	s.Findings = []Finding{
		{
			ID:       "f-first",
			Severity: "critical",
			Title:    "First critical finding",
			Message:  "First root cause",
			Category: "bugs",
			Status:   "open",
			Location: SourceAnchor{Path: "internal/app/first.go", StartLine: 10},
		},
		{
			ID:         "f-second",
			Severity:   "high",
			Confidence: "high",
			Category:   "security",
			Rule:       "SEC-JWT-01",
			Title:      "Second targeted finding",
			Message:    "Second root cause",
			Evidence:   "if token == \"\" { return nil }",
			Status:     "open",
			Freshness:  "current",
			Location:   SourceAnchor{Path: "internal/app/second.go", StartLine: 25, EndLine: 32, Symbol: "ValidateToken"},
			TaskSpec: &TaskSpec{
				TargetSymbol:       "ValidateToken",
				TargetSignature:    "func ValidateToken(tok string) error",
				AcceptanceCriteria: []string{"Reject empty token in ValidateToken", "Cover empty token in unit test"},
				NonGoals:           []string{"Do not alter token signing format"},
			},
		},
		{
			ID:       "f-third",
			Severity: "medium",
			Title:    "Third medium finding",
			Message:  "Third root cause",
			Category: "performance",
			Status:   "open",
			Location: SourceAnchor{Path: "internal/app/third.go", StartLine: 40},
		},
	}

	s.Navigate(PageBugs)
	// Move cursor down to 2nd issue (index 1) and press 'i' to inspect
	s.HandleKey(KeyEvent{Key: "j"})
	if s.SelectedIdx != 1 {
		t.Fatalf("expected SelectedIdx=1 on PageBugs, got %d", s.SelectedIdx)
	}
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Page != PageIssueDetail {
		t.Fatalf("expected PageIssueDetail after 'i', got %s", s.Page)
	}
	cur := s.SelectedIssue()
	if cur == nil || cur.ID != "semantic:f-second" {
		t.Fatalf("expected Issue Detail to target cursor issue f-second, got %+v", cur)
	}
	detailFrame := s.Render(120, 44)
	if !strings.Contains(detailFrame, "Second targeted finding") || strings.Contains(detailFrame, "First critical finding") {
		t.Fatalf("expected Issue Detail frame to render targeted 2nd issue, got:\n%s", detailFrame)
	}
	if strings.Contains(detailFrame, "Finding:") || strings.Contains(detailFrame, "semantic:f-second") {
		t.Fatalf("expected Issue Detail metadata panel to omit Finding row and finding ID in title row:\n%s", detailFrame)
	}
	for _, want := range []string{
		"Issue 2 of 3",
		"Confidence:",
		"HIGH",
		"SEC-JWT-01",
		"internal/app/second.go:25-32",
		"ValidateToken",
		"func ValidateToken(tok string) error",
		"Go ·",
		"120 lines",
		"if token == \"\" { return nil }",
		"✔ Recommended Fix:",
		"Enforce strict authentication/cryptographic validation",
		"▸ Verification:",
		"Reject empty token in ValidateToken",
		"Cover empty token in unit test",
		"▸ Guardrails:",
		"Do not alter token signing format",
		"▸ Next Action:",
		"internal/app/second_test.go",
	} {
		if !strings.Contains(detailFrame, want) {
			t.Fatalf("expected Issue Detail frame to contain enriched field %q:\n%s", want, detailFrame)
		}
	}

	// Background Issues shortcuts ('s', 't', 'Space', 'x') must NOT fire while in PageIssueDetail
	s.HandleKey(KeyEvent{Key: "s"})
	s.HandleKey(KeyEvent{Key: "t"})
	s.HandleKey(KeyEvent{Key: " "})
	s.HandleKey(KeyEvent{Key: "x"})
	if s.IssueView != "open" || s.IssueCategory != "all" || len(s.SelectedIssueIDs) != 0 {
		t.Fatalf("expected background Issues shortcuts to be blocked on PageIssueDetail, got view=%s cat=%s sel=%v",
			s.IssueView, s.IssueCategory, s.SelectedIssueIDs)
	}

	// Left/Right arrows and j/k step between issues inside PageIssueDetail without changing IssueView status
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.IssueView != "open" || s.SelectedIssue() == nil || s.SelectedIssue().ID != "semantic:f-first" {
		t.Fatalf("expected ArrowLeft in PageIssueDetail to step to 1st issue without changing status, got view=%s issue=%+v",
			s.IssueView, s.SelectedIssue())
	}
	firstDetailFrame := s.Render(120, 44)
	if strings.Contains(firstDetailFrame, "Synthesize a minimal, verified fix") {
		t.Fatalf("expected specific Recommended Fix instead of generic placeholder on 1st issue:\n%s", firstDetailFrame)
	}
	for _, wantFirst := range []string{"Update internal/app/first.go:10 to resolve", "First critical finding", "internal/app/first_test.go"} {
		if !strings.Contains(firstDetailFrame, wantFirst) {
			t.Fatalf("expected 1st issue detail frame to include specific remediation context %q:\n%s", wantFirst, firstDetailFrame)
		}
	}
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	s.HandleKey(KeyEvent{Key: "j"})
	if s.SelectedIssue() == nil || s.SelectedIssue().ID != "semantic:f-third" {
		t.Fatalf("expected ArrowRight + j in PageIssueDetail to step to 3rd issue, got %+v", s.SelectedIssue())
	}

	// Pressing 'i' (or Esc) returns to PageBugs keeping cursor on the inspected 3rd issue (index 2)
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Page != PageBugs || s.SelectedIdx != 2 {
		t.Fatalf("expected 'i' on PageIssueDetail to return to PageBugs at index 2, got page=%s idx=%d", s.Page, s.SelectedIdx)
	}
}

func TestAgentRepairInWorktreeDoesNotDuplicateWorktreeRow(t *testing.T) {
	wtPath := "/repo/.mini-orca/autopilot/fix-discarded-clipboard-copy-d7901b"
	s := NewState()
	s.Project = &Project{
		Name:            "mini-orca",
		Path:            "/repo",
		ProjectIdentity: ProjectIdentity{ProjectID: "proj-main", ProjectRevision: "rev-main-1"},
	}
	s.GitWorkspace = &GitWorkspace{
		GitRepository: GitRepository{
			Available:   true,
			Root:        "/repo",
			GitIdentity: GitIdentity{Head: "aaaa1111", Branch: "main"},
		},
		Worktrees: []GitWorktree{
			{
				Path:        wtPath,
				GitIdentity: GitIdentity{Head: "bbbb2222", Branch: "orca/fix-discarded-clipboard-copy-d7901b"},
				Mergeable:   true,
			},
		},
	}
	chg := &ChangeSession{
		ID:              "chg-wt-1",
		ProjectIdentity: ProjectIdentity{ProjectID: "proj-wt", ProjectRevision: "rev-wt-1"},
		WorktreePath:    wtPath,
		Kind:            "fix",
		Title:           "Discarded clipboard copy",
		State:           "proposed",
		Revision:        2,
		Hash:            "h-wt-1",
		Changes:         []FileChange{{Path: "internal/tui/clipboard.go", Content: "package tui\n"}},
		Checks: []Check{
			{Name: "go test", State: "failed", Required: true},
		},
		Workflow: &ChangeWorkflow{
			ID:     "wf-wt-1",
			Status: "failed",
			Stages: []ChangeWorkflowStage{
				{Name: "Architect & Code", Status: "completed"},
				{Name: "Verify & Test", Status: "failed"},
			},
		},
	}
	s.Change = chg
	s.ActiveChanges = []*ChangeSession{chg}
	s.Navigate(PageAgents)

	// 1. Pressing 'r' when checks failed on a worktree session must trigger repair-change (not treat as stale due to worktree ProjectRevision != main ProjectRevision)
	s.HandleKey(KeyEvent{Key: "r"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "repair-change" || s.PendingActions[0].Target != "chg-wt-1" {
		t.Fatalf("expected 'r' on failed worktree session to enqueue repair-change, got %+v", s.PendingActions)
	}
	items := s.AgentItems()
	if len(items) != 1 || items[0].ScopeBadge == "WORKTREE" || strings.HasPrefix(items[0].Title, "Worktree ") {
		t.Fatalf("expected a single agent row without duplicate WORKTREE row after 'r', got %+v", items)
	}

	// 2. Even when 'r' restarts a rejected/empty worktree session, WorktreePath is preserved and no WORKTREE row appears
	s.PendingActions = nil
	s.Change.Changes = nil
	s.HandleKey(KeyEvent{Key: "r"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "start-agent" || s.PendingActions[0].Seed == nil || s.PendingActions[0].Seed.WorktreePath != wtPath {
		t.Fatalf("expected restart seed to preserve WorktreePath %q, got %+v", wtPath, s.PendingActions)
	}
	itemsAfterRestart := s.AgentItems()
	if len(itemsAfterRestart) != 1 || itemsAfterRestart[0].ScopeBadge == "WORKTREE" || strings.HasPrefix(itemsAfterRestart[0].Title, "Worktree ") {
		t.Fatalf("expected a single agent row without duplicate WORKTREE row after restart, got %+v", itemsAfterRestart)
	}
	frame := s.Render(120, 36)
	if strings.Contains(frame, "Worktree orca/") || strings.Contains(frame, "WORKTREE") {
		t.Fatalf("expected Agent Studio frame not to contain duplicate Worktree row:\n%s", frame)
	}
}
