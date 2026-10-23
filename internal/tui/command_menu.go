package tui

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/theme"
)

type commandItem struct {
	command, title, hint string
	argument             bool
	page                 Page
}

func commandCatalog() []commandItem {
	items := []commandItem{
		{command: "project", title: "Open or switch project", hint: "project /absolute/path · Restore local project data and index.", argument: true},
		{command: "agent", title: "New task", hint: "agent <request> · Prepare an editable plan before generation.", argument: true},
		{command: "analyze", title: "Run or resume analysis", hint: "Analyze the selected scope. Remote providers require consent."},
		{command: "test", title: "Run project checks", hint: "Execute configured checks in an isolated workspace. Requires project trust."},
		{command: "reindex", title: "Update project index", hint: "Read current files and invalidate outdated checks and workflows."},
		{command: "reconnect", title: "Reconnect to daemon", hint: "Reconnect and reload local workspace information."},
	}
	for _, hub := range HubTabs() {
		for _, view := range hub.SubPages {
			if view.Page == PageIssueDetail || view.Page == PagePlan {
				continue
			}
			command := string(view.Page)
			if view.Page == PageProject {
				command = "checks"
			}
			if view.Page == PageBootstrap {
				command = "setup"
			}
			items = append(items, commandItem{command: command, title: hub.Label + " / " + view.Label, hint: "Open this view. No work starts until you choose an action.", page: view.Page})
		}
	}
	return append(items,
		commandItem{command: "plan", title: "Review task plan", hint: "Inspect the approach, evidence, scope and acceptance criteria."},
		commandItem{command: "pause", title: "Pause analysis", hint: "Stop dispatching new analysis work and retain completed results."},
		commandItem{command: "reanalyze", title: "Start fresh analysis", hint: "Run a new analysis, preserving work already in progress."},
		commandItem{command: "retry", title: "Retry a task stage", hint: "retry test or retry review · Reuse implementation or passing checks.", argument: true},
		commandItem{command: "open", title: "Read a source file", hint: "open <project-relative path> · Read-only source viewer.", argument: true},
		commandItem{command: "trust", title: "Manage execution trust", hint: "Grant or revoke permission to execute this project's checks."},
		commandItem{command: "apply", title: "Apply reviewed proposal", hint: "Write the checked proposal after every changed file has been reviewed."},
		commandItem{command: "undo", title: "Undo last applied change", hint: "Restore the previous source only when the current hashes still match."},
		commandItem{command: "merge", title: "Merge selected worktree", hint: "Integrate the reviewed task into its destination branch."},
		commandItem{command: "reject", title: "Reject selected task or worktree", hint: "Discard the fix and return its linked issues to TODO."},
		commandItem{command: "model", title: "Assign a model", hint: "model <analyze|bug|function> <model id>", argument: true},
		commandItem{command: "palette", title: "Change color theme", hint: "Cycle Tokyo, CRT, Amber and Paper."},
		commandItem{command: "keys", title: "Keyboard reference", hint: "Browse shortcuts for the current view and all colon commands."},
		commandItem{command: "quit", title: "Quit Mini-Orca", hint: "Close the terminal interface."},
	)
}

func (s *State) commandMatches() []commandItem {
	if s.commandArgumentContext() != nil {
		return nil
	}
	query := strings.ToLower(strings.TrimSpace(s.CmdInput))
	var items []commandItem
	for _, item := range commandCatalog() {
		text := strings.ToLower(item.command + " " + item.title)
		matches := true
		for _, word := range strings.Fields(query) {
			if !strings.Contains(text, word) {
				matches = false
				break
			}
		}
		if matches {
			if item.command == query {
				items = append([]commandItem{item}, items...)
			} else {
				items = append(items, item)
			}
		}
	}
	return items
}

func (s *State) commandArgumentContext() *commandItem {
	command, _, hasSpace := strings.Cut(strings.TrimLeft(s.CmdInput, " "), " ")
	if !hasSpace {
		return nil
	}
	for _, item := range commandCatalog() {
		if item.argument && strings.EqualFold(command, item.command) {
			return &item
		}
	}
	return nil
}

func (s *State) openCommandInput(value string) {
	s.CmdActive, s.CmdInput = true, value
	s.commandCursor = len([]rune(value))
	s.commandSelection, s.commandHistoryIndex = -1, -1
	s.commandHistoryDraft = ""
}

func (s *State) handleCommandInputKey(ev KeyEvent) {
	if ev.Ctrl {
		s.handleCommandControlKey(ev.Key)
		return
	}
	switch ev.Key {
	case "Escape":
		s.CmdActive, s.CmdInput = false, ""
	case "Enter":
		s.submitCommandInput()
	case "Tab":
		s.completeCommand(false)
	case "ArrowUp", "ArrowDown":
		delta := 1
		if ev.Key == "ArrowUp" {
			delta = -1
		}
		s.commandSelection = min(max(0, s.commandSelection+delta), max(0, len(s.commandMatches())-1))
	case "ArrowLeft", "ArrowRight", "Home", "End", "Backspace", "Delete":
		s.editCommandInput(ev.Key)
	default:
		text := ev.Text
		if text == "" && len([]rune(ev.Key)) == 1 {
			text = ev.Key
		}
		s.insertCommandText(text)
	}
}

