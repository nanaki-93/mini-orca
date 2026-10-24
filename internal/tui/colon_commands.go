package tui

import (
	"fmt"
	"strings"

	"github.com/nanaki-93/mini-orca/v2/internal/tui/domain"
)

// ActivateCurrentRow performs the primary Enter action for the currently highlighted row.
func (s *State) ActivateCurrentRow() {
	switch s.Page {
	case PageSummary:
		s.activateDashboardRow()
	case PageBugs, PageIssueDetail:
		s.activateIssueRow()
	case PageRepo:
		s.CmdOutput = "Reindexing repository..."
		s.enqueueAction(ActionRequest{Kind: "reindex"})
	case PageProject:
		s.CmdOutput = "Running project verification checks..."
		s.enqueueAction(ActionRequest{Kind: "scan"})
	case PageEditor:
		s.activateEditorFileRow()
	case PageFeatures:
		s.Navigate(PageFeatureDetail)
	case PageFeatureDetail:
		features := s.activeFeatures()
		if idx := s.ClampedIdx(); idx < len(features) {
			s.dispatchFeatureIdea(features[idx])
		}
	case PageAgents:
		s.activateAgentRow()
	case PageAnalysisFiles:
		s.toggleCurrentFileExclusion()
	default:
		s.activateSecondaryRow()
	}
}

func (s *State) activateIssueRow() {
	if len(s.SelectedIssueIDs) > 0 {
		s.dispatchBatchIssues()
		return
	}
	issue := s.SelectedIssue()
	if issue == nil {
		if s.Page == PageSummary {
			s.Navigate(PageBugs)
			s.CmdOutput = "Switched to [2] Agent Studio · Issues"
		}
		return
	}
	if issue.AgentPath != "" {
		s.Navigate(PageAgents)
		s.AgentDetailOpen = true
		s.selectActiveAgentStage()
		s.CmdOutput = fmt.Sprintf("Opened agent for %s", issue.Name)
		return
	}
	if issue.Status != "done" {
		s.dispatchSingleIssue(*issue)
		return
	}
	s.openFileInEditor(issue.Path)
}

func (s *State) activateEditorFileRow() {
	if s.isViewingDiff() {
		s.applyAndCommitProposal("")
		return
	}
	if s.Index == nil {
		return
	}
	idx := s.ClampedIdx()
	if idx < len(s.Index.Files) {
		s.openFileInEditor(s.Index.Files[idx].Path)
	}
}

func (s *State) openAgentDiffInspector() {
	s.DiffSinceReview, s.DiffOffset, s.DetailScroll = false, 0, 0
	s.syncSelectedAgentSession()
	s.File = nil
	s.BrowseFiles = false
	if s.Change != nil && len(domain.EffectiveSessionChanges(s.Change)) > 0 {
		s.Navigate(PageEditor)
		s.syncDiffSelectionToHunk(0)
		s.CmdOutput = fmt.Sprintf("Inspecting diff for %s", s.Change.Title)
		return
	}
	if s.Change != nil && s.Change.ID != "" && !isLocalPreviewChangeID(s.Change.ID) {
		s.Navigate(PageEditor)
		s.CmdOutput = fmt.Sprintf("Loading diff for %s...", s.Change.Title)
		s.enqueueAction(ActionRequest{Kind: "load-change", Target: s.Change.ID})
		return
	}
	if s.Change != nil && len(s.Change.Targets) > 0 && s.Change.Targets[0].Path != "" {
		s.openFileInEditor(s.Change.Targets[0].Path)
		return
	}
	s.Navigate(PageEditor)
	s.CmdOutput = "Switched to [3] Code & Diff · Inspector"
}

func (s *State) activateAgentRow() {
	s.syncSelectedAgentSession()
	items := s.AgentItems()
	idx := s.ClampedIdx()
	if idx >= len(items) {
		return
	}
	s.AgentDetailOpen = true
	s.selectActiveAgentStage()
	s.enqueueAgentInspection()
	s.CmdOutput = fmt.Sprintf("Inspecting agent %s (%s)", items[idx].Title, items[idx].Branch)
}

