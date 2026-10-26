package tui

import (
	"context"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

func TestBackReturnsToThePreviousViewAndSelection(t *testing.T) {
	s := workspaceSampleState()
	s.Navigate(PageBugs)
	s.SelectedIdx = 2
	s.HandleKey(KeyEvent{Key: "3"})
	if s.ActiveHub().Key != "3" {
		t.Fatalf("hub switch failed: %s", s.Page)
	}
	s.HandleKey(KeyEvent{Key: "o", Ctrl: true})
	if s.Page != PageBugs || s.SelectedIdx != 2 {
		t.Fatalf("Ctrl+O must restore the issues view and its row: page=%s idx=%d", s.Page, s.SelectedIdx)
	}
	if !strings.Contains(s.CmdOutput, "Back to") {
		t.Fatalf("going back should say where it went, got %q", s.CmdOutput)
	}
	s.HandleKey(KeyEvent{Key: "Backspace"})
	if s.Page != PageSummary {
		t.Fatalf("Backspace must keep walking back through history, got %s", s.Page)
	}
	s.HandleKey(KeyEvent{Key: "Backspace"})
	if s.Page != PageSummary {
		t.Fatalf("with no history Backspace must stay put, got %s", s.Page)
	}
}

func TestEachViewRemembersItsSelectedRow(t *testing.T) {
	s := workspaceSampleState()
	s.Navigate(PageBugs)
	s.SelectedIdx = 1
	s.Navigate(PageModels)
	if s.SelectedIdx != 0 {
		t.Fatalf("a view never visited starts at the top, got %d", s.SelectedIdx)
	}
	s.SelectedIdx = 2
	s.Navigate(PageBugs)
	if s.SelectedIdx != 1 {
		t.Fatalf("returning to issues must restore its row, got %d", s.SelectedIdx)
	}
	s.Navigate(PageModels)
	if s.SelectedIdx != 2 {
		t.Fatalf("returning to models must restore its row, got %d", s.SelectedIdx)
	}
}

func TestCtrlOFromTheTerminalMapsToBack(t *testing.T) {
	if ev := keyEventFromTea(tea.KeyMsg{Type: tea.KeyCtrlO}); !ev.Ctrl || ev.Key != "o" {
		t.Fatalf("Ctrl+O decoded as %+v", ev)
	}
}

func TestMessageLogKeepsStatusAndErrorsNewestFirst(t *testing.T) {
	s := workspaceSampleState()
	m := NewBubbleModel(context.Background(), s, nil, 120, 30)
	s.CmdOutput = "Reindexed 128 files"
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	s.Error = "daemon request failed: connection refused"
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	s.Error = ""
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	if len(s.nav.log) != 2 {
		t.Fatalf("log entries = %+v", s.nav.log)
	}
	s.ExecuteCommand(":messages")
	frame := ansi.Strip(s.Render(120, 30))
	first, second := strings.Index(frame, "connection refused"), strings.Index(frame, "Reindexed 128 files")
	if !strings.Contains(frame, "MESSAGES") || first < 0 || second < 0 || first > second {
		t.Fatalf("message log must list newest first:\n%s", frame)
	}
	assertFrameSize(t, s.Render(120, 30), 120, 30)
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.nav.showLog {
		t.Fatal("Esc must close the message log")
	}
}

func TestGrantingTrustAsksButRevokingDoesNot(t *testing.T) {
	s := workspaceSampleState()
	s.WorkspaceTrust = false
	s.Navigate(PageInstructions)
	s.HandleKey(KeyEvent{Key: "t"})
	if s.Confirmation == nil || len(s.PendingActions) != 0 {
		t.Fatal("granting trust must ask for confirmation first")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.WorkspaceTrust || len(s.PendingActions) != 0 {
		t.Fatal("canceling must leave trust unchanged")
	}
	s.WorkspaceTrust = true
	s.HandleKey(KeyEvent{Key: "ArrowRight"})
	if s.Confirmation != nil || len(s.PendingActions) != 0 {
		t.Fatal("ArrowRight on an already trusted project must do nothing")
	}
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	if s.Confirmation != nil || s.WorkspaceTrust || len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "set-trust" {
		t.Fatalf("revoking trust must happen immediately: %+v", s.PendingActions)
	}
}

func TestNoColorRendersWithoutColorEscapes(t *testing.T) {
	previous := theme.NoColor
	theme.NoColor = true
	t.Cleanup(func() { theme.NoColor = previous })
	for _, s := range []*State{workspaceSampleState(), twoFileReviewState()} {
		frame := s.Render(120, 30)
		if strings.Contains(frame, "38;5;") || strings.Contains(frame, "48;5;") || strings.Contains(frame, "38;2;") {
			t.Fatalf("NO_COLOR frame still carries color escapes:\n%q", frame[:min(400, len(frame))])
		}
		if !strings.Contains(frame, "\033[7m") {
			t.Fatal("the selection must stay visible with reverse video")
		}
		assertFrameSize(t, frame, 120, 30)
	}
}
