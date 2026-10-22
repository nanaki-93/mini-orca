package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandMenuSearchAndNavigationArePassive(t *testing.T) {
	for _, item := range commandCatalog() {
		if item.page == "" {
			continue
		}
		s := NewState()
		s.HandleKey(KeyEvent{Key: ":"})
		s.HandleKey(KeyEvent{Text: item.command})
		frame := s.Render(80, 24)
		if !strings.Contains(frame, "COMMANDS") || len(s.PendingActions) != 0 {
			t.Fatalf("searching %s dispatched an action or hid the menu", item.command)
		}
		s.HandleKey(KeyEvent{Key: "Enter"})
		if s.EffectivePage() != item.page || len(s.PendingActions) != 0 || s.Confirmation != nil {
			t.Fatalf("navigation %s was not passive: page=%s actions=%v", item.command, s.Page, s.PendingActions)
		}
	}
	s := NewState()
	s.openCommandInput("switch project")
	s.HandleKey(KeyEvent{Key: "Enter"})
	if !s.CmdActive || s.CmdInput != "project " || len(s.PendingActions) != 0 {
		t.Fatal("search result should open the project path editor")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.CmdActive || s.CmdInput != "" || len(s.PendingActions) != 0 {
		t.Fatal("canceling the menu must be passive")
	}
}

func TestCommandMenuCompletesAndSubmitsExplicitProjectPath(t *testing.T) {
	s := NewState()
	s.HandleKey(KeyEvent{Key: "o"})
	if s.CmdInput != "project " || !s.CmdActive {
		t.Fatal("dashboard shortcut must open a project prompt")
	}
	s.HandleKey(KeyEvent{Text: "/tmp/my project"})
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 1 || s.PendingActions[0].Kind != "open-project" || s.PendingActions[0].Target != "/tmp/my project" {
		t.Fatalf("project path was changed: %+v", s.PendingActions)
	}
	s.PendingActions = nil
	s.openCommandInput("analy")
	s.HandleKey(KeyEvent{Key: "Tab"})
	if s.CmdInput != "analyze" || len(s.PendingActions) != 0 {
		t.Fatal("completion must not start analysis")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	s.openCommandInput("apply")
	s.HandleKey(KeyEvent{Key: "Enter"})
	if len(s.PendingActions) != 0 {
		t.Fatal("command menu must preserve Apply guards")
	}
}

func TestCommandInputEditingPasteAndHistory(t *testing.T) {
	s := NewState()
	s.openCommandInput("project /tmp/猫")
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	s.HandleKey(KeyEvent{Text: "新"})
	if s.CmdInput != "project /tmp/新猫" {
		t.Fatalf("rune insertion failed: %s", s.CmdInput)
	}
	s.HandleKey(KeyEvent{Key: "Delete"})
	s.HandleKey(KeyEvent{Key: "Backspace"})
	if s.CmdInput != "project /tmp/" {
		t.Fatalf("rune deletion failed: %s", s.CmdInput)
	}
	s.HandleKey(KeyEvent{Key: "u", Ctrl: true})
	s.HandleKey(keyEventFromTea(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("agent improve errors\nthen checks"), Paste: true}))
	if s.CmdInput != "agent improve errors then checks" || len(s.PendingActions) != 0 {
		t.Fatal("pasted text must stay inert in a single line")
	}
	draft := s.CmdInput
	s.recordCommandHistory(":repo")
	s.recordCommandHistory(":bugs")
	s.HandleKey(keyEventFromTea(tea.KeyMsg{Type: tea.KeyCtrlP}))
	if s.CmdInput != "bugs" {
		t.Fatal("Ctrl+P did not recall the last command")
	}
	s.HandleKey(KeyEvent{Key: "p", Ctrl: true})
	if s.CmdInput != "repo" {
		t.Fatal("Ctrl+P did not recall the earlier command")
	}
	s.HandleKey(KeyEvent{Key: "n", Ctrl: true})
	s.HandleKey(KeyEvent{Key: "n", Ctrl: true})
	if s.CmdInput != draft || len(s.PendingActions) != 0 {
		t.Fatal("history navigation must restore the draft without execution")
	}
	for _, dims := range [][2]int{{52, 16}, {80, 24}, {120, 36}} {
		assertFrameSize(t, s.Render(dims[0], dims[1]), dims[0], dims[1])
	}
}
