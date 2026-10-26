package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func twoFileReviewState() *State {
	s := NewState()
	s.Connected = true
	s.Project = &Project{ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, Name: "atlas"}
	s.Change = &ChangeSession{
		ID: "chg-review", ProjectIdentity: ProjectIdentity{ProjectID: "p", ProjectRevision: "r"}, Revision: 2, Hash: "h-2", Title: "Harden server", State: "checked",
		Changes: []FileChange{
			{Path: "internal/api/server.go", Content: "package api\nvar T = 5\n", Diff: FileDiff{Lines: []DiffLine{{Kind: "context", OldLine: 1, NewLine: 1, Text: "package api"}, {Kind: "add", NewLine: 2, Text: "var T = 5"}}}},
			{Path: "internal/api/server_test.go", Content: "package api\nfunc TestT() {}\n", Diff: FileDiff{Lines: []DiffLine{{Kind: "context", OldLine: 1, NewLine: 1, Text: "package api"}, {Kind: "add", NewLine: 2, Text: "func TestT() {}"}}}},
		},
	}
	s.Navigate(PageEditor)
	return s
}

func TestBreadcrumbShowsWorkspaceViewAndFocusedItem(t *testing.T) {
	s := workspaceSampleState()
	s.Navigate(PageBugs)
	frame := ansi.Strip(s.Render(120, 30))
	if !strings.Contains(frame, "Work › Issues") {
		t.Fatalf("issues page breadcrumb missing:\n%s", frame)
	}
	review := twoFileReviewState()
	if frame := ansi.Strip(review.Render(120, 30)); !strings.Contains(frame, "Code › Files & Diff › internal/api/server.go") {
		t.Fatalf("diff breadcrumb must name the active file:\n%s", frame)
	}
	s.ShowKeybindsModal = true
	if line := s.renderBreadcrumb(themeForPalette(s.Palette), 120); line != "" {
		t.Fatalf("overlays must not show a breadcrumb, got %q", line)
	}
	for _, size := range [][2]int{{60, 20}, {80, 24}, {160, 40}} {
		s.ShowKeybindsModal = false
		assertFrameSize(t, s.Render(size[0], size[1]), size[0], size[1])
	}
}

// Blocking daemon actions are deferred by one message so the busy status is
// rendered before the request runs, then cleared when it completes.
func TestBlockingActionsShowBusyStatusBeforeDispatch(t *testing.T) {
	var reindexes atomic.Int32
	s := workspaceSampleState()
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/current/reindex" {
			reindexes.Add(1)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"project_id": "fixture", "project_revision": "rev-1"})
	})
	m := NewBubbleModel(context.Background(), s, cl, 120, 30)
	m.nativeStreams = false
	s.enqueueAction(ActionRequest{Kind: "reindex"})
	cmds := m.flushActionsAsync()
	if reindexes.Load() != 0 {
		t.Fatal("blocking action ran before the busy frame could render")
	}
	if s.Busy != "Reindexing project…" || !strings.Contains(ansi.Strip(s.Render(120, 30)), "Reindexing project…") {
		t.Fatalf("busy status not rendered: busy=%q", s.Busy)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected one deferred command, got %d", len(cmds))
	}
	msg, ok := cmds[0]().(syncActionsMsg)
	if !ok || len(msg.actions) != 1 || msg.actions[0].act.Kind != "reindex" {
		t.Fatalf("unexpected deferred message: %#v", msg)
	}
	m.Update(msg)
	if reindexes.Load() != 1 || s.Busy != "" {
		t.Fatalf("deferred action did not run exactly once or busy stayed set: calls=%d busy=%q", reindexes.Load(), s.Busy)
	}
}

func TestDeferredActionsKeepTheirOrderAndClearBusyAfterErrors(t *testing.T) {
	var order []string
	s := workspaceSampleState()
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	})
	m := NewBubbleModel(context.Background(), s, cl, 120, 30)
	s.enqueueAction(ActionRequest{Kind: "reindex"})
	s.enqueueAction(ActionRequest{Kind: "scan"})
	cmds := m.flushActionsAsync()
	m.Update(cmds[len(cmds)-1]())
	if len(order) < 2 || order[0] != "/api/projects/current/reindex" {
		t.Fatalf("actions ran out of order: %v", order)
	}
	if s.Busy != "" || s.Error == "" {
		t.Fatalf("failure must surface as an error and release busy: busy=%q error=%q", s.Busy, s.Error)
	}
}

