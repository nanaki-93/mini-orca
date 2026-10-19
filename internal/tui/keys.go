package tui

import (
	"fmt"
	"strings"
)

// KeyEvent represents a decoded terminal keystroke.
type KeyEvent struct {
	Key   string
	Shift bool
	Ctrl  bool
}

var paletteOrder = []struct {
	ID    Palette
	Label string
}{
	{ID: PaletteTokyo, Label: "Tokyo"},
	{ID: PaletteCRT, Label: "CRT Green"},
	{ID: PaletteAmber, Label: "Amber"},
	{ID: PalettePaper, Label: "Paper"},
}

// CyclePalette advances the TUI color palette to the next theme.
func (s *State) CyclePalette() {
	cur := 0
	for idx, p := range paletteOrder {
		if p.ID == s.Palette {
			cur = idx
			break
		}
	}
	next := paletteOrder[(cur+1)%len(paletteOrder)]
	s.Palette = next.ID
	s.CmdOutput = fmt.Sprintf("Switched TUI palette to %s", next.Label)
}

// HandleKey processes a single keystroke and updates the TUI state deterministically.
func (s *State) HandleKey(ev KeyEvent) {
	if ev.Ctrl && strings.EqualFold(ev.Key, "c") {
		s.Quitting = true
		return
	}
	if s.CmdActive {
		s.handleCommandInputKey(ev)
		return
	}
	if s.Confirmation != nil {
		s.handleConfirmationKey(ev)
		return
	}
	if s.ShowKeybindsModal {
		s.handleKeybindsModalKey(ev)
		return
	}
	if s.handleGlobalNavigationKey(ev) {
		return
	}
	s.handlePageScopedKey(ev)
}

func (s *State) handleCommandInputKey(ev KeyEvent) {
	switch ev.Key {
	case "Escape":
		s.CmdActive = false
		s.CmdInput = ""
	case "Enter":
		raw := s.CmdInput
		s.CmdActive = false
		s.CmdInput = ""
		s.ExecuteCommand(raw)
	case "Backspace":
		runes := []rune(s.CmdInput)
		if len(runes) > 0 {
			s.CmdInput = string(runes[:len(runes)-1])
		}
	default:
		if len([]rune(ev.Key)) == 1 {
			s.CmdInput += ev.Key
		}
	}
}

func (s *State) handleConfirmationKey(ev KeyEvent) {
	switch ev.Key {
	case "y", "Y", "Enter":
		conf := s.Confirmation
		s.Confirmation = nil
		if conf.OnConfirm != nil {
			conf.OnConfirm(s)
		}
	case "n", "N", "Escape":
		s.Confirmation = nil
		s.CmdOutput = "Canceled confirmation."
	}
}

func (s *State) handleKeybindsModalKey(ev KeyEvent) {
	switch ev.Key {
	case "Escape", "?", "Enter", "q":
		s.ShowKeybindsModal = false
	}
}

func (s *State) handleGlobalNavigationKey(ev KeyEvent) bool {
	switch ev.Key {
	case "?":
		s.ShowKeybindsModal = !s.ShowKeybindsModal
		return true
	case ":", "/":
		s.CmdActive = true
		s.CmdInput = ""
		return true
	case "1", "2", "3", "4", "5":
		return s.switchHubByKey(ev.Key)
	case "[", "]":
		s.cycleHub(ev.Key == "]")
		return true
	case "Tab":
		s.cycleSubView(!ev.Shift)
		return true
	case "j", "ArrowDown":
		s.moveSelection(1)
		return true
	case "k", "ArrowUp":
		s.moveSelection(-1)
		return true
	case "Enter":
		s.ActivateCurrentRow()
		return true
	case "p":
		s.CyclePalette()
		return true
	}
	return s.handleBackOrHorizontalKey(ev)
}