func (s *State) handleCommandControlKey(key string) {
	switch strings.ToLower(key) {
	case "a":
		s.commandCursor = 0
	case "e":
		s.commandCursor = len([]rune(s.CmdInput))
	case "u":
		s.CmdInput, s.commandCursor = "", 0
		s.commandSelection = -1
	case "p":
		s.recallCommand(1)
	case "n":
		s.recallCommand(-1)
	}
}

func (s *State) editCommandInput(key string) {
	runes := []rune(s.CmdInput)
	pos := min(max(0, s.commandCursor), len(runes))
	switch key {
	case "Home":
		pos = 0
	case "End":
		pos = len(runes)
	case "ArrowLeft":
		pos = max(0, pos-1)
	case "ArrowRight":
		pos = min(len(runes), pos+1)
	case "Backspace":
		if pos > 0 {
			runes = append(runes[:pos-1], runes[pos:]...)
			pos--
		}
	case "Delete":
		if pos < len(runes) {
			runes = append(runes[:pos], runes[pos+1:]...)
		}
	}
	s.CmdInput, s.commandCursor = string(runes), pos
	s.commandSelection = -1
}

func (s *State) insertCommandText(text string) {
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, text)
	if text == "" {
		return
	}
	runes := []rune(s.CmdInput)
	pos := min(max(0, s.commandCursor), len(runes))
	s.CmdInput = string(runes[:pos]) + text + string(runes[pos:])
	s.commandCursor = pos + len([]rune(text))
	s.commandSelection = -1
}

func (s *State) recallCommand(delta int) {
	if s.commandHistoryIndex < 0 {
		s.commandHistoryDraft = s.CmdInput
	}
	s.commandHistoryIndex = min(max(-1, s.commandHistoryIndex+delta), len(s.CmdHistory)-1)
	s.CmdInput = s.commandHistoryDraft
	if s.commandHistoryIndex >= 0 {
		s.CmdInput = strings.TrimPrefix(s.CmdHistory[s.commandHistoryIndex], ":")
	}
	s.commandCursor = len([]rune(s.CmdInput))
	s.commandSelection = -1
}

func (s *State) submitCommandInput() {
	if s.commandSelection >= 0 || strings.TrimSpace(s.CmdInput) == "" {
		s.completeCommand(true)
		return
	}
	matches := s.commandMatches()
	if len(matches) > 0 && !strings.EqualFold(strings.TrimSpace(s.CmdInput), matches[0].command) {
		s.completeCommand(true)
		return
	}
	// A bare command needing arguments opens an editor instead of using an alias.
	for _, item := range commandCatalog() {
		if item.argument && strings.TrimSpace(s.CmdInput) == item.command {
			s.openCommandInput(item.command + " ")
			return
		}
	}
	raw := s.CmdInput
	s.CmdActive, s.CmdInput = false, ""
	s.ExecuteCommand(raw)
}

func (s *State) completeCommand(execute bool) {
	items := s.commandMatches()
	if len(items) == 0 {
		return
	}
	item := items[min(max(0, s.commandSelection), len(items)-1)]
	if item.argument || !execute {
		value := item.command
		if item.argument {
			value += " "
		}
		s.openCommandInput(value)
		return
	}
	s.CmdActive, s.CmdInput = false, ""
	s.ExecuteCommand(item.command)
}

func (s *State) renderCommandMenu(th ansiTheme, width, rows int) []string {
	lines := renderPageHeader(th, "COMMANDS", "")
	input := th.renderCommandInput(s.CmdInput, s.commandCursor, width-4)
	lines[1] = "  " + theme.PaintLine(th.surface, th.fg, input, width-4)
	if rows >= 12 {
		lines = append(lines, "")
	}
	if item := s.commandArgumentContext(); item != nil {
		lines = append(lines, "  "+th.cyan+item.title+th.reset)
		return appendWrappedText(lines, item.hint, width)
	}
	items := s.commandMatches()
	if len(items) == 0 {
		return append(lines, "  No matching suggestions.", "  Enter runs the command you typed.")
	}
	selected := min(max(0, s.commandSelection), len(items)-1)
	hint := theme.WrapVisibleLine(items[selected].hint, width-4)
	start, end := windowBounds(len(items), selected, max(1, rows-len(lines)-len(hint)-1))
	commandWidth := min(20, width/3)
	for i := start; i < end; i++ {
		item := items[i]
		line := th.renderCursorPrefix(i == selected) + theme.PadVisibleRight(th.cyan+":"+item.command+th.reset, commandWidth) + " " + rowTitleStyle(th, i == selected) + item.title + th.reset
		lines = append(lines, "  "+theme.TruncateVisible(line, width-4))
	}
	lines = append(lines, "  "+th.muted+fmt.Sprintf("%d–%d of %d commands", start+1, end, len(items))+th.reset)
	for _, line := range hint {
		lines = append(lines, "  "+th.cyan+line+th.reset)
	}
	return lines
}

func (s *State) execViewCommand(cmd string) bool {
	for _, item := range commandCatalog() {
		if item.page != "" && cmd == item.command {
			s.Navigate(item.page)
			s.CmdOutput = item.title
			return true
		}
	}
	return false
}
