package tui

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// runDispatch drives a queued batch of blocking actions to completion the way
// the Bubble Tea runtime would: start message, background command, done message.
func runDispatch(t *testing.T, m *BubbleModel, start tea.Msg) {
	t.Helper()
	_, cmd := m.Update(start)
	if cmd == nil {
		t.Fatal("starting a blocking batch must return its background command")
	}
	done, ok := cmd().(dispatchDoneMsg)
	if !ok {
		t.Fatalf("background command returned %T, want dispatchDoneMsg", done)
	}
	m.Update(done)
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
	if s.Busy != "Reindexing project…" || !strings.Contains(ansi.Strip(m.View()), "Reindexing project…") {
		t.Fatalf("busy status not rendered: busy=%q", s.Busy)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected one deferred command, got %d", len(cmds))
	}
	msg, ok := cmds[0]().(syncActionsMsg)
	if !ok {
		t.Fatalf("unexpected deferred message: %#v", msg)
	}
	runDispatch(t, m, msg)
	if reindexes.Load() != 1 || s.Busy != "" || m.dispatch != nil {
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
	runDispatch(t, m, cmds[len(cmds)-1]())
	if len(order) < 2 || order[0] != "/api/projects/current/reindex" {
		t.Fatalf("actions ran out of order: %v", order)
	}
	if s.Busy != "" || s.Error == "" {
		t.Fatalf("failure must surface as an error and release busy: busy=%q error=%q", s.Busy, s.Error)
	}
}

// startBlockedDispatch queues one blocking action against a handler that holds
// the request open, and returns once the request reached the daemon.
func startBlockedDispatch(t *testing.T, act ActionRequest, handler func(http.ResponseWriter, *http.Request)) (*State, *BubbleModel, chan tea.Msg) {
	t.Helper()
	entered := make(chan struct{}, 1)
	s := workspaceSampleState()
	cl := investigationClient(t, func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		handler(w, r)
	})
	m := NewBubbleModel(context.Background(), s, cl, 120, 30)
	m.nativeStreams = false
	s.enqueueAction(act)
	cmds := m.flushActionsAsync()
	_, cmd := m.Update(cmds[len(cmds)-1]())
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("blocking action never reached the daemon")
	}
	return s, m, done
}

func awaitDispatch(t *testing.T, done chan tea.Msg) tea.Msg {
	t.Helper()
	select {
	case msg := <-done:
		return msg
	case <-time.After(5 * time.Second):
		t.Fatal("blocking action did not finish")
		return nil
	}
}

// While a request is in flight the frame keeps rendering and keys are queued,
// then replayed in order once the action completes.
func TestBlockingActionKeepsRenderingAndReplaysQueuedKeys(t *testing.T) {
	release := make(chan struct{})
	s, m, done := startBlockedDispatch(t, ActionRequest{Kind: "reindex"}, func(w http.ResponseWriter, r *http.Request) {
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"project_id": "fixture", "project_revision": "rev-1"})
	})
	startPage := PageSummary
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 28})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	if cmd := m.updateDuringDispatch(spinnerTickMsg{}); cmd == nil {
		t.Fatal("the spinner must keep ticking while an action runs")
	}
	frame := ansi.Strip(m.View())
	if !strings.Contains(frame, "Reindexing project…") || !strings.Contains(frame, "1 key queued") {
		t.Fatalf("in-flight frame must show the busy label and queued input:\n%s", frame)
	}
	assertFrameSize(t, m.View(), 100, 28)
	close(release)
	m.Update(awaitDispatch(t, done))
	if s.Busy != "" || m.dispatch != nil {
		t.Fatalf("busy state not released: %q", s.Busy)
	}
	if s.Page == startPage || s.ActiveHub().Key != "2" {
		t.Fatalf("queued key was not replayed after the action: page=%v", s.Page)
	}
}

func TestEscCancelsReadOnlyActionsButNotMutations(t *testing.T) {
	s, m, done := startBlockedDispatch(t, ActionRequest{Kind: "open-file", Target: "internal/api/server.go"}, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	if !strings.Contains(ansi.Strip(m.View()), "Esc cancel") {
		t.Fatal("a read-only action must advertise Esc to cancel")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m.Update(awaitDispatch(t, done))
	if s.Error != "" || !strings.HasPrefix(s.CmdOutput, "Canceled: Opening file") {
		t.Fatalf("cancel must be reported as a cancel, not an error: error=%q output=%q", s.Error, s.CmdOutput)
	}

	release := make(chan struct{})
	_, m, done = startBlockedDispatch(t, ActionRequest{Kind: "reindex"}, func(w http.ResponseWriter, r *http.Request) {
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"project_id": "fixture", "project_revision": "rev-1"})
	})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if frame := ansi.Strip(m.View()); !strings.Contains(frame, "can't cancel") {
		t.Fatalf("a mutation must explain it cannot be canceled:\n%s", frame)
	}
	close(release)
	if msg := awaitDispatch(t, done).(dispatchDoneMsg); msg.canceled {
		t.Fatal("Esc must not cancel a mutation")
	}
}

func TestCtrlCDuringAMutationQuitsAfterItFinishes(t *testing.T) {
	release := make(chan struct{})
	s, m, done := startBlockedDispatch(t, ActionRequest{Kind: "reindex"}, func(w http.ResponseWriter, r *http.Request) {
		<-release
		_ = json.NewEncoder(w).Encode(map[string]any{"project_id": "fixture", "project_revision": "rev-1"})
	})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	close(release)
	_, cmd := m.Update(awaitDispatch(t, done))
	if !s.Quitting || cmd == nil {
		t.Fatal("Ctrl+C must quit once the running mutation completes")
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