func (s *State) handleBackOrHorizontalKey(ev KeyEvent) bool {
	if ev.Key == "Escape" && s.Error != "" {
		s.Error = ""
		s.CmdOutput = "Dismissed error alert."
		return true
	}
	if (ev.Key == "Escape" || ev.Key == "Backspace") && s.Page == PageAgents && s.AgentDetailOpen {
		s.AgentDetailOpen = false
		s.CmdOutput = "Returned to Agents in Progress Dashboard"
		return true
	}
	if ev.Key == "h" || ev.Key == "ArrowLeft" || ev.Key == "l" || ev.Key == "ArrowRight" {
		if ev.Shift && s.Page == PageBugs {
			s.cycleIssueCategory(ev.Key == "ArrowRight")
			return true
		}
		s.stepHorizontalSection(ev.Key == "l" || ev.Key == "ArrowRight")
		return true
	}
	return false
}

func (s *State) switchHubByKey(key string) bool {
	for _, tab := range HubTabs() {
		if tab.Key == key {
			s.Navigate(tab.Page)
			s.CmdOutput = fmt.Sprintf("Switched to [%s] %s", tab.Key, tab.Label)
			return true
		}
	}
	return false
}

func (s *State) cycleHub(forward bool) {
	tabs := HubTabs()
	active := s.ActiveHub()
	cur := 0
	for idx, t := range tabs {
		if t.Key == active.Key {
			cur = idx
			break
		}
	}
	delta := -1
	if forward {
		delta = 1
	}
	next := tabs[(cur+delta+len(tabs))%len(tabs)]
	s.Navigate(next.Page)
	s.CmdOutput = fmt.Sprintf("Switched to [%s] %s", next.Key, next.Label)
}

func (s *State) cycleSubView(forward bool) {
	hub := s.ActiveHub()
	if len(hub.SubPages) > 0 {
		cur := 0
		for idx, sp := range hub.SubPages {
			if sp.Page == s.Page {
				cur = idx
				break
			}
		}
		delta := -1
		if forward {
			delta = 1
		}
		next := hub.SubPages[(cur+delta+len(hub.SubPages))%len(hub.SubPages)]
		s.Navigate(next.Page)
		s.CmdOutput = fmt.Sprintf("Sub-view: %s", next.Label)
		return
	}
	if s.Page == PageInstructions {
		s.cycleRulesSection(forward)
	}
}

func (s *State) stepHorizontalSection(forward bool) {
	switch s.Page {
	case PageBugs:
		s.cycleIssueStatusView(forward)
	case PageDocs:
		s.cycleDocsSection(forward)
	case PageInstructions:
		s.cycleRulesSection(forward)
	default:
		if len(s.ActiveHub().SubPages) > 1 {
			s.cycleSubView(forward)
		}
	}
}

func (s *State) moveSelection(delta int) {
	maxIdx := max(0, s.ActiveListLength()-1)
	next := s.ClampedIdx() + delta
	if next < 0 {
		next = 0
	}
	if next > maxIdx {
		next = maxIdx
	}
	s.SelectedIdx = next
}

func (s *State) cycleIssueCategory(forward bool) {
	categories := []string{"all", "bugs", "security", "performance"}
	s.IssueCategory = stepStringSlice(categories, s.IssueCategory, forward)
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Issue type filter: %s", strings.ToUpper(s.IssueCategory))
}

func (s *State) cycleIssueStatusView(forward bool) {
	views := []string{"open", "in_progress", "done"}
	s.IssueView = stepStringSlice(views, s.IssueView, forward)
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Issue status view: %s", strings.ToUpper(s.IssueView))
}

func (s *State) cycleDocsSection(forward bool) {
	sections := []string{"context", "components", "scenarios", "decisions"}
	s.DocsSection = stepStringSlice(sections, s.DocsSection, forward)
	s.SelectedIdx = 0
}

func (s *State) cycleRulesSection(forward bool) {
	sections := []string{"trust", "models", "bootstrap", "lessons"}
	s.RulesSection = stepStringSlice(sections, s.RulesSection, forward)
	s.SelectedIdx = 0
}

func stepStringSlice(items []string, current string, forward bool) string {
	cur := 0
	for idx, item := range items {
		if item == current {
			cur = idx
			break
		}
	}
	delta := -1
	if forward {
		delta = 1
	}
	return items[(cur+delta+len(items))%len(items)]
}

func (s *State) handlePageScopedKey(ev KeyEvent) {
	if s.handleBugsPageKey(ev) || s.handleAgentsAndEditorKey(ev) || s.handleRulesAndOverviewKey(ev) {
		return
	}
	if ev.Key == "r" && isAnalysisTriggerPage(s.Page) {
		s.triggerOrResumeAnalysis()
	}
}

