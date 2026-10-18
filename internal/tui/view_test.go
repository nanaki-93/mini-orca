package tui

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderAllHubsAndPalettes(t *testing.T) {
	s := NewState()
	s.Connected = true
	s.Project = &Project{
		ProjectIdentity: ProjectIdentity{ProjectID: "p1", ProjectRevision: "rev1"},
		Name:            "mini-orca",
		Path:            "/repo",
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
		PageSummary, PageProject, PageAgents, PageBugs, PageFeatures,
		PageAnalysisRun, PageAnalysisFiles, PageEditor, PageGit,
		PageHistory, PageDocs, PageDiagrams, PageInstructions,
	}
	for _, p := range pages {
		s.Navigate(p)
		frame := s.Render(120, 40)
		if !strings.Contains(frame, "MINI-ORCA // TUI") {
			t.Fatalf("expected status bar on page %s", p)
		}
	}

	for _, pal := range []Palette{PaletteTokyo, PaletteCRT, PaletteAmber, PalettePaper} {
		s.Palette = pal
		frame := s.Render(100, 30)
		if !strings.Contains(frame, string(pal)) {
			t.Fatalf("expected palette %s in status bar", pal)
		}
	}
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