func (s *State) activateSecondaryRow() {
	switch s.Page {
	case PageArtifacts:
		s.inspectLibraryRecord()
	case PageAnalysisRun:
		s.activateAnalysisRunRow()
	case PageHistory:
		if idx := s.ClampedIdx(); idx < len(s.ChangeHistory) {
			entry := s.ChangeHistory[idx]
			s.File = nil
			s.BrowseFiles = false
			s.CmdOutput = fmt.Sprintf("Loaded proposal %s", entry.Title)
			s.Navigate(PageEditor)
			s.enqueueAction(ActionRequest{Kind: "load-change", Target: entry.ID})
		}
	case PageInstructions, PageModels, PageRoles, PageSkills, PageConnections, PageExternalWork, PageBootstrap, PageLessons, PageMemory:
		s.activateInstructionsRow()
	}
}

func (s *State) activateAnalysisRunRow() {
	secCount := s.analysisRunSectionCount()
	idx := s.ClampedIdx()
	if s.Run != nil && (s.Run.Status == "paused" || s.Run.Status == "interrupted") && idx <= secCount {
		s.triggerOrResumeAnalysis()
		return
	}
	if s.Run != nil && idx < secCount {
		if idx < len(s.Run.Sections) {
			s.IssueCategory = s.Run.Sections[idx].Category
			s.Navigate(PageBugs)
		} else if s.Run.Features != nil {
			s.Navigate(PageFeatures)
		}
		return
	}
	s.stepAnalysisConfigByIndex(idx-secCount, true)
}

func (s *State) activateInstructionsRow() {
	switch s.EffectivePage() {
	case PageMemory:
		s.inspectLibraryRecord()
	case PageExternalWork:
		s.inspectExternalReceipt()
	case PageConnections:
		s.confirmConnectionControl("discover")
	case PageRoles, PageSkills:
		s.activateAgentSettingsRow()
	case PageModels:
		scopes := domain.CanonicalModelScopes()
		presets := s.AllModelPresets()
		idx := s.ClampedIdx()
		if idx < len(scopes) {
			s.cycleSelectedScopeModel(true)
			return
		}
		if pIdx := idx - len(scopes); pIdx >= 0 && pIdx < len(presets) {
			s.applyModelPreset(presets[pIdx])
		}
	case PageBootstrap:
		pillars := domain.EvaluateBootstrapChecklist(s.Index, s.Instructions)
		if idx := s.ClampedIdx(); idx < len(pillars) && !pillars[idx].Ready {
			s.CmdOutput = fmt.Sprintf("Bootstrapping %s...", pillars[idx].Title)
			s.enqueueAction(ActionRequest{Kind: "bootstrap", Target: pillars[idx].ID})
		}
	case PageLessons:
		s.graduateSelectedLesson()
	default:
		s.toggleWorkspaceTrust()
	}
}

func (s *State) openFileInEditor(path string) {
	if path == "" {
		return
	}
	s.BrowseFiles = false
	s.DiffOffset = 0
	s.File = &FileInfo{Path: path, Name: path}
	s.Navigate(PageEditor)
	s.CmdOutput = fmt.Sprintf("Opened %s", path)
	s.enqueueAction(ActionRequest{Kind: "open-file", Target: path})
}

// ExecuteCommand parses and executes a TUI colon command (e.g., ":analyze", ":bugs", ":q").
func (s *State) ExecuteCommand(raw string) {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), ":"))
	if trimmed == "" {
		return
	}
	s.Error = ""
	s.recordCommandHistory(raw)
	fields := strings.Fields(trimmed)
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))

	if s.execLibraryCommand(cmd, arg) || s.execExternalCommand(cmd, arg) || s.execConnectionCommand(cmd, arg) || s.execAgentSettingsCommand(cmd, arg) || s.execAgentRunCommand(cmd, arg) || s.execPlanCommand(cmd, arg) || s.execSystemCommand(cmd, arg) || s.execAnalysisCommand(cmd, arg) || s.execMutationCommand(cmd, arg) || s.execNavigationCommand(cmd, arg) {
		return
	}
	s.CmdOutput = fmt.Sprintf("Unknown command %q. Type \":keys\" or \"help\" for available commands.", trimmed)
}

