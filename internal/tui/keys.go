package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

// KeyEvent represents a decoded terminal keystroke.
type KeyEvent struct {
	Key    string
	Shift  bool
	Ctrl   bool
	Pasted bool
	Text   string
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
	if ev.Pasted && !s.CmdActive {
		return
	}
	if ev.Ctrl && strings.EqualFold(ev.Key, "c") {
		s.Quitting = true
		return
	}
	if s.handleOverlayKey(ev) {
		return
	}
	if s.handleFocusedPageKey(ev) {
		return
	}
	if s.handleDetailScrollKey(ev) {
		return
	}
	if s.Page == PagePlan {
		s.handlePlanKey(ev)
		return
	}
	if s.Page == PageIssueDetail {
		s.handleIssueDetailKey(ev)
		return
	}
	if s.handleDiffReviewKey(ev) {
		return
	}
	if s.handleGlobalNavigationKey(ev) {
		return
	}
	s.handlePageScopedKey(ev)
}

func (s *State) handleOverlayKey(ev KeyEvent) bool {
	if s.CmdActive {
		s.handleCommandInputKey(ev)
		return true
	}
	if !s.ShowKeybindsModal && s.handleDialogScrollKey(ev) {
		return true
	}
	if s.Confirmation != nil {
		s.handleConfirmationKey(ev)
		return true
	}
	if s.ShowKeybindsModal {
		s.handleKeybindsModalKey(ev)
		return true
	}
	if s.Error != "" {
		s.handleErrorOverlayKey(ev)
		return true
	}
	return false
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
		s.KeybindsScroll = 0
	case "PageDown":
		s.KeybindsScroll += max(1, s.viewportBodyRows-4)
	case "PageUp":
		s.KeybindsScroll = max(0, s.KeybindsScroll-max(1, s.viewportBodyRows-4))
	case "Home":
		s.KeybindsScroll = 0
	case "End":
		s.KeybindsScroll = 1 << 30
	case "j", "ArrowDown":
		s.KeybindsScroll++
	case "k", "ArrowUp":
		if s.KeybindsScroll > 0 {
			s.KeybindsScroll--
		}
	}
}

func (s *State) handleIssueDetailKey(ev KeyEvent) {
	if ev.Key == "Escape" && s.Error != "" {
		s.Error = ""
		s.CmdOutput = "Dismissed error alert."
		return
	}
	switch ev.Key {
	case "Escape", "Backspace", "i", "b", "q":
		s.Navigate(PageBugs)
		s.CmdOutput = "Returned to Issues list"
	case "j", "ArrowDown", "n":
		s.stepInspectedIssue(1)
	case "k", "ArrowUp", "N":
		s.stepInspectedIssue(-1)
	case "1", "2", "3", "4", "5":
		s.switchHubByKey(ev.Key)
	case "[", "]":
		s.cycleHub(ev.Key == "]")
	case "Tab":
		s.cycleSubView(!ev.Shift)
	case "?":
		s.ShowKeybindsModal = !s.ShowKeybindsModal
		s.KeybindsScroll = 0
	case ":", "/":
		s.openCommandInput("")
	case "p":
		s.CyclePalette()
	default:
		s.handleIssueDetailActionKey(ev.Key)
	}
}

func (s *State) handleIssueDetailActionKey(key string) {
	switch key {
	case "Enter", "w":
		s.activateIssueRow()
	case "f", "a":
		s.dispatchSingleSelectedIssue()
	case "d", "D", "Delete":
		s.dismissSelectedIssue()
	case "o":
		if issue := s.SelectedIssue(); issue != nil && issue.Path != "" {
			s.openFileInEditor(issue.Path)
		}
	}
}

func (s *State) stepInspectedIssue(delta int) {
	issues := s.FilteredIssues()
	if len(issues) == 0 {
		return
	}
	next := min(max(0, s.ClampedIdx()+delta), len(issues)-1)
	s.SelectedIdx = next
	s.queueIssueSource(issues[next])
	s.CmdOutput = fmt.Sprintf("Inspecting issue %d/%d: %s", next+1, len(issues), issues[next].Name)
}

