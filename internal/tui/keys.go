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
	if s.handleEscapeOrBackKey(ev.Key) {
		return true
	}
	return s.handleHorizontalStepKey(ev)
}

func (s *State) handleEscapeOrBackKey(key string) bool {
	if key == "Escape" && s.Error != "" {
		s.Error = ""
		s.CmdOutput = "Dismissed error alert."
		return true
	}
	if key != "Escape" && key != "Backspace" {
		return false
	}
	if s.Page == PageIssueDetail {
		s.Page = PageBugs
		s.CmdOutput = "Returned to Issues list"
		return true
	}
	if s.Page == PageAgents && s.AgentDetailOpen {
		s.AgentDetailOpen = false
		s.CmdOutput = "Returned to Agents in Progress Dashboard"
		return true
	}
	return false
}

func (s *State) handleHorizontalStepKey(ev KeyEvent) bool {
	if ev.Key != "h" && ev.Key != "ArrowLeft" && ev.Key != "l" && ev.Key != "ArrowRight" {
		return false
	}
	if ev.Shift && s.Page == PageBugs {
		s.cycleIssueCategory(ev.Key == "ArrowRight")
		return true
	}
	s.stepHorizontalSection(ev.Key == "l" || ev.Key == "ArrowRight")
	return true
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
	var subPages []HubSubPage
	for _, sp := range hub.SubPages {
		if sp.Page != PageIssueDetail {
			subPages = append(subPages, sp)
		}
	}
	if len(subPages) == 0 {
		return
	}
	curPage := s.EffectivePage()
	if curPage == PageIssueDetail {
		if !forward {
			s.Navigate(PageBugs)
			s.CmdOutput = "Sub-view: Issues"
			return
		}
		curPage = PageBugs
	}
	cur := 0
	for idx, sp := range subPages {
		if sp.Page == curPage {
			cur = idx
			break
		}
	}
	delta := -1
	if forward {
		delta = 1
	}
	next := subPages[(cur+delta+len(subPages))%len(subPages)]
	s.Navigate(next.Page)
	s.CmdOutput = fmt.Sprintf("Sub-view: %s", next.Label)
}

func (s *State) stepHorizontalSection(forward bool) {
	if len(s.ActiveHub().SubPages) > 1 {
		s.cycleSubView(forward)
	}
}

func (s *State) moveSelection(delta int) {
	switch s.EffectivePage() {
	case PageDocs, PageDocsComponents, PageDocsDecisions, PageDiagrams:
		s.cycleSubView(delta > 0)
		return
	case PageEditor:
		if s.Change != nil && len(s.Change.Changes) > 0 {
			s.moveEditorDiffSelection(delta)
			return
		}
	}
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
	if s.Page == PageBugs && s.handleIssueListKey(ev.Key) {
		return true
	}
	return s.handleIssueActionKey(ev.Key)
}

func (s *State) handleIssueListKey(key string) bool {
	switch key {
	case "i":
		s.Navigate(PageIssueDetail)
		s.CmdOutput = "Opened Issue Detail view ([Enter/a] Fix with Agent · [Esc] Back)"
		return true
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
	return false
}

func (s *State) handleIssueActionKey(key string) bool {
	switch s.Page {
	case PageBugs, PageIssueDetail, PageSummary:
		return s.handleIssueShortcut(key)
	case PageFeatures:
		return s.handleFeaturesKey(key)
	case PageAnalysisFiles:
		if key == "x" || key == " " {
			s.toggleCurrentFileExclusion()
			return true
		}
	}
	return false
}

func (s *State) handleIssueShortcut(key string) bool {
	switch key {
	case "a":
		s.dispatchSelectedIssues()
		return true
	case "w":
		s.Navigate(PageAgents)
		s.CmdOutput = "Switched to [2] Agent Studio · Pipeline to watch active agent."
		return true
	case "o":
		if s.Page != PageSummary {
			if issue := s.SelectedIssue(); issue != nil && issue.Path != "" {
				s.openFileInEditor(issue.Path)
				return true
			}
		}
	case "d":
		if s.Page == PageIssueDetail {
			s.dismissSelectedIssue()
			return true
		}
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
	case "d", "i":
		s.openAgentDiffInspector()
		return true
	case "b":
		s.Navigate(PageBugs)
		s.CmdOutput = "Returned to [2] Agent Studio · Issues"
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
	case "x":
		s.cancelActiveWorkflow()
		return true
	}
	return false
}

func (s *State) handleEditorKey(key string) bool {
	if s.handleEditorDiffNavKey(key) {
		return true
	}
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
	case "w":
		s.Navigate(PageAgents)
		s.CmdOutput = "Returned to [2] Agent Studio · Pipeline"
		return true
	case "b":
		s.Navigate(PageBugs)
		s.CmdOutput = "Returned to [2] Agent Studio · Issues"
		return true
	}
	return false
}

func (s *State) handleEditorDiffNavKey(key string) bool {
	switch key {
	case "n", "N", "}", "{":
		s.stepDiffHunk(key == "n" || key == "}")
		return true
	case "f", "F":
		s.stepDiffFile(key == "f")
		return true
	case "v", "V":
		s.cycleDiffViewMode(key == "v")
		return true
	}
	return false
}

func (s *State) stepDiffHunk(forward bool) {
	active, allHunks, ok := s.activeDiffHunk()
	if !ok {
		return
	}
	delta := -1
	if forward {
		delta = 1
	}
	nextIdx := (active.GlobalIndex + delta + len(allHunks)) % len(allHunks)
	s.syncDiffSelectionToHunk(nextIdx)
	s.CmdOutput = s.formatFocusedHunkStatus(allHunks[nextIdx], len(allHunks))
}