func (s *State) recordCommandHistory(raw string) {
	entry := strings.TrimSpace(raw)
	var next []string
	next = append(next, entry)
	for _, item := range s.CmdHistory {
		if item != entry && len(next) < 30 {
			next = append(next, item)
		}
	}
	s.CmdHistory = next
}

func (s *State) execSystemCommand(cmd, arg string) bool {
	switch cmd {
	case "q", "quit", "exit", "gui":
		s.Quitting = true
		return true
	case "help", "?", "keys", "keybinds", "keybind", "shortcuts":
		s.ShowKeybindsModal = true
		s.CmdOutput = "Opened Keybindings Reference ([?] or Esc to close). Commands: keys | analyze | resume | reanalyze | test | fix | debug | agent <prompt> | open <file> | project <path> | models | apply | undo | quit"
		return true
	case "palette", "theme":
		s.CyclePalette()
		return true
	case "trust":
		s.toggleWorkspaceTrust()
		return true
	case "bootstrap":
		s.CmdOutput = "Bootstrapping missing repository workflow pillars..."
		s.enqueueAction(ActionRequest{Kind: "bootstrap"})
		return true
	case "reconnect", "connect":
		s.CmdOutput = "Reconnecting to local daemon..."
		s.enqueueAction(ActionRequest{Kind: "reconnect"})
		return true
	case "project":
		return s.execProjectCommand(arg)
	}
	return false
}

func (s *State) execProjectCommand(arg string) bool {
	if arg != "" {
		s.CmdOutput = fmt.Sprintf("Opening project at %s...", arg)
		s.enqueueAction(ActionRequest{Kind: "open-project", Target: arg})
		return true
	}
	s.Navigate(PageProject)
	s.CmdOutput = "Switched to [1] Overview · Project & Tests."
	return true
}

func (s *State) execNavigationCommand(cmd, arg string) bool {
	if s.execIssueFilterCommand(cmd, arg) || s.execModelCommand(cmd, arg) || s.execViewCommand(cmd) {
		return true
	}
	switch cmd {
	case "overview", "dashboard":
		s.Navigate(PageSummary)
		return true
	case "studio", "tasks":
		s.Navigate(PageAgents)
		return true
	case "debug", "logs":
		s.Navigate(PageAgents)
		s.AgentDetailOpen = true
		s.selectActiveAgentStage()
		s.enqueueAgentInspection()
		s.CmdOutput = "Task stages opened. Use ↑/↓ to select; Esc returns to tasks."
		return true
	case "scope", "files":
		s.Navigate(PageAnalysisFiles)
		s.CmdOutput = "Switched to [2] Agent Studio · File Scope."
		return true
	case "proposals", "history":
		s.Navigate(PageHistory)
		s.CmdOutput = "Switched to [3] Code & Diff · Saved Proposals."
		return true
	case "features":
		s.Navigate(PageFeatures)
		s.CmdOutput = "Switched to [2] Agent Studio · Features."
		return true
	case "arch", "diagrams":
		s.Navigate(PageDiagrams)
		s.CmdOutput = "Switched to [4] Handbook & Arch · Architecture."
		return true
	case "docs", "handbook":
		s.Navigate(PageDocs)
		s.CmdOutput = "Switched to [4] Handbook & Arch · Handbook / Docs."
		return true
	case "git":
		s.Navigate(PageGit)
		s.CmdOutput = "Switched to [3] Code & Diff · Git & Worktrees."
		return true
	case "open":
		if arg != "" {
			s.openFileInEditor(arg)
			return true
		}
		s.setIssueStatusFilter("open")
		return true
	}
	return s.execHubMatchCommand(cmd)
}

