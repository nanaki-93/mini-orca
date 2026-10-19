package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	}

	s.Navigate(PageBugs)
	if got := len(s.FilteredIssues()); got != 2 {
		t.Fatalf("expected 2 issues in ALL category, got %d", got)
	}

	// Cycle issue type with 't': all -> bugs -> security -> performance
	s.HandleKey(KeyEvent{Key: "t"})
	if s.IssueCategory != "bugs" || len(s.FilteredIssues()) != 1 {
		t.Fatalf("expected bugs category with 1 issue, got %s (%d)", s.IssueCategory, len(s.FilteredIssues()))
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

	// Reject current hunk with 'x'
	s.HandleKey(KeyEvent{Key: "x"})
	if !strings.Contains(s.Change.Changes[0].Content, "var timeout = 5") {
		t.Fatalf("expected hunk rejection to restore old line, got %q", s.Change.Changes[0].Content)
	}

	// Apply and commit with 'c'
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
		capturedPaths   []string
		capturedLimits  map[string]int
		reviewedHash    string
		appliedHead     string
		undoConfirmed   bool
		mergedWorktree  string
		savedExclusions []string
		bootstrapped    bool
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/projects/current/execution-trust", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ProjectRevision string `json:"project_revision"`
			Confirm         bool   `json:"confirm"`
		}
		if r.Method == http.MethodPost {
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ProjectRevision == "" || !body.Confirm {
				w.WriteHeader(http.StatusBadRequest)
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
