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
	// across ALL pages (including 2-Agent Studio and large 150-row tables)
	for i := 0; i < 60; i++ {
		s.Index = &ProjectIndex{Files: append(s.IndexSafeFiles(), IndexedFile{
			Path:      "internal/very/deep/nested/package/service_handler_implementation.go",
			Language:  "Go",
			LineCount: 420,
		})}
	}
	allPages := []Page{
		PageSummary, PageRepo, PageProject, PageAgents, PageBugs, PageIssueDetail,
		PageFeatures, PageAnalysisRun, PageAnalysisFiles, PageEditor, PageGit,
		PageHistory, PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams,
		PageInstructions, PageModels, PageBootstrap, PageLessons,
	}
	for _, dims := range [][2]int{{68, 24}, {100, 30}, {120, 36}, {180, 50}} {
		for _, p := range allPages {
			s.Navigate(p)
			out := s.Render(dims[0], dims[1])
			rows := strings.Split(out, "\n")
			if len(rows) != dims[1] {
				t.Fatalf("page %s (%dx%d): expected exact frame height %d, got %d rows", p, dims[0], dims[1], dims[1], len(rows))
			}
			for rIdx, row := range rows {
				if gotW := visibleRuneCount(row); gotW != dims[0] {
					t.Fatalf("page %s (%dx%d) row %d: expected exact visible width %d, got %d: %q", p, dims[0], dims[1], rIdx, dims[0], gotW, row)
				}
			}
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

	// Verify sidebar highlight moves in Sections 4 and 5 when stepping through submenus
	s.HandleKey(KeyEvent{Key: "4"})
	if !strings.Contains(s.Render(120, 36), "◆ Context & Deps") {
		t.Fatal("expected Section 4 initial highlight on Context & Deps")
	}
	s.HandleKey(KeyEvent{Key: "l"})
	if !strings.Contains(s.Render(120, 36), "◆ Components") {
		t.Fatal("expected Section 4 highlight to move to Components after 'l'")
	}
	s.HandleKey(KeyEvent{Key: "5"})
	if !strings.Contains(s.Render(120, 36), "◆ Trust Policy") {
		t.Fatal("expected Section 5 initial highlight on Trust Policy")
	}
	s.HandleKey(KeyEvent{Key: "l"})
	if !strings.Contains(s.Render(120, 36), "◆ Model Routing") {
		t.Fatal("expected Section 5 highlight to move to Model Routing after 'l'")
	}

	s.HandleKey(KeyEvent{Key: "?"})
	helpOut := s.Render(120, 36)
	if !strings.Contains(helpOut, "KEYBINDINGS & COLON COMMANDS REFERENCE") {
		t.Fatalf("expected on-demand keybindings modal when pressing '?':\n%s", helpOut)
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
}

func (s *State) IndexSafeFiles() []IndexedFile {
	if s.Index == nil {
		return nil
	}
	return s.Index.Files
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

func TestPageSpecificHelpOverlayAcrossAllPages(t *testing.T) {
	allPages := []Page{
		PageSummary, PageRepo, PageProject,
		PageAgents, PageBugs, PageIssueDetail, PageFeatures, PageAnalysisRun, PageAnalysisFiles,
		PageEditor, PageGit, PageHistory,
		PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams,
		PageInstructions, PageModels, PageBootstrap, PageLessons,
	}
	seenTitles := make(map[string]Page, len(allPages))
	s := NewState()
	s.ShowKeybindsModal = true

	for _, p := range allPages {
		s.Navigate(p)
		spec := lookupPageHelpSpec(s.EffectivePage())
		if prev, exists := seenTitles[spec.title]; exists {
			t.Fatalf("duplicate help spec title %q between %s and %s", spec.title, prev, p)
		}
		seenTitles[spec.title] = p

		frame := s.Render(120, 36)
		for _, token := range []string{
			"KEYBINDINGS & COLON COMMANDS REFERENCE",
			"PAGE SHORTCUTS · " + spec.title,
			spec.primary[0].desc,
			"GLOBAL NAV",
			"GENERIC CMDS",
		} {
			if !strings.Contains(frame, token) {
				t.Fatalf("page %s help overlay missing %q:\n%s", p, token, frame)
			}
		}
	}
}

func TestCodeDiffAndSourceTabAndLongLineSanitization(t *testing.T) {
	s := NewState()
	s.Connected = true

	var diffLines []DiffLine
	for i := 1; i <= 60; i++ {
		diffLines = append(diffLines, DiffLine{
			Kind:    "context",
			OldLine: i,
			NewLine: i,
			Text:    "\t\t// context line with tabs\r",
		})
	}
	diffLines[15] = DiffLine{
		Kind:    "delete",
		OldLine: 16,
		Text:    "\t\t\tReadTimeout: 0, // legacy unbounded timeout with\ttabs\tand\r\n",
	}
	diffLines[16] = DiffLine{
		Kind:    "add",
		NewLine: 16,
		Text:    "\t\t\tReadHeaderTimeout: 5 * time.Second, // " + strings.Repeat("very_long_identifier_", 10),
	}
	diffLines[45] = DiffLine{
		Kind:    "add",
		NewLine: 46,
		Text:    "\t\t\tIdleTimeout: 60 * time.Second,\t// second hunk with tabs\r",
	}

	s.Change = &ChangeSession{
		ID:    "chg-tabs",
		Title: "Harden\tHTTP\nserver timeouts",
		State: "checked",
		Changes: []FileChange{
			{
				Path:    "internal/api/server.go",
				Summary: "Enforce explicit read header timeout\nand\tstrip control chars\r\n",
				Diff:    FileDiff{Lines: diffLines},
			},
		},
	}

	s.Navigate(PageEditor)
	for _, dims := range [][2]int{{68, 24}, {100, 30}, {120, 36}, {180, 50}} {
		frame := s.Render(dims[0], dims[1])
		if strings.ContainsAny(frame, "\t\r") {
			t.Fatalf("diff frame (%dx%d) must not contain raw tab or CR characters", dims[0], dims[1])
		}
		rows := strings.Split(frame, "\n")
		if len(rows) != dims[1] {
			t.Fatalf("diff frame (%dx%d): expected %d rows, got %d", dims[0], dims[1], dims[1], len(rows))
		}
		for rIdx, row := range rows {
			if gotW := visibleRuneCount(row); gotW != dims[0] {
				t.Fatalf("diff frame (%dx%d) row %d: expected width %d, got %d: %q", dims[0], dims[1], rIdx, dims[0], gotW, row)
			}
		}
	}

	// Verify focused hunk view shows changed lines immediately and 'n'/'N' steps hunks
	frame120 := s.Render(120, 36)
	if !strings.Contains(frame120, "ReadHeaderTimeout") || !strings.Contains(frame120, "Hunk #1/2") {
		t.Fatalf("expected focused hunk #1 in diff view:\n%s", frame120)
	}
	s.HandleKey(KeyEvent{Key: "n"})
	if s.HunkIdx != 1 || !strings.Contains(s.CmdOutput, "hunk #2/2") {
		t.Fatalf("expected 'n' to focus hunk #2/2, got HunkIdx=%d CmdOutput=%q", s.HunkIdx, s.CmdOutput)
	}
	s.HandleKey(KeyEvent{Key: "N"})
	if s.HunkIdx != 0 {
		t.Fatalf("expected 'N' to return to hunk #1/2, got HunkIdx=%d", s.HunkIdx)
	}

	// Verify read-only source file viewer with tabs, CRLF, and long lines
	s.Change = nil
	s.File = &FileInfo{
		Path:      "internal/api/server.go",
		Language:  "Go",
		LineCount: 3,
		Content:   "package api\r\n\tfunc Start() {\r\n\t\t\tprintln(\"" + strings.Repeat("A", 200) + "\")\r\n\t}\r\n",
	}
	sourceFrame := s.Render(120, 36)
	if strings.ContainsAny(sourceFrame, "\t\r") {
		t.Fatal("source viewer frame must not contain raw tab or CR characters")
	}
	for rIdx, row := range strings.Split(sourceFrame, "\n") {
		if gotW := visibleRuneCount(row); gotW != 120 {
			t.Fatalf("source frame row %d: expected width 120, got %d: %q", rIdx, gotW, row)
		}
	}
}

func TestMultiFileDiffHunkNavigationAndViewModes(t *testing.T) {
	s := NewState()
	s.Connected = true
	s.Change = &ChangeSession{
		ID:    "chg-multi",
		Title: "Harden server and add regression test",
		State: "checked",
		Changes: []FileChange{
			{
				Path:    "internal/api/server.go",
				Summary: "Add read header timeout",
				Content: "package api\nvar ReadHeaderTimeout = 5\n",
				Diff: FileDiff{
					Lines: []DiffLine{
						{Kind: "context", OldLine: 1, NewLine: 1, Text: "package api"},
						{Kind: "delete", OldLine: 2, NewLine: 0, Text: "var ReadHeaderTimeout = 0"},
						{Kind: "add", OldLine: 0, NewLine: 2, Text: "var ReadHeaderTimeout = 5"},
					},
				},
			},
			{
				Path:    "internal/api/server_test.go",
				Summary: "Verify timeout configuration in unit test",
				Content: "package api\nfunc TestTimeout() {}\n",
				Diff: FileDiff{
					Lines: []DiffLine{
						{Kind: "context", OldLine: 1, NewLine: 1, Text: "package api"},
						{Kind: "add", OldLine: 0, NewLine: 2, Text: "func TestTimeout() {}"},
					},
				},
			},
		},
	}

	s.Navigate(PageEditor)

	// 1. Initially focused on Hunk #1/2 in file 1/2 (server.go)
	frameHunk1 := s.Render(120, 36)
	if !strings.Contains(frameHunk1, "Hunk #1/2") || !strings.Contains(frameHunk1, "ReadHeaderTimeout = 5") {
		t.Fatalf("expected Hunk #1/2 from server.go in initial view:\n%s", frameHunk1)
	}

	// 2. Press 'n' -> must advance to Hunk #2/2 in file 2/2 (server_test.go)
	s.HandleKey(KeyEvent{Key: "n"})
	if s.HunkIdx != 1 || !strings.Contains(s.CmdOutput, "hunk #2/2") || !strings.Contains(s.CmdOutput, "server_test.go") {
		t.Fatalf("expected 'n' to focus hunk #2/2 in server_test.go, got HunkIdx=%d CmdOutput=%q", s.HunkIdx, s.CmdOutput)
	}
	frameHunk2 := s.Render(120, 36)
	if !strings.Contains(frameHunk2, "Hunk #2/2") || !strings.Contains(frameHunk2, "TestTimeout") || !strings.Contains(frameHunk2, "Verify timeout configuration") {
		t.Fatalf("expected Hunk #2/2 and rationale from server_test.go:\n%s", frameHunk2)
	}

	// 3. Press 'f' -> cycles back to file 1/2 (server.go)
	s.HandleKey(KeyEvent{Key: "f"})
	if s.HunkIdx != 0 || !strings.Contains(s.CmdOutput, "server.go") {
		t.Fatalf("expected 'f' to cycle back to file 1/2 (server.go), got HunkIdx=%d CmdOutput=%q", s.HunkIdx, s.CmdOutput)
	}

	// 4. Scroll down with 'j' past the end of Hunk #1 -> automatically advances into Hunk #2
	for range 5 {
		s.HandleKey(KeyEvent{Key: "j"})
	}
	if s.HunkIdx != 1 {
		t.Fatalf("expected scrolling past end of hunk #1 with 'j' to advance to hunk #2, got HunkIdx=%d", s.HunkIdx)
	}

	// 5. Cycle diff view modes with 'v' (hunk -> split -> unified -> hunk) and verify geometry
	for _, expectedMode := range []string{"split", "unified", "hunk"} {
		s.HandleKey(KeyEvent{Key: "v"})
		if got := s.EffectiveDiffMode(); got != expectedMode {
			t.Fatalf("expected diff mode %q after 'v', got %q", expectedMode, got)
		}
		for _, dims := range [][2]int{{68, 24}, {120, 36}} {
			frame := s.Render(dims[0], dims[1])
			rows := strings.Split(frame, "\n")
			if len(rows) != dims[1] {
				t.Fatalf("mode %s (%dx%d): expected %d rows, got %d", expectedMode, dims[0], dims[1], dims[1], len(rows))
			}
			for rIdx, row := range rows {
				if gotW := visibleRuneCount(row); gotW != dims[0] {
					t.Fatalf("mode %s (%dx%d) row %d: expected width %d, got %d: %q", expectedMode, dims[0], dims[1], rIdx, dims[0], gotW, row)
				}
			}
		}
	}

	// 6. Reject hunk #2 in file 2 (server_test.go) using 'x' without affecting file 1
	s.ExecuteCommand(":diff hunk")
	s.HunkIdx = 1
	s.HandleKey(KeyEvent{Key: "x"})
	if strings.Contains(s.Change.Changes[1].Content, "TestTimeout") {
		t.Fatalf("expected rejecting hunk in file 2 to remove TestTimeout from file 2 content, got %q", s.Change.Changes[1].Content)
	}
	if !strings.Contains(s.Change.Changes[0].Content, "ReadHeaderTimeout = 5") {
		t.Fatalf("expected file 1 content to remain intact after rejecting file 2 hunk, got %q", s.Change.Changes[0].Content)
	}
}
