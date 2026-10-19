package tui

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderAllHubsAndPalettes(t *testing.T) {
	s := NewState()
	s.Connected = true
	s.Version = "2.4.0"
	s.Project = &Project{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "rev1"},
		Name:            "mini-orca",
		Path:            "/repo",
		Type:            "go",
		BuildFile:       "go.mod",
		FileCount:       24,
		TotalLines:      4200,
	}
	s.Repository = &GitRepository{GitIdentity: GitIdentity{Branch: "main"}, Available: true}
	s.GitWorkspace = &GitWorkspace{
		GitRepository: *s.Repository,
		Worktrees: []GitWorktree{
			{GitIdentity: GitIdentity{Branch: "main"}, Path: "/repo", Current: true},
			{GitIdentity: GitIdentity{Branch: "fix/issue-1"}, Path: "/repo/.worktrees/1", Mergeable: true},
		},
	}
	s.Index = &ProjectIndex{
		Files: []IndexedFile{{Path: "cmd/tui/main.go", Language: "Go", LineCount: 120}},
	}
	s.Findings = []Finding{
		{
			ID:       "f1",
			Severity: "high",
			Title:    "Unchecked error return",
			Message:  "Check error from Close",
			Category: "bugs",
			Status:   "open",
			Location: SourceAnchor{Path: "cmd/tui/main.go", StartLine: 45},
		},
	}
	s.Features = &FeatureReport{
		Suggestions: []FeatureSuggestion{
			{ID: "feat-1", Title: "Add snapshot export", Benefit: "Faster CI debugging", Effort: "low", Status: "open"},
		},
	}
	s.Run = &AnalysisRun{
		Identity:       RunIdentity{ID: "run-1"},
		Status:         "running",
		ElapsedSeconds: 12,
		Sections:       []Section{{Category: "bugs", Status: "running", Coverage: Coverage{Total: 10, Succeeded: 5}}},
	}
	s.Scan = &Scan{
		Status: "passed",
		Phases: []Check{{Name: "go test", State: "passed", Command: []string{"go", "test", "./..."}}},
	}
	s.ChangeHistory = []ChangeHistoryEntry{
		{ID: "chg-1", Title: "Fix Close error", State: "applied", Revision: 1, Freshness: "current"},
	}

	pages := []Page{
		PageSummary, PageRepo, PageProject, PageAgents, PageBugs, PageIssueDetail,
		PageFeatures, PageAnalysisRun, PageAnalysisFiles, PageEditor, PageGit,
		PageHistory, PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams,
		PageInstructions, PageModels, PageBootstrap, PageLessons,
	}
	for _, p := range pages {
		s.Navigate(p)
		frame := s.Render(120, 40)
		if !strings.Contains(frame, "MINI-ORCA // TUI") {
			t.Fatalf("expected status bar on page %s", p)
		}
		if !strings.Contains(frame, "┏") || !strings.Contains(frame, "┗") {
			t.Fatalf("expected enterprise frame borders on page %s", p)
		}
	}

	// Verify Overview Dashboard does NOT duplicate the raw issues table, while Issues & Issue Detail do
	s.Navigate(PageSummary)
	summaryFrame := s.Render(120, 40)
	if strings.Contains(summaryFrame, "Unchecked error return") {
		t.Fatalf("expected Overview Dashboard to avoid redundant issue table rows:\n%s", summaryFrame)
	}
	s.Navigate(PageBugs)
	bugsFrame := s.Render(120, 40)
	if !strings.Contains(bugsFrame, "Unchecked error return") {
		t.Fatalf("expected Issues page to list finding 'Unchecked error return'")
	}
	s.HandleKey(KeyEvent{Key: "i"})
	if s.Page != PageIssueDetail {
		t.Fatalf("expected 'i' on PageBugs to open PageIssueDetail, got %s", s.Page)
	}
	detailFrame := s.Render(120, 40)
	if !strings.Contains(detailFrame, "ROOT CAUSE DIAGNOSTIC & REMEDIATION") || !strings.Contains(detailFrame, "Check error from Close") {
		t.Fatalf("expected dedicated Issue Detail view with root cause:\n%s", detailFrame)
	}

	// Verify Bubble Tea tea.Model lifecycle (Init, Update, View)
	bm := NewBubbleModel(context.Background(), s, nil, 110, 32)
	_ = bm.Init()
	_, _ = bm.Update(tea.WindowSizeMsg{Width: 130, Height: 38})
	_, _ = bm.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !strings.Contains(bm.View(), "130x38") {
		t.Fatalf("expected BubbleModel View to reflect updated 130x38 window size")
	}

	for _, pal := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s.Palette = pal
		frame := s.Render(100, 30)
		if !strings.Contains(frame, string(pal)) {
			t.Fatalf("expected palette %s in status bar", pal)
		}
	}
}