func TestSpinnerOnlyAnimatesWhileWorkIsInFlight(t *testing.T) {
	s := workspaceSampleState()
	s.Run = nil
	m := NewBubbleModel(context.Background(), s, nil, 120, 30)
	var cmds []tea.Cmd
	m.maybeScheduleSpinner(&cmds)
	if len(cmds) != 0 || s.showsActivity() {
		t.Fatal("an idle workspace must not schedule spinner ticks")
	}
	s.Busy = "Applying reviewed changes…"
	m.maybeScheduleSpinner(&cmds)
	if len(cmds) != 1 || !m.spinnerScheduled {
		t.Fatal("busy work must schedule one spinner tick")
	}
	m.maybeScheduleSpinner(&cmds)
	if len(cmds) != 1 {
		t.Fatal("spinner ticks must not stack")
	}
	first := s.statusBarActivity()
	m.Update(spinnerTickMsg{})
	if second := s.statusBarActivity(); first == second || !strings.HasSuffix(second, "Applying reviewed changes…") {
		t.Fatalf("spinner did not advance: %q -> %q", first, second)
	}
}

func TestCommandPaletteRanksExactPrefixAndRecentCommands(t *testing.T) {
	s := NewState()
	s.openCommandInput("apply")
	if got := s.commandMatches(); len(got) == 0 || got[0].command != "apply" {
		t.Fatalf("exact command must rank first, got %+v", got)
	}
	s.openCommandInput("aply")
	if got := s.commandMatches(); len(got) == 0 || got[0].command != "apply" {
		t.Fatalf("typo fallback must still find apply, got %+v", got)
	}
	s.openCommandInput("zzzz")
	if got := s.commandMatches(); len(got) != 0 {
		t.Fatalf("unrelated input must not match, got %+v", got)
	}
	s.CmdHistory = []string{"undo"}
	s.openCommandInput("")
	if got := s.commandMatches(); len(got) == 0 || got[0].command != "undo" {
		t.Fatalf("recent commands must rank first on an empty query, got %+v", got[:min(3, len(got))])
	}
}

func TestBackspaceNeverDismissesAnIssue(t *testing.T) {
	s := workspaceSampleState()
	s.Navigate(PageBugs)
	before := len(s.FilteredIssues())
	s.HandleKey(KeyEvent{Key: "Backspace"})
	if len(s.PendingActions) != 0 || s.Confirmation != nil {
		t.Fatalf("Backspace queued a destructive action: %+v", s.PendingActions)
	}
	if s.Page == PageBugs && len(s.FilteredIssues()) != before {
		t.Fatal("Backspace changed the issue list")
	}
}

func TestMarkingAFileReviewedMovesToTheNextUnreviewedFile(t *testing.T) {
	s := twoFileReviewState()
	reviewed := *s.Change
	reviewed.FileReviewHash, reviewed.ReviewedFiles = reviewed.Hash, []string{"internal/api/server.go"}
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/review-file") {
			t.Errorf("unexpected request %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(reviewed)
	})
	if active, _, ok := s.activeDiffHunk(); !ok || active.Path != "internal/api/server.go" {
		t.Fatal("fixture should start on the first file")
	}
	s.HandleKey(KeyEvent{Key: "w"})
	s.FlushActions(context.Background(), cl)
	active, _, ok := s.activeDiffHunk()
	if !ok || active.Path != "internal/api/server_test.go" {
		t.Fatalf("review did not advance to the next unreviewed file: %+v", active)
	}
	if !strings.Contains(s.CmdOutput, "1/2 files") {
		t.Fatalf("status should report review progress, got %q", s.CmdOutput)
	}
}

func TestActionBarShowsHowManyHintsDidNotFit(t *testing.T) {
	s := twoFileReviewState()
	th := themeForPalette(s.Palette)
	narrow := ansi.Strip(s.renderActionBar(th, 70))
	if !strings.Contains(narrow, "more") {
		t.Fatalf("narrow action bar must count hidden hints: %q", narrow)
	}
	if wide := ansi.Strip(s.renderActionBar(th, 220)); strings.Contains(wide, "more") {
		t.Fatalf("wide action bar should fit every hint: %q", wide)
	}
}

func TestWebUIGitTabIsReachableFromTheTUI(t *testing.T) {
	for _, arg := range []string{"git", "worktrees", "merge"} {
		if got := normalizeWebUITab(arg); got != "git" {
			t.Fatalf(":web %s = %q, want git", arg, got)
		}
	}
	s := workspaceSampleState()
	s.Navigate(PageGit)
	if got := s.defaultWebUITab(); got != "git" {
		t.Fatalf("worktrees page should open the Web UI Git page, got %q", got)
	}
}
