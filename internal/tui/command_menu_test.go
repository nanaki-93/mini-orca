package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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

func TestCommandShortcutWorksAcrossViewsWithoutDispatch(t *testing.T) {
	for _, hub := range HubTabs() {
		for _, page := range hub.SubPages {
			s := workspaceSampleState()
			s.Navigate(page.Page)
			s.HandleKey(keyEventFromTea(tea.KeyMsg{Type: tea.KeyCtrlK}))
			if !s.CmdActive || s.Page != page.Page || len(s.PendingActions) != 0 || s.Confirmation != nil {
				t.Fatalf("Ctrl+K did not passively open commands on %s", page.Page)
			}
			s.HandleKey(KeyEvent{Key: "Escape"})
			if s.CmdActive || s.Page != page.Page || len(s.PendingActions) != 0 {
				t.Fatal("canceling command search must restore its original view")
			}
		}
	}
	events := DecodeInputBytes([]byte{0x0b})
	if len(events) != 1 || events[0].Key != "k" || !events[0].Ctrl {
		t.Fatal("stream and interactive input must agree on Ctrl+K")
	}
}

func TestCommandShortcutRespectsOverlaysAndExistingDraft(t *testing.T) {
	s := NewState()
	confirmed := false
	s.Confirmation = &Confirmation{Title: "Approve scope", OnConfirm: func(*State) { confirmed = true }}
	s.HandleKey(KeyEvent{Key: "k", Ctrl: true})
	if s.CmdActive || s.Confirmation == nil || confirmed {
		t.Fatal("command shortcut must not bypass a confirmation")
	}
	s.Confirmation = nil
	s.Error, s.ShowKeybindsModal = "Connection unavailable", true
	s.HandleKey(KeyEvent{Key: "k", Ctrl: true})
	if !s.CmdActive || s.ShowKeybindsModal || s.Error == "" {
		t.Fatal("command access should close help and preserve error recovery")
	}
	s.HandleKey(KeyEvent{Text: "project /tmp/my project"})
	s.HandleKey(KeyEvent{Key: "ArrowLeft"})
	draft, cursor := s.CmdInput, s.commandCursor
	s.HandleKey(KeyEvent{Key: "k", Ctrl: true})
	if s.CmdInput != draft || s.commandCursor != cursor || len(s.PendingActions) != 0 {
		t.Fatal("Ctrl+K inside the editor must preserve the draft and cursor")
	}
	s.HandleKey(KeyEvent{Key: "Escape"})
	if s.Error == "" || s.CmdActive {
		t.Fatal("canceling search must return to the error overlay")
	}
}

func TestCommandInputAndSelectedHintStayTogetherAtEverySize(t *testing.T) {
	for _, dims := range [][2]int{{40, 10}, {52, 16}, {80, 24}, {100, 20}, {120, 34}} {
		s := NewState()
		s.openCommandInput("agent draft-cursor")
		frame := ansi.Strip(s.Render(dims[0], dims[1]))
		assertFrameSize(t, frame, dims[0], dims[1])
		lines := strings.Split(frame, "\n")
		inputRow := -1
		for i, line := range lines {
			if strings.Contains(line, "draft-cursor") {
				if inputRow >= 0 {
					t.Fatal("command input must be rendered only once")
				}
				inputRow = i
			}
		}
		if inputRow < 0 || inputRow >= len(lines)-3 {
			t.Fatalf("input should be beside its results, not at the bottom, at %v", dims)
		}
		if dims[0] < 52 {
			continue
		}
		s.openCommandInput("test")
		frame = ansi.Strip(s.Render(dims[0], dims[1]))
		var content []string
		for _, line := range strings.Split(frame, "\n") {
			if _, body, sidebar := strings.Cut(line, "│"); sidebar {
				line = body
			}
			content = append(content, strings.Fields(line)...)
		}
		if !strings.Contains(strings.Join(content, " "), "Requires project trust.") || !strings.Contains(frame, "[Esc] Cancel") || len(s.PendingActions) != 0 {
			t.Fatalf("selected command lost its scope hint or cancel at %v:\n%s", dims, frame)
		}
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

func TestCommandArgumentsAreNeverTreatedAsSearchTerms(t *testing.T) {
	for _, argument := range []string{"open", "project", "switch project"} {
		s := NewState()
		s.openCommandInput("project " + argument)
		if len(s.commandMatches()) != 0 {
			t.Fatal("argument input must not select a different command")
		}
		s.HandleKey(KeyEvent{Key: "Enter"})
		if len(s.PendingActions) != 1 || s.PendingActions[0].Target != argument {
			t.Fatalf("argument %q was consumed as a search: %+v", argument, s.PendingActions)
		}
	}
	s := NewState()
	s.openCommandInput("agent task")
	s.HandleKey(KeyEvent{Key: "Enter"})
	if s.PlanDraft == nil || s.PlanDraft.Seed.Title != "task" || len(s.PendingActions) != 0 {
		t.Fatal("task text must open its plan unchanged without starting generation")
	}
}

func TestCommandInputEditingPasteAndHistory(t *testing.T) {
	s := NewState()
	s.HandleKey(keyEventFromTea(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r"), Paste: true}))
	if len(s.PendingActions) != 0 {
		t.Fatal("pasting outside the command editor must not trigger shortcuts")
	}
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