func TestEnterprisePanelsModalsDiffAndViewports(t *testing.T) {
	s := NewState()
	s.Connected = true
	s.Change = &ChangeSession{
		ID:    "chg-9",
		Title: "Harden HTTP timeout",
		State: "checked",
		Changes: []FileChange{
			{
				Path:    "internal/api/server.go",
				Summary: "Enforce explicit read header timeout",
				Diff: FileDiff{
					Lines: []DiffLine{
						{Kind: "delete", OldLine: 10, Text: "ReadTimeout: 0,"},
						{Kind: "add", NewLine: 10, Text: "ReadHeaderTimeout: 5 * time.Second,"},
					},
				},
			},
		},
		Checks: []Check{{Name: "go test", State: "passed"}},
	}

	// Verify Diff Inspector panel & inline Why
	s.Navigate(PageEditor)
	diffFrame := s.Render(110, 34)
	if !strings.Contains(diffFrame, "GUARDED DIFF REVIEW") || !strings.Contains(diffFrame, "Enforce explicit read header timeout") {
		t.Fatalf("expected enterprise diff review panel and inline why:\n%s", diffFrame)
	}

	// Verify Governance sections (trust, models, bootstrap, lessons)
	s.Navigate(PageInstructions)
	for _, sec := range []string{"trust", "models", "bootstrap", "lessons"} {
		s.RulesSection = sec
		govFrame := s.Render(110, 34)
		if !strings.Contains(govFrame, "[5] GOVERNANCE & RULES") {
			t.Fatalf("expected governance header for section %s", sec)
		}
	}

	// Verify Confirmation Modal
	s.Confirmation = &Confirmation{
		Title:       "Apply Guarded Proposal",
		Message:     "Write verified changes to internal/api/server.go?",
		AcceptLabel: "Apply",
	}
	modalFrame := s.Render(90, 28)
	if !strings.Contains(modalFrame, "CONFIRMATION REQUIRED") || !strings.Contains(modalFrame, "Apply Guarded Proposal") {
		t.Fatalf("expected confirmation modal panel:\n%s", modalFrame)
	}
	s.Confirmation = nil

	// Verify Error Alert Banner and Escape dismissal
	s.Error = "daemon connection refused on 127.0.0.1:9090"
	errFrame := s.Render(110, 34)
	if !strings.Contains(errFrame, "ERROR / DIAGNOSTIC ALERT") || !strings.Contains(errFrame, "daemon connection refused") {
		t.Fatalf("expected prominent error alert banner:\n%s", errFrame)
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.Error != "" {
		t.Fatalf("expected Escape to clear active error, got %q", s.Error)
	}

	// Verify full-screen adaptive width and height filling across compact, standard, and ultrawide viewports
	for _, dims := range [][2]int{{68, 24}, {120, 36}, {180, 50}} {
		s.Navigate(PageSummary)
		out := s.Render(dims[0], dims[1])
		rows := strings.Split(out, "\n")
		if len(rows) < dims[1] {
			t.Fatalf("expected full-screen frame height >= %d, got %d rows", dims[1], len(rows))
		}
		if gotW := visibleRuneCount(rows[0]); gotW != dims[0] {
			t.Fatalf("expected exact full-screen width %d on top border, got %d", dims[0], gotW)
		}
	}

	// Verify split-screen sidebar menu division and hidden keymappings on standard screen
	s.Navigate(PageSummary)
	splitOut := s.Render(120, 36)
	if !strings.Contains(splitOut, "NAVIGATION") || !strings.Contains(splitOut, "WORKSPACE PULSE") || !strings.Contains(splitOut, "┳") {
		t.Fatalf("expected split-screen sidebar menu and divider on 120x36 viewport:\n%s", splitOut)
	}
	if strings.Contains(splitOut, "ACTIONS:") || strings.Contains(splitOut, "[b/Enter]") || strings.Contains(splitOut, "Press [1-5]") {
		t.Fatalf("expected keymappings to be hidden from main screen:\n%s", splitOut)
	}
	s.HandleKey(KeyEvent{Key: "?"})
	helpOut := s.Render(120, 36)
	if !strings.Contains(helpOut, "KEYBINDINGS & COLON COMMANDS REFERENCE") {
		t.Fatalf("expected on-demand keybindings modal when pressing '?':\n%s", helpOut)
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
}

func TestDecodeInputBytesAndInteractiveSession(t *testing.T) {
	raw := []byte("\x1b[A\x1b[B\x1b[C\x1b[D\x1b[Z\x1b\r\t\x7f2:q\n")
	events := DecodeInputBytes(raw)
	wantKeys := []string{
		"ArrowUp", "ArrowDown", "ArrowRight", "ArrowLeft",
		"Tab", "Escape", "Enter", "Tab", "Backspace",
		"2", ":", "q", "Enter",
	}
	if len(events) != len(wantKeys) {
		t.Fatalf("decoded %d events, want %d: %+v", len(events), len(wantKeys), events)
	}
	for i, want := range wantKeys {
		if events[i].Key != want {
			t.Fatalf("event[%d].Key = %q, want %q", i, events[i].Key, want)
		}
	}
	if !events[4].Shift {
		t.Fatal("expected Shift+Tab for \\x1b[Z")
	}

	reindexed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/current/reindex" {
			reindexed = true
			_, _ = w.Write([]byte(`{"project_id":"p1","project_revision":"r2","files":[{"path":"a.go","language":"Go"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	s := NewState()
	input := bytes.NewBufferString(":reindex\n?\x1b:q\n")
	var output bytes.Buffer
	if err := s.RunSession(context.Background(), client, input, &output, 100, 30); err != nil {
		t.Fatalf("RunSession failed: %v", err)
	}
	if !reindexed {
		t.Fatal("expected :reindex action to be flushed to daemon")
	}
	if !s.Quitting {
		t.Fatal("expected session to quit after :q")
	}
	if !strings.Contains(output.String(), "MINI-ORCA // TUI") {
		t.Fatal("expected rendered frames in output")
	}
}
