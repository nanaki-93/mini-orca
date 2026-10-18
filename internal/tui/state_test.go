package tui

import (
	"context"
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
	if !strings.Contains(s.CmdOutput, "grouped agent fix for 2 issues") {
		t.Fatalf("unexpected command output: %s", s.CmdOutput)
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