func isAnalysisTriggerPage(p Page) bool {
	return p == PageSummary || p == PageAnalysisRun || p == PageBugs || p == PageFeatures || p == PageDocs
}

func (s *State) handleBugsPageKey(ev KeyEvent) bool {
	if s.Page == PageBugs {
		switch ev.Key {
		case "t", "c", "f", ".", ">", "L":
			s.cycleIssueCategory(true)
			return true
		case "T", "C", "F", ",", "<", "H":
			s.cycleIssueCategory(false)
			return true
		case "s", "v":
			s.cycleIssueStatusView(true)
			return true
		case "S", "V":
			s.cycleIssueStatusView(false)
			return true
		case "x", " ":
			s.toggleSelectedIssue()
			return true
		case "d":
			s.dismissSelectedIssue()
			return true
		}
	}
	if (s.Page == PageBugs || s.Page == PageSummary) && ev.Key == "a" {
		s.dispatchSelectedIssues()
		return true
	}
	if s.Page == PageFeatures {
		return s.handleFeaturesKey(ev.Key)
	}
	if s.Page == PageAnalysisFiles && (ev.Key == "x" || ev.Key == " ") {
		s.toggleCurrentFileExclusion()
		return true
	}
	return false
}

func (s *State) handleFeaturesKey(key string) bool {
	features := s.activeFeatures()
	idx := s.ClampedIdx()
	if idx >= len(features) {
		return false
	}
	idea := features[idx]
	switch key {
	case "d":
		s.CmdOutput = fmt.Sprintf("Dismissed feature idea: %s", idea.Title)
		s.enqueueAction(ActionRequest{Kind: "dismiss-feature", Target: idea.ID})
		return true
	case "a":
		s.dispatchFeatureIdea(idea)
		return true
	}
	return false
}

func (s *State) handleAgentsAndEditorKey(ev KeyEvent) bool {
	switch s.Page {
	case PageAgents:
		return s.handleAgentsKey(ev.Key)
	case PageEditor:
		return s.handleEditorKey(ev.Key)
	case PageGit:
		if ev.Key == "m" {
			s.mergeSelectedWorktree()
			return true
		}
	}
	return false
}

func (s *State) handleAgentsKey(key string) bool {
	switch key {
	case "n":
		s.CmdOutput = "Use :agent <objective> to dispatch a new precision agent mission"
		return true
	case "c":
		s.applyAndCommitProposal("")
		return true
	case "u":
		s.requestGuardedUndo()
		return true
	case "m":
		s.mergeSelectedWorktree()
		return true
	case "r":
		s.triggerAgentRecheckOrRepair()
		return true
	}
	return false
}

func (s *State) handleEditorKey(key string) bool {
	switch key {
	case "e":
		s.ShowInlineWhy = !s.ShowInlineWhy
		stateWord := "hidden"
		if s.ShowInlineWhy {
			stateWord = "enabled"
		}
		s.CmdOutput = fmt.Sprintf("Inline diff explanations %s", stateWord)
		return true
	case "c":
		s.applyAndCommitProposal("")
		return true
	case "u":
		s.requestGuardedUndo()
		return true
	case "x":
		s.rejectCurrentDiffHunk()
		return true
	}
	return false
}

func (s *State) handleRulesAndOverviewKey(ev KeyEvent) bool {
	if s.Page == PageInstructions {
		switch ev.Key {
		case "b":
			s.RulesSection = "bootstrap"
			s.CmdOutput = "Bootstrapping missing repository workflow pillars..."
			s.enqueueAction(ActionRequest{Kind: "bootstrap"})
			return true
		case "t":
			s.toggleWorkspaceTrust()
			return true
		case "g":
			s.RulesSection = "lessons"
			s.graduateSelectedLesson()
			return true
		}
	}
	if (s.Page == PageSummary || s.Page == PageProject) && ev.Key == "t" {
		s.CmdOutput = "Running project tests and checks..."
		s.enqueueAction(ActionRequest{Kind: "scan"})
		return true
	}
	return false
}
