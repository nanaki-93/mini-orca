package tui

import (
	"fmt"
	"strings"
	"testing"
)

func workspaceSampleState() *State {
	s := NewState()
	s.Connected = true
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "fixture", ProjectRevision: "rev-1"}, Name: "atlas", Path: "/workspace/atlas", Type: "Go", FileCount: 128, TotalLines: 18420}
	s.Repository = &GitRepository{Available: true, GitIdentity: GitIdentity{Branch: "main"}}
	s.WorkspaceTrust = true
	s.Models = &ModelCatalog{Scopes: map[string]Model{"analyze": {Model: "local-analyst"}, "bug": {Model: "local-reviewer"}, "function": {Model: "local-coder"}}}
	s.Findings = []Finding{
		{ID: "timeout", Title: "Propagate request deadlines to the storage client", Message: "The downstream call ignores cancellation.", Severity: "high", Category: "bugs", Status: "open", Location: SourceAnchor{Path: "internal/storage/client.go", StartLine: 84}},
		{ID: "path", Title: "Validate upload paths before opening files", Message: "Check containment before writing uploads.", Severity: "high", Category: "security", Status: "open", Location: SourceAnchor{Path: "internal/api/uploads.go", StartLine: 32}},
		{ID: "cache", Title: "Avoid rebuilding the route index on every request", Severity: "medium", Category: "performance", Status: "open", Location: SourceAnchor{Path: "internal/router/routes.go", StartLine: 117}},
	}
	s.Features = &FeatureReport{Suggestions: []FeatureSuggestion{{ID: "export", Title: "Export a project health report", Benefit: "Share verified findings with the team.", Effort: "small", Status: "open"}}}
	s.Scan = &Scan{Status: "passed", Phases: []Check{{Name: "Unit tests", State: "passed", Command: []string{"go", "test", "./..."}, Output: "ok atlas/internal/api\nok atlas/internal/storage"}}}
	s.Run = &AnalysisRun{Status: "completed", Sections: []Section{{Category: "bugs", Status: "completed", Coverage: Coverage{Total: 128, Succeeded: 128}}}}
	return s
}

func TestDashboardNavigationAndUnknownResults(t *testing.T) {
	s := workspaceSampleState()
	s.Scan, s.Run = nil, nil
	frame := s.Render(120, 30)
	if strings.Count(frame, "Not run yet") != 2 || strings.Contains(frame, "[CLEAN]") {
		t.Fatal("unrun analysis or checks must not appear clean")
	}
	for i, page := range dashboardPages {
		s.Navigate(PageSummary)
		s.SelectedIdx = i
		s.HandleKey(KeyEvent{Key: "Enter"})
		if s.Page != page || len(s.PendingActions) != 0 {
			t.Fatalf("dashboard navigation started work: %s %+v", s.Page, s.PendingActions)
		}
	}
	s = NewState()
	s.HandleKey(KeyEvent{Key: "Enter"})
	if !s.CmdActive || s.CmdInput != "project " || len(s.PendingActions) != 0 {
		t.Fatal("welcome must open an editable project prompt")
	}
}

func TestLongProjectInformationAndCheckLogsRemainReachable(t *testing.T) {
	s := workspaceSampleState()
	s.Index = &ProjectIndex{}
	for i := 0; i < 80; i++ {
		s.Index.Files = append(s.Index.Files, IndexedFile{Path: fmt.Sprintf("internal/package/file_%02d.go", i), Language: "Go"})
	}
	s.Navigate(PageRepo)
	s.Render(80, 24)
	s.HandleKey(KeyEvent{Key: "End"})
	if !strings.Contains(s.Render(80, 24), "file_79.go") {
		t.Fatal("repository inventory was truncated")
	}
	s.Scan.Phases[0].Output = strings.Repeat("A detailed diagnostic line\n", 50) + "LAST_DIAGNOSTIC"
	s.Navigate(PageProject)
	s.Render(80, 24)
	s.HandleKey(KeyEvent{Key: "End"})
	if !strings.Contains(s.Render(80, 24), "LAST_DIAGNOSTIC") {
		t.Fatal("check diagnostics were truncated")
	}
	if len(s.PendingActions) != 0 {
		t.Fatal("scrolling must not execute code")
	}
}

