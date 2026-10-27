package tui

import (
	"fmt"
	"strings"
	"time"
)

// Navigation history, remembered selection and the message log make moving
// around the TUI reversible: Backspace or Ctrl+O returns to the previous view
// with its selection, each view keeps the row you left it on, and status
// messages that scroll past remain readable in :messages.

const (
	maxNavHistory = 32
	maxMessageLog = 200
)

type navEntry struct {
	page     Page
	selected int
}

type messageEntry struct {
	at      time.Time
	text    string
	isError bool
}

// navigation holds per-session movement state that the views do not render
// directly.
type navigation struct {
	history    []navEntry
	selection  map[Page]int
	goingBack  bool
	log        []messageEntry
	lastOutput string
	lastError  string
	showLog    bool
	logScroll  int
}

// rememberLeaving records the page being left so it can be restored later.
func (s *State) rememberLeaving(next Page) {
	if s.nav.selection == nil {
		s.nav.selection = map[Page]int{}
	}
	s.nav.selection[s.Page] = s.SelectedIdx
	if s.nav.goingBack || s.Page == "" || s.Page == next {
		return
	}
	entry := navEntry{page: s.Page, selected: s.SelectedIdx}
	if n := len(s.nav.history); n > 0 && s.nav.history[n-1].page == entry.page {
		s.nav.history[n-1] = entry
		return
	}
	s.nav.history = append(s.nav.history, entry)
	if len(s.nav.history) > maxNavHistory {
		s.nav.history = s.nav.history[len(s.nav.history)-maxNavHistory:]
	}
}

// rememberedSelection returns the row last selected on page, or 0.
func (s *State) rememberedSelection(page Page) int {
	return s.nav.selection[page]
}

// NavigateBack returns to the previous view and its selection.
func (s *State) NavigateBack() bool {
	for len(s.nav.history) > 0 {
		last := s.nav.history[len(s.nav.history)-1]
		s.nav.history = s.nav.history[:len(s.nav.history)-1]
		if last.page == s.Page {
			continue
		}
		s.nav.goingBack = true
		s.Navigate(last.page)
		s.nav.goingBack = false
		s.SelectedIdx = last.selected
		s.CmdOutput = "Back to " + s.pageTitle(last.page)
		return true
	}
	s.CmdOutput = "No earlier view to return to."
	return false
}

// pageTitle names a page as the breadcrumb does.
func (s *State) pageTitle(page Page) string {
	for _, hub := range HubTabs() {
		for _, sub := range hub.SubPages {
			if sub.Page == page {
				return hub.Label + " › " + sub.Label
			}
		}
		if hub.Page == page {
			return hub.Label
		}
	}
	return string(page)
}

// recordFeedback appends new status and error messages to the message log.
func (s *State) recordFeedback() {
	if out := strings.TrimSpace(s.CmdOutput); out != "" && out != s.nav.lastOutput {
		s.nav.lastOutput = out
		if out != "Workspace ready" {
			s.appendMessage(out, false)
		}
	}
	if s.Error != s.nav.lastError {
		s.nav.lastError = s.Error
		if s.Error != "" {
			s.appendMessage(s.Error, true)
		}
	}
}

func (s *State) appendMessage(text string, isError bool) {
	s.nav.log = append(s.nav.log, messageEntry{at: time.Now(), text: text, isError: isError})
	if len(s.nav.log) > maxMessageLog {
		s.nav.log = s.nav.log[len(s.nav.log)-maxMessageLog:]
	}
}

func (s *State) openMessageLog() {
	s.recordFeedback()
	s.nav.showLog, s.nav.logScroll = true, 0
}

func (s *State) handleMessageLogKey(ev KeyEvent) {
	switch ev.Key {
	case "Escape", "q", "Enter":
		s.nav.showLog = false
	case "j", "ArrowDown":
		s.nav.logScroll++
	case "k", "ArrowUp":
		s.nav.logScroll = max(0, s.nav.logScroll-1)
	case "PageDown":
		s.nav.logScroll += max(1, s.viewportBodyRows-2)
	case "PageUp":
		s.nav.logScroll = max(0, s.nav.logScroll-max(1, s.viewportBodyRows-2))
	case "Home":
		s.nav.logScroll = 0
	case "End":
		s.nav.logScroll = 1 << 30
	}
}

// wrapPlain wraps text on spaces to at most width runes per line.
func wrapPlain(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	line := words[0]
	for _, word := range words[1:] {
		if len([]rune(line))+1+len([]rune(word)) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		line += " " + word
	}
	return append(lines, line)
}

// requestTrustChange asks before granting execution trust, because trust lets
// checks run project code. Revoking is always safe and happens immediately.
func (s *State) requestTrustChange() {
	if s.WorkspaceTrust {
		s.toggleWorkspaceTrust()
		return
	}
	s.Confirmation = &Confirmation{
		Title:       "Trust this project?",
		Message:     "Checks will run this project's configured commands in copied workspaces. Trust is remembered for this project path until you revoke it.",
		AcceptLabel: "Trust project",
		OnConfirm:   func(s *State) { s.toggleWorkspaceTrust() },
	}
}

// confirmApplyFromDiff guards Enter in the diff view: Apply writes source and
// commits, so a stray Enter while reading a diff must not trigger it.
func (s *State) confirmApplyFromDiff() {
	if !s.applyAllowed() {
		return
	}
	message := fmt.Sprintf("Write and commit %q in the current checkout. Undo stays available for the latest Apply while the source is unchanged.", s.Change.Title)
	if note := s.stoppedTasksNote(s.Change); note != "" {
		message += " " + note
	}
	s.Confirmation = &Confirmation{
		Title:       "Apply reviewed changes?",
		Message:     message,
		AcceptLabel: "Apply & commit",
		OnConfirm: func(state *State) {
			if state.applyAllowed() {
				state.queueApply("")
			}
		},
	}
}