func (s *State) execIssueFilterCommand(cmd, arg string) bool {
	switch cmd {
	case "issues", "all", "bugs", "security", "performance", "perf":
		s.navigateToIssuesCategory(cmd)
		return true
	case "in-progress", "in_progress", "wip":
		s.setIssueStatusFilter("in_progress")
		return true
	case "done", "resolved", "fixed":
		s.setIssueStatusFilter("done")
		return true
	case "status":
		return s.applyStatusFilterArg(arg)
	case "category", "cat":
		return s.applyCategoryFilterArg(arg)
	case "filter", "filters":
		return s.applyCombinedFilterArg(arg)
	}
	return false
}

func (s *State) setIssueStatusFilter(view string) {
	s.Navigate(PageBugs)
	s.IssueView = view
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Issue status filter: %s (Category: %s)",
		strings.ToUpper(s.IssueView), strings.ToUpper(s.IssueCategory))
}

func (s *State) applyStatusFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueStatusView(true)
	case "open":
		s.setIssueStatusFilter("open")
	case "in_progress", "in-progress", "progress", "wip":
		s.setIssueStatusFilter("in_progress")
	case "done", "fixed", "resolved", "closed":
		s.setIssueStatusFilter("done")
	case "all":
		s.setIssueStatusFilter("all")
	default:
		s.CmdOutput = fmt.Sprintf("Unknown status filter %q (use: open, in_progress, done, all)", arg)
	}
	return true
}

func (s *State) applyCategoryFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueCategory(true)
	case "all":
		s.navigateToIssuesCategory("all")
	case "bugs", "bug":
		s.navigateToIssuesCategory("bugs")
	case "security", "sec":
		s.navigateToIssuesCategory("security")
	case "performance", "perf":
		s.navigateToIssuesCategory("performance")
	default:
		s.CmdOutput = fmt.Sprintf("Unknown category filter %q (use: all, bugs, security, perf)", arg)
	}
	return true
}

func (s *State) applyCombinedFilterArg(arg string) bool {
	s.Navigate(PageBugs)
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.cycleIssueStatusView(true)
		return true
	case "open", "in_progress", "in-progress", "progress", "wip", "done", "fixed", "resolved", "closed":
		return s.applyStatusFilterArg(norm)
	case "all-status", "all_status":
		return s.applyStatusFilterArg("all")
	case "all", "bugs", "bug", "security", "sec", "performance", "perf":
		return s.applyCategoryFilterArg(norm)
	default:
		s.CmdOutput = fmt.Sprintf("Unknown filter %q (use: open, in_progress, done, all, bugs, security, perf)", arg)
		return true
	}
}

func (s *State) navigateToIssuesCategory(cmd string) {
	s.Navigate(PageBugs)
	if cmd == "issues" {
		s.CmdOutput = "Switched to [2] Agent Studio · Issues."
		return
	}
	cat := cmd
	if cmd == "perf" {
		cat = "performance"
	}
	s.IssueCategory = cat
	s.SelectedIdx = 0
	s.CmdOutput = fmt.Sprintf("Switched to [2] Agent Studio · Issues (%s).", strings.ToUpper(cat))
}

func (s *State) execHubMatchCommand(cmd string) bool {
	for _, tab := range HubTabs() {
		if tab.Key == cmd || string(tab.Page) == cmd || strings.Contains(strings.ToLower(tab.Label), cmd) {
			s.Navigate(tab.Page)
			s.CmdOutput = fmt.Sprintf("Switched to [%s] %s.", tab.Key, tab.Label)
			return true
		}
	}
	return false
}

func (s *State) execDiffInspectorCommand(arg string) {
	norm := strings.ToLower(strings.TrimSpace(arg))
	switch norm {
	case "":
		s.openAgentDiffInspector()
	case "hunk", "focused", "split", "side", "side-by-side", "unified", "all", "stream":
		s.Navigate(PageEditor)
		s.setDiffViewMode(norm)
	case "next":
		s.Navigate(PageEditor)
		s.stepDiffHunk(true)
	case "prev":
		s.Navigate(PageEditor)
		s.stepDiffHunk(false)
	default:
		s.openAgentDiffInspector()
	}
}