func TestLongDialogsKeepActionsAndDiagnosticsReachable(t *testing.T) {
	s := NewState()
	s.Error = strings.Repeat("Long failure explanation. ", 100) + "LAST_ERROR"
	s.Render(60, 20)
	s.HandleKey(KeyEvent{Key: "End"})
	frame := s.Render(60, 20)
	if !strings.Contains(frame, "LAST_ERROR") || !strings.Contains(frame, "[Esc]") {
		t.Fatal("error recovery or diagnostic tail is hidden")
	}
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.CmdActive || len(s.PendingActions) != 0 {
		t.Fatal("error overlay activated the hidden page")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	confirmed := false
	s.Confirmation = &Confirmation{Title: "Review scope", Message: strings.Repeat("A long project path. ", 100) + "LAST_SCOPE", OnConfirm: func(*State) { confirmed = true }}
	s.Render(60, 20)
	s.HandleKey(KeyEvent{Key: "End"})
	frame = s.Render(60, 20)
	if !strings.Contains(frame, "LAST_SCOPE") || !strings.Contains(frame, "[y/Enter]") || !strings.Contains(frame, "[n/Esc]") {
		t.Fatal("confirmation lost full scope or its pinned actions")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if confirmed || s.Confirmation != nil {
		t.Fatal("cancel must not confirm a scrolled dialog")
	}
}

func TestIssueSelectionAndSourceNavigationAtSmallSizes(t *testing.T) {
	s := workspaceSampleState()
	for i := 0; i < 80; i++ {
		s.Findings = append(s.Findings, Finding{ID: fmt.Sprint(i), Title: fmt.Sprintf("Finding %02d", i), Severity: "high", Status: "open", Category: "bugs"})
	}
	s.Navigate(PageBugs)
	for i := 0; i < len(s.FilteredIssues()); i++ {
		frame := s.Render(60, 20)
		if !strings.Contains(frame, s.SelectedIssue().Name) {
			// Very long titles remain available in the selected item's scrollable details.
			if len(s.SelectedIssue().Name) < 20 {
				t.Fatalf("selection %d disappeared at small terminal size", i)
			}
		}
		s.HandleKey(KeyEvent{Key: "ArrowDown"})
	}
	s.Navigate(PageEditor)
	s.File = &FileInfo{Path: "long.go", Content: strings.Repeat("short line\n", 50) + strings.Repeat(" ", 60) + "LONG_LINE_END"}
	s.Render(80, 24)
	s.HandleKey(KeyEvent{Key: "End"})
	for i := 0; i < 5; i++ {
		s.HandleKey(KeyEvent{Key: "l"})
	}
	if !strings.Contains(s.Render(80, 24), "LONG_LINE_END") || len(s.PendingActions) != 0 {
		t.Fatal("source navigation lost long lines or dispatched work")
	}
}

func TestDiffKeepsCodeVisibleAtMinimumViewport(t *testing.T) {
	for _, mode := range []string{"hunk", "unified", "split"} {
		s := NewState()
		s.Navigate(PageEditor)
		s.Change = &ChangeSession{ID: "fixture", State: "checked", Title: strings.Repeat("Long proposal title ", 10), Changes: []FileChange{{Path: "main.go", Diff: FileDiff{Lines: []DiffLine{{Kind: "delete", OldLine: 1, Text: "BEFORE_CODE"}, {Kind: "add", NewLine: 1, Text: "AFTER_CODE"}}}}}}
		s.setDiffViewMode(mode)
		for _, dims := range [][2]int{{52, 16}, {60, 20}, {80, 24}, {100, 20}} {
			frame := s.Render(dims[0], dims[1])
			assertFrameSize(t, frame, dims[0], dims[1])
			if !strings.Contains(frame, "BEFORE_CODE") && !strings.Contains(frame, "AFTER_CODE") {
				t.Fatalf("%s diff lost code at %v", mode, dims)
			}
			if mode == "split" && (!strings.Contains(frame, "BEFORE_CODE") || !strings.Contains(frame, "AFTER_CODE")) {
				t.Fatalf("split diff lost one side at %v", dims)
			}
		}
	}
}