func (s *State) handleGlobalNavigationKey(ev KeyEvent) bool {
	switch ev.Key {
	case "?":
		s.ShowKeybindsModal = !s.ShowKeybindsModal
		s.KeybindsScroll = 0
		return true
	case ":", "/":
		s.openCommandInput("")
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
	if s.Page == PageAgents && s.AgentDetailOpen {
		s.AgentDetailOpen = false
		s.AgentStage = 0
		s.CmdOutput = "Returned to Agents in Progress Dashboard"
		return true
	}
	if s.Page == PageEditor && (!s.BrowseFiles && (s.File != nil || s.isViewingDiff())) {
		s.File = nil
		s.BrowseFiles = true
		s.SelectedIdx = 0
		s.CmdOutput = "Browsing repository files (press [d] for proposal diff)"
		return true
	}
	return false
}

func (s *State) handleHorizontalStepKey(ev KeyEvent) bool {
	if ev.Key == "ArrowLeft" || ev.Key == "ArrowRight" {
		return s.handleArrowOptionKey(ev.Key == "ArrowRight", ev.Shift)
	}
	if ev.Key != "h" && ev.Key != "l" {
		return false
	}
	s.stepHorizontalSection(ev.Key == "l")
	return true
}

func (s *State) handleArrowOptionKey(forward, shift bool) bool {
	switch s.EffectivePage() {
	case PageAnalysisRun:
		if idx := s.ClampedIdx(); idx >= s.analysisRunSectionCount() {
			s.stepAnalysisConfigByIndex(idx-s.analysisRunSectionCount(), forward)
		}
	case PageAnalysisFiles:
		s.toggleCurrentFileExclusion()
	case PageEditor:
		s.handleEditorArrowOption(forward)
	case PageInstructions:
		s.toggleWorkspaceTrust()
	case PageModels:
		if s.ClampedIdx() < len(domain.CanonicalModelScopes()) {
			s.cycleSelectedScopeModel(forward)
		} else {
			s.activateInstructionsRow()
		}
	case PageBootstrap:
		s.activateInstructionsRow()
	case PageLessons:
		s.graduateSelectedLesson()
	}
	return true
}

func (s *State) handleEditorArrowOption(forward bool) {
	if s.isViewingDiff() {
		s.cycleDiffViewMode(forward)
		return
	}
	if !s.BrowseFiles && s.File != nil {
		s.stepIndexedEditorFile(forward)
		return
	}
	s.activateEditorFileRow()
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
	curPage := parentPage(s.EffectivePage())
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
	if s.isReadingPage() {
		s.DetailScroll = max(0, s.DetailScroll+delta)
		s.viewport.followSelection = false
		return
	}
	s.viewport.followSelection = true

	switch s.EffectivePage() {
	case PageAgents:
		if s.AgentDetailOpen {
			s.moveAgentDetailStageSelection(delta)
			return
		}
	case PageEditor:
		if s.isViewingDiff() {
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
	s.DetailScroll = 0
	if s.Page == PageAgents {
		s.syncSelectedAgentSession()
	}
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
	if isAnalysisTriggerPage(s.Page) {
		s.handleAnalysisKey(ev.Key)
	}
}

func isAnalysisTriggerPage(p Page) bool {
	switch p {
	case PageSummary, PageProject, PageAnalysisRun, PageAnalysisFiles, PageBugs, PageFeatures:
		return true
	default:
		return isHandbookPage(p)
	}
}

func isHandbookPage(p Page) bool {
	return p == PageDocs || p == PageDocsComponents || p == PageDocsDecisions || p == PageDiagrams
}

func (s *State) handleAnalysisKey(key string) {
	switch key {
	case "r":
		s.triggerOrResumeAnalysis()
	case "R":
		if s.isAnalysisPausedOrInterrupted() {
			s.triggerOrResumeAnalysis()
			return
		}
		s.CmdOutput = "Retrying failed/stale analysis files..."
		s.enqueueAction(ActionRequest{Kind: "repair-analysis"})
	case "n":
		s.CmdOutput = "Starting fresh project analysis (preserving in-progress issues)..."
		s.enqueueAction(ActionRequest{Kind: "reanalyze"})
	case "P":
		s.CmdOutput = "Pausing active analysis..."
		s.enqueueAction(ActionRequest{Kind: "pause-analysis"})
	}
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
		s.openSelectedIssueDetail()
		return true
	case "t", "c", ".", ">", "L":
		s.cycleIssueCategory(true)
		return true
	case "T", "C", ",", "<", "H":
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
	case "d", "D", "Delete", "Backspace":
		s.dismissSelectedIssue()
		return true
	}
	return false
}

func (s *State) openSelectedIssueDetail() {
	issues := s.FilteredIssues()
	if len(issues) == 0 {
		s.CmdOutput = "No finding selected to inspect."
		return
	}
	idx := s.ClampedIdx()
	s.Navigate(PageIssueDetail)
	s.SelectedIdx = idx
	s.queueIssueSource(issues[idx])
	s.CmdOutput = fmt.Sprintf("Inspecting issue %d/%d: %s ([f/Enter] Fix · [j/k] Prev/Next · [i/Esc] Back)", idx+1, len(issues), issues[idx].Name)
}

func (s *State) handleIssueActionKey(key string) bool {
	switch s.Page {
	case PageBugs, PageSummary:
		return s.handleIssueShortcut(key)
	case PageFeatures:
		return s.handleFeaturesKey(key)
	case PageAnalysisRun:
		return s.handleAnalysisRunConfigKey(key)
	case PageAnalysisFiles:
		if key == "x" || key == " " {
			s.toggleCurrentFileExclusion()
			return true
		}
	}
	return false
}

func (s *State) handleAnalysisRunConfigKey(key string) bool {
	switch key {
	case " ":
		if idx := s.ClampedIdx(); idx >= s.analysisRunSectionCount() {
			s.stepAnalysisConfigByIndex(idx-s.analysisRunSectionCount(), true)
		}
		return true
	case "b":
		s.stepAnalysisBatch(true)
		return true
	case "B":
		s.stepAnalysisBatch(false)
		return true
	case "t":
		s.stepAnalysisBudget(true)
		return true
	case "T":
		s.stepAnalysisBudget(false)
		return true
	case "a":
		s.stepAnalysisAttempts(true)
		return true
	case "A":
		s.stepAnalysisAttempts(false)
		return true
	case "f":
		s.toggleAnalysisFeatures()
		return true
	case "c":
		s.cycleAnalysisCachePolicy()
		return true
	case "x":
		s.resetAnalysisConfig()
		return true
	}
	return false
}

func (s *State) handleIssueShortcut(key string) bool {
	switch key {
	case "f":
		if s.Page == PageBugs {
			s.dispatchSingleSelectedIssue()
			return true
		}
	case "a":
		s.dispatchSelectedIssues()
		return true
	case "o":
		if s.Page != PageSummary {
			if issue := s.SelectedIssue(); issue != nil && issue.Path != "" {
				s.openFileInEditor(issue.Path)
				return true
			}
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
	case "a", "f":
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
	case PageGit, PageWorktreeDetail, PageHistory:
		return s.handleGitOrHistoryKey(ev.Key)
	}
	return false
}

func (s *State) handleGitOrHistoryKey(key string) bool {
	switch key {
	case "m":
		if parentPage(s.Page) == PageGit {
			s.mergeSelectedWorktree()
			return true
		}
	case "x", "X", "Delete":
		if parentPage(s.Page) == PageGit {
			s.discardSelectedWorktree()
			return true
		}
	case "a", "c":
		s.applyAndCommitProposal("")
		return true
	case "u":
		s.requestGuardedUndo()
		return true
	}
	return false
}

func (s *State) handleAgentsKey(key string) bool {
	switch key {
	case "b":
		s.Navigate(PageBugs)
		s.CmdOutput = "Switched to [2] Agent Studio · Issues"
		return true
	case "n":
		s.openCommandInput("agent ")
		return true
	case "i", "g":
		s.toggleAgentDebugInspector()
		return true
	case "d":
		s.openAgentDiffInspector()
		return true
	case "a", "c":
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
		s.cancelOrRejectActiveWorkflow()
		return true
	case "X", "Delete":
		s.rejectSelectedAgentFix()
		return true
	}
	return false
}

func (s *State) toggleAgentDebugInspector() {
	s.syncSelectedAgentSession()
	s.AgentDetailOpen = !s.AgentDetailOpen
	if s.AgentDetailOpen {
		s.AgentStage = s.activeAgentStageNumber()
		s.CmdOutput = "Opened Agent Workflow Debug & Check Inspector ([i/Esc] to return)"
		if s.Change != nil && s.Change.ID != "" && !isLocalPreviewChangeID(s.Change.ID) {
			s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
		}
		return
	}
	s.AgentStage = 0
	s.CmdOutput = "Returned to Agents in Progress Dashboard"
}

func (s *State) moveAgentDetailStageSelection(delta int) {
	cur := s.effectiveAgentDetailStage()
	next := min(4, max(1, cur+delta))
	s.AgentStage = next
	s.DetailScroll = 0
	s.CmdOutput = fmt.Sprintf("Selected Stage %d/4: %s", next, domain.CanonicalStageLabel(next-1, ""))
}

func (s *State) handleEditorKey(key string) bool {
	if s.handleEditorDiffNavKey(key) {
		return true
	}
	switch key {
	case "e":
		s.ShowInlineWhy = !s.ShowInlineWhy
		s.DetailScroll = 0
		stateWord := "hidden"
		if s.ShowInlineWhy {
			stateWord = "enabled"
		}
		s.CmdOutput = fmt.Sprintf("Inline diff explanations %s", stateWord)
		return true
	case "a", "c":
		s.applyAndCommitProposal("")
		return true
	case "u":
		s.requestGuardedUndo()
		return true
	case "x":
		s.rejectCurrentDiffHunk()
		return true
	case "o":
		s.toggleEditorFileBrowser()
		return true
	case "d":
		s.openAgentDiffInspector()
		return true
	}
	return false
}

func (s *State) toggleEditorFileBrowser() {
	if s.BrowseFiles {
		s.BrowseFiles = false
		s.SelectedIdx = 0
		s.CmdOutput = "Returned to diff inspector"
		return
	}
	s.File = nil
	s.BrowseFiles = true
	s.SelectedIdx = 0
	s.CmdOutput = "Browsing repository files"
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
	rowCount := s.diffViewRowCount()
	if rowCount > 14 && s.EffectiveDiffMode() != "unified" {
		if forward && s.SelectedIdx+12 < rowCount {
			s.SelectedIdx = min(rowCount-1, s.SelectedIdx+12)
			s.CmdOutput = fmt.Sprintf("Focused diff hunk #%d/%d · %s (line %d/%d)",
				active.GlobalIndex+1, len(allHunks), active.Path, s.SelectedIdx+1, rowCount)
			return
		}
		if !forward && s.SelectedIdx > domain.FirstChangedRowForMode(active.Hunk, s.EffectiveDiffMode()) {
			s.SelectedIdx = max(0, s.SelectedIdx-12)
			s.CmdOutput = fmt.Sprintf("Focused diff hunk #%d/%d · %s (line %d/%d)",
				active.GlobalIndex+1, len(allHunks), active.Path, s.SelectedIdx+1, rowCount)
			return
		}
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
	if !s.isViewingDiff() {
		s.stepIndexedEditorFile(forward)
		return
	}
	active, allHunks, ok := s.activeDiffHunk()
	if !ok {
		return
	}
	if active.FileTotal <= 1 {
		s.CmdOutput = fmt.Sprintf("Single file in proposal: %s", active.Path)
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

func (s *State) stepIndexedEditorFile(forward bool) {
	if s.Index == nil || len(s.Index.Files) == 0 {
		return
	}
	cur := 0
	curPath := ""
	if s.File != nil {
		curPath = s.File.Path
	} else if active, _, ok := s.activeDiffHunk(); ok {
		curPath = active.Path
	}
	for i, f := range s.Index.Files {
		if f.Path == curPath {
			cur = i
			break
		}
	}
	delta := -1
	if forward {
		delta = 1
	}
	next := (cur + delta + len(s.Index.Files)) % len(s.Index.Files)
	s.openFileInEditor(s.Index.Files[next].Path)
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
	s.CmdOutput = fmt.Sprintf("Switched diff inspector view to %s", diffModeDisplayName(eff))
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
	case "d":
		if s.Change != nil {
			s.openAgentDiffInspector()
			return true
		}
	case "a", "c":
		if s.Change != nil {
			s.applyAndCommitProposal("")
			return true
		}
	case "u":
		s.requestGuardedUndo()
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
	case "s":
		s.SaveModelPreset("")
		return true
	case "n":
		s.openCommandInput("preset-save ")
		s.CmdOutput = "Enter a name for the new model preset and press Enter"
		return true
	case "d":
		if s.DeleteSelectedCustomPreset() {
			return true
		}
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
	if (s.Page == PageSummary || s.Page == PageRepo) && key == "o" {
		s.openCommandInput("project ")
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