func (s *State) stepDiffFile(forward bool) {
	active, allHunks, ok := s.activeDiffHunk()
	if !ok {
		return
	}
	if active.FileTotal <= 1 {
		s.CmdOutput = fmt.Sprintf("Single file in proposal: %s (use [n/N] to step hunks)", active.Path)
		return
	}
	delta := -1
	if forward {
		delta = 1
	}
	targetFile := (active.FileIndex + delta + active.FileTotal) % active.FileTotal
	for range active.FileTotal {
		for _, fh := range allHunks {
			if fh.FileIndex == targetFile {
				s.syncDiffSelectionToHunk(fh.GlobalIndex)
				s.CmdOutput = s.formatFocusedHunkStatus(fh, len(allHunks))
				return
			}
		}
		targetFile = (targetFile + delta + active.FileTotal) % active.FileTotal
	}
}

func (s *State) cycleDiffViewMode(forward bool) {
	modes := []string{"hunk", "split", "unified"}
	next := stepStringSlice(modes, s.EffectiveDiffMode(), forward)
	s.setDiffViewMode(next)
}

func (s *State) setDiffViewMode(mode string) {
	s.DiffMode = mode
	eff := s.EffectiveDiffMode()
	if active, _, ok := s.activeDiffHunk(); ok {
		s.syncDiffSelectionToHunk(active.GlobalIndex)
	} else {
		s.SelectedIdx = 0
	}
	s.CmdOutput = fmt.Sprintf("Switched diff inspector view to %s ([v] to cycle · [n/N] hunk · [f/F] file)", diffModeDisplayName(eff))
}

func (s *State) moveEditorDiffSelection(delta int) {
	active, allHunks, ok := s.activeDiffHunk()
	if !ok {
		return
	}
	if s.EffectiveDiffMode() == "unified" {
		uRows := s.unifiedDiffRows()
		if len(uRows) == 0 {
			return
		}
		next := min(max(0, s.ClampedIdx()+delta), len(uRows)-1)
		s.SelectedIdx = next
		s.HunkIdx = uRows[next].HunkGlobalIdx
		s.CmdOutput = s.formatFocusedHunkStatus(allHunks[s.HunkIdx], len(allHunks))
		return
	}
	rowCount := s.diffViewRowCount()
	next := s.ClampedIdx() + delta
	if next >= rowCount && delta > 0 && active.GlobalIndex+1 < len(allHunks) {
		nextHunk := active.GlobalIndex + 1
		s.HunkIdx = nextHunk
		s.SelectedIdx = 0
		s.CmdOutput = s.formatFocusedHunkStatus(allHunks[nextHunk], len(allHunks))
		return
	}
	if next < 0 && delta < 0 && active.GlobalIndex > 0 {
		prevHunk := active.GlobalIndex - 1
		s.HunkIdx = prevHunk
		s.SelectedIdx = max(0, s.diffViewRowCount()-1)
		s.CmdOutput = s.formatFocusedHunkStatus(allHunks[prevHunk], len(allHunks))
		return
	}
	s.SelectedIdx = min(max(0, next), max(0, rowCount-1))
}

func isGovernancePage(p Page) bool {
	return p == PageInstructions || p == PageModels || p == PageBootstrap || p == PageLessons
}

func (s *State) handleRulesAndOverviewKey(ev KeyEvent) bool {
	if isGovernancePage(s.Page) {
		return s.handleGovernanceKey(ev.Key)
	}
	return s.handleOverviewShortcuts(ev.Key)
}

func (s *State) handleGovernanceKey(key string) bool {
	if s.EffectivePage() == PageModels && s.handleModelsPageKey(key) {
		return true
	}
	switch key {
	case "b":
		s.Navigate(PageBootstrap)
		s.CmdOutput = "Bootstrapping missing repository workflow pillars..."
		s.enqueueAction(ActionRequest{Kind: "bootstrap"})
		return true
	case "t":
		s.toggleWorkspaceTrust()
		return true
	case "g":
		s.Navigate(PageLessons)
		s.graduateSelectedLesson()
		return true
	}
	return false
}

func (s *State) handleModelsPageKey(key string) bool {
	switch key {
	case "m":
		s.cycleSelectedScopeModel(true)
		return true
	case "M":
		s.cycleSelectedScopeModel(false)
		return true
	case "v":
		s.cycleModelPreset()
		return true
	case "x":
		s.resetSelectedScopeModel()
		return true
	case "r":
		s.requestRefreshModels()
		return true
	}
	return false
}

func (s *State) handleOverviewShortcuts(key string) bool {
	if s.Page == PageSummary && key == "b" {
		s.Navigate(PageBugs)
		s.CmdOutput = "Switched to [2] Agent Studio · Issues"
		return true
	}
	if s.Page == PageSummary && key == "f" {
		s.Navigate(PageFeatures)
		s.CmdOutput = "Switched to [2] Agent Studio · Features"
		return true
	}
	if s.Page == PageRepo && key == "r" {
		s.CmdOutput = "Reindexing repository..."
		s.enqueueAction(ActionRequest{Kind: "reindex"})
		return true
	}
	if (s.Page == PageSummary || s.Page == PageRepo || s.Page == PageProject) && key == "t" {
		s.CmdOutput = "Running project tests and checks..."
		s.enqueueAction(ActionRequest{Kind: "scan"})
		return true
	}
	return false
}
